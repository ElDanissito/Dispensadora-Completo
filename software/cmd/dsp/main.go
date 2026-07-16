// Comando dsp: herramienta CLI del agente de Software para el token de
// dispensado (contrato-token.md v2). Permite:
//
//	dsp keygen    genera un par de llaves Ed25519 (privada fuera del repo)
//	dsp sign      firma un token y (opcional) genera su QR
//	dsp verify    verifica un token aplicando todas las validaciones del contrato
//	dsp qr        genera el QR PNG de un token ya existente
//	dsp vectors   genera los vectores de prueba (§10) para el firmware
//
// La llave PRIVADA nunca se imprime en el repo: se guarda en software/.keys/
// (ignorada por git) o se pasa por la variable de entorno DSP_PRIVATE_KEY.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dispensadoras/software/internal/dsptoken"
	"dispensadoras/software/internal/qr"
)

// Rutas por defecto relativas a la raíz del repo (se ejecuta desde /software).
const (
	defaultPrivPath = ".keys/private-k1.key"
	defaultPubPath  = "../especificaciones/vectores-prueba/llave-publica-k1.txt"
	vectorsDir      = "../especificaciones/vectores-prueba"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = cmdKeygen(os.Args[2:])
	case "sign":
		err = cmdSign(os.Args[2:])
	case "verify":
		err = cmdVerify(os.Args[2:])
	case "qr":
		err = cmdQR(os.Args[2:])
	case "vectors":
		err = cmdVectors(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `dsp — herramienta del token de dispensado (contrato v2)

Uso:
  dsp keygen  [-priv RUTA] [-pub RUTA] [-kid k1] [-force]
  dsp sign    [-priv RUTA] -mid M001 [-items "3:1,5:2"] [-jti X] [-exp N|-ttl 300] [-kid k1] [-qr salida.png] [-size 512]
  dsp verify  -pub RUTA -mid M001 (-token STR | -in RUTA) [-now N]
  dsp qr      (-token STR | -in RUTA) -out salida.png [-size 512]
  dsp vectors [-priv RUTA] [-pub RUTA]

La llave privada se toma de -priv o de la variable de entorno DSP_PRIVATE_KEY.
`)
}

// ---------- keygen ----------

func cmdKeygen(args []string) error {
	fs := newFlags("keygen")
	priv := fs.String("priv", defaultPrivPath, "ruta donde guardar la llave privada")
	pub := fs.String("pub", defaultPubPath, "ruta donde guardar la llave pública")
	force := fs.Bool("force", false, "sobrescribir la privada si ya existe")
	fs.Parse(args)

	if _, err := os.Stat(*priv); err == nil && !*force {
		return fmt.Errorf("ya existe una llave privada en %s (usa -force para sobrescribir)", *priv)
	}

	pk, sk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	if err := writeFile(*priv, []byte(dsptoken.EncodePrivate(sk)+"\n"), 0o600); err != nil {
		return err
	}
	if err := writeFile(*pub, []byte(dsptoken.EncodePublic(pk)+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Printf("Llave privada → %s (¡NO commitear!)\n", *priv)
	fmt.Printf("Llave pública → %s\n", *pub)
	fmt.Printf("Pública (base64): %s\n", dsptoken.EncodePublic(pk))
	return nil
}

// ---------- sign ----------

func cmdSign(args []string) error {
	fs := newFlags("sign")
	priv := fs.String("priv", defaultPrivPath, "ruta de la llave privada (o env DSP_PRIVATE_KEY)")
	mid := fs.String("mid", "", "machine_id (obligatorio)")
	items := fs.String("items", "", `items "slot:cant,slot:cant" (ej. "3:1,5:2")`)
	jti := fs.String("jti", "", "id de orden (por defecto aleatorio)")
	exp := fs.Int64("exp", 0, "expira en (epoch s; 0 = ahora+ttl)")
	ttl := fs.Int64("ttl", dsptoken.DefaultTTL, "ventana de validez en segundos si -exp=0")
	kid := fs.String("kid", dsptoken.DefaultKID, "id de la llave")
	qrOut := fs.String("qr", "", "si se indica, escribe el QR PNG a esta ruta")
	size := fs.Int("size", 512, "lado del QR en px")
	fs.Parse(args)

	if *mid == "" {
		return fmt.Errorf("-mid es obligatorio")
	}
	sk, err := loadPrivate(*priv)
	if err != nil {
		return err
	}
	parsedItems, err := parseItems(*items)
	if err != nil {
		return err
	}

	// v2: `iat` ya no viaja en el token. Se usa solo como base para calcular exp
	// (el servidor real lo registra en su BD, no en el QR).
	emit := time.Now().Unix()
	expVal := *exp
	if expVal == 0 {
		expVal = emit + *ttl
	}
	jtiVal := *jti
	if jtiVal == "" {
		jtiVal = randomJTI()
	}

	p := dsptoken.Payload{
		Mid:   *mid,
		Jti:   jtiVal,
		Exp:   expVal,
		Items: parsedItems,
	}
	token, err := dsptoken.Sign(sk, dsptoken.DefaultHeader(*kid), p)
	if err != nil {
		return err
	}

	fmt.Println(token)
	fmt.Fprintf(os.Stderr, "jti=%s  exp=%d  len=%d chars\n", jtiVal, expVal, len(token))

	if *qrOut != "" {
		if err := qr.WritePNG(token, *qrOut, *size); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "QR → %s\n", *qrOut)
	}
	return nil
}

// ---------- verify ----------

func cmdVerify(args []string) error {
	fs := newFlags("verify")
	pub := fs.String("pub", defaultPubPath, "ruta de la llave pública")
	mid := fs.String("mid", "", "machine_id de ESTA máquina (obligatorio)")
	token := fs.String("token", "", "token a verificar")
	in := fs.String("in", "", "archivo que contiene el token")
	now := fs.Int64("now", 0, "hora del RTC (epoch s; 0 = ahora real)")
	kid := fs.String("kid", dsptoken.DefaultKID, "kid al que corresponde la llave pública dada")
	fs.Parse(args)

	if *mid == "" {
		return fmt.Errorf("-mid es obligatorio")
	}
	tok, err := loadToken(*token, *in)
	if err != nil {
		return err
	}
	pk, err := loadPublic(*pub)
	if err != nil {
		return err
	}
	nowVal := *now
	if nowVal == 0 {
		nowVal = time.Now().Unix()
	}

	res := dsptoken.Verify(tok, dsptoken.VerifyParams{
		Keys:      dsptoken.MapKeyStore{*kid: pk},
		MachineID: *mid,
		Now:       nowVal,
		Used:      dsptoken.MemUsed{}, // sesión única: sin persistencia de jti
	})

	fmt.Println(res.Code)
	if res.Payload != nil {
		b, _ := json.MarshalIndent(res.Payload, "", "  ")
		fmt.Fprintln(os.Stderr, string(b))
	}
	if res.Code != dsptoken.OK {
		os.Exit(3) // salida distinta para scripts
	}
	return nil
}

// ---------- qr ----------

func cmdQR(args []string) error {
	fs := newFlags("qr")
	token := fs.String("token", "", "token")
	in := fs.String("in", "", "archivo que contiene el token")
	out := fs.String("out", "", "ruta PNG de salida (obligatorio)")
	size := fs.Int("size", 512, "lado del QR en px")
	fs.Parse(args)

	if *out == "" {
		return fmt.Errorf("-out es obligatorio")
	}
	tok, err := loadToken(*token, *in)
	if err != nil {
		return err
	}
	if err := qr.WritePNG(tok, *out, *size); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "QR → %s (%d chars)\n", *out, len(tok))
	return nil
}

// ---------- vectors ----------

// cmdVectors produce el juego completo de vectores de prueba (§10) para que el
// firmware valide sin hardware. Usa timestamps FIJOS y documenta el NOW de
// referencia contra el que deben evaluarse.
func cmdVectors(args []string) error {
	fs := newFlags("vectors")
	priv := fs.String("priv", defaultPrivPath, "ruta de la llave privada (se genera si no existe)")
	pub := fs.String("pub", defaultPubPath, "ruta de la llave pública")
	fs.Parse(args)

	// Asegurar par de llaves.
	sk, err := loadPrivate(*priv)
	if err != nil {
		pk, newSk, gerr := ed25519.GenerateKey(rand.Reader)
		if gerr != nil {
			return gerr
		}
		sk = newSk
		if werr := writeFile(*priv, []byte(dsptoken.EncodePrivate(sk)+"\n"), 0o600); werr != nil {
			return werr
		}
		if werr := writeFile(*pub, []byte(dsptoken.EncodePublic(pk)+"\n"), 0o644); werr != nil {
			return werr
		}
		fmt.Fprintf(os.Stderr, "Generado nuevo par de llaves (priv=%s, pub=%s)\n", *priv, *pub)
	}
	pk := sk.Public().(ed25519.PublicKey)

	// Parámetros fijos de los vectores.
	const (
		machineID = "M001"
		nowRef    = int64(1752460900) // hora de referencia para evaluar
	)
	items := []dsptoken.Item{{S: 3, Q: 1}, {S: 5, Q: 2}}

	// token-valido: vigente respecto a nowRef (nowRef < exp).
	valido, err := dsptoken.Sign(sk, dsptoken.DefaultHeader(dsptoken.DefaultKID), dsptoken.Payload{
		Mid: machineID, Jti: "ord_valid01",
		Exp: 1752461100, Items: items,
	})
	if err != nil {
		return err
	}

	// token-expirado: firma válida pero exp < nowRef.
	expirado, err := dsptoken.Sign(sk, dsptoken.DefaultHeader(dsptoken.DefaultKID), dsptoken.Payload{
		Mid: machineID, Jti: "ord_exp01",
		Exp: 1752460300, Items: items,
	})
	if err != nil {
		return err
	}

	// token-firma-mala: partimos de uno válido y corrompemos el último carácter
	// de la firma, manteniéndolo bien formado (3 partes, base64url válido).
	base, err := dsptoken.Sign(sk, dsptoken.DefaultHeader(dsptoken.DefaultKID), dsptoken.Payload{
		Mid: machineID, Jti: "ord_badsig01",
		Exp: 1752461100, Items: items,
	})
	if err != nil {
		return err
	}
	firmaMala := corromperFirma(base)

	// Escribir archivos.
	files := map[string]string{
		"token-valido.txt":     valido + "\n",
		"token-expirado.txt":   expirado + "\n",
		"token-firma-mala.txt": firmaMala + "\n",
	}
	for name, content := range files {
		if err := writeFile(filepath.Join(vectorsDir, name), []byte(content), 0o644); err != nil {
			return err
		}
	}

	// resultados-esperados.md
	md := resultadosMD(machineID, nowRef, dsptoken.EncodePublic(pk), valido, expirado, firmaMala)
	if err := writeFile(filepath.Join(vectorsDir, "resultados-esperados.md"), []byte(md), 0o644); err != nil {
		return err
	}

	fmt.Printf("Vectores generados en %s\n", vectorsDir)
	fmt.Printf("  token-valido      len=%d\n", len(valido))
	fmt.Printf("  token-expirado    len=%d\n", len(expirado))
	fmt.Printf("  token-firma-mala  len=%d\n", len(firmaMala))
	fmt.Printf("NOW de referencia: %d\n", nowRef)
	return nil
}

// corromperFirma altera un carácter de la firma sin romper el formato.
//
// IMPORTANTE: se corrompe el PRIMER carácter, no el último. El último carácter
// de una firma de 64 bytes en base64url solo lleva 2 bits significativos (los
// altos); cambiar 'A'↔'B' ahí solo toca un bit de relleno y la firma decodifica
// IGUAL → el token seguiría siendo válido. El primer carácter lleva 6 bits
// significativos, así que cambiarlo garantiza una firma distinta (inválida).
func corromperFirma(token string) string {
	parts := strings.Split(token, ".")
	sig := []byte(parts[2])
	// cambiar el primer carácter por otro base64url válido pero distinto
	if sig[0] == 'A' {
		sig[0] = 'B'
	} else {
		sig[0] = 'A'
	}
	parts[2] = string(sig)
	return strings.Join(parts, ".")
}

func resultadosMD(mid string, nowRef int64, pub, valido, expirado, firmaMala string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Vectores de prueba — resultados esperados\n\n")
	fmt.Fprintf(&b, "> Generado por `dsp vectors`. Fuente: contrato-token.md v2.\n")
	fmt.Fprintf(&b, "> El simulador de verificación (02) y el firmware (03) DEBEN dar exactamente estos resultados.\n\n")
	fmt.Fprintf(&b, "## Parámetros de evaluación\n\n")
	fmt.Fprintf(&b, "- **MACHINE_ID de la máquina de prueba:** `%s`\n", mid)
	fmt.Fprintf(&b, "- **kid → llave pública:** `k1` → ver `llave-publica-k1.txt`\n")
	fmt.Fprintf(&b, "  - base64: `%s`\n", pub)
	fmt.Fprintf(&b, "- **NOW de referencia (RTC simulado):** `%d` (epoch s)\n", nowRef)
	fmt.Fprintf(&b, "- **jti usados al inicio:** ninguno (lista vacía)\n\n")
	fmt.Fprintf(&b, "> Los tokens llevan `exp` FIJO. Para reproducir los resultados hay que evaluar\n")
	fmt.Fprintf(&b, "> con `now = %d`, no con la hora real (si no, `token-valido` aparecería expirado).\n\n", nowRef)
	fmt.Fprintf(&b, "## Casos\n\n")
	fmt.Fprintf(&b, "| Archivo | Resultado esperado | Por qué |\n")
	fmt.Fprintf(&b, "|---------|--------------------|---------|\n")
	fmt.Fprintf(&b, "| `token-valido.txt` | `OK` | Firma válida, `mid` correcto, `now (%d) ≤ exp (1752461100)`, jti no usado. Dispensa items `[{s:3,q:1},{s:5,q:2}]`. |\n", nowRef)
	fmt.Fprintf(&b, "| `token-expirado.txt` | `EXPIRED` | Firma válida pero `now (%d) > exp (1752460300)`. Se rechaza en el paso 7 (v2). |\n", nowRef)
	fmt.Fprintf(&b, "| `token-firma-mala.txt` | `BAD_SIGNATURE` | Igual que el válido pero con la firma corrompida. Se rechaza en el paso 4 (antes de mirar exp). |\n\n")
	fmt.Fprintf(&b, "## Nota sobre segundo uso (ALREADY_USED)\n\n")
	fmt.Fprintf(&b, "Si se verifica `token-valido.txt` DOS veces con el mismo registro de `jti`,\n")
	fmt.Fprintf(&b, "la primera da `OK` y la segunda `ALREADY_USED` (el jti `ord_valid01` queda marcado).\n\n")
	fmt.Fprintf(&b, "## Tokens (para inspección)\n\n")
	fmt.Fprintf(&b, "```\ntoken-valido:     %s\ntoken-expirado:   %s\ntoken-firma-mala: %s\n```\n", valido, expirado, firmaMala)
	return b.String()
}

// ---------- helpers ----------

func newFlags(name string) *flag.FlagSet { return flag.NewFlagSet(name, flag.ExitOnError) }

func parseItems(s string) ([]dsptoken.Item, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("se requiere al menos un item (-items \"3:1\")")
	}
	var items []dsptoken.Item
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		sp := strings.SplitN(pair, ":", 2)
		if len(sp) != 2 {
			return nil, fmt.Errorf("item inválido %q (formato slot:cant)", pair)
		}
		slot, err := strconv.Atoi(strings.TrimSpace(sp[0]))
		if err != nil {
			return nil, fmt.Errorf("slot inválido en %q: %w", pair, err)
		}
		qty, err := strconv.Atoi(strings.TrimSpace(sp[1]))
		if err != nil {
			return nil, fmt.Errorf("cantidad inválida en %q: %w", pair, err)
		}
		if qty < 1 {
			return nil, fmt.Errorf("la cantidad debe ser ≥1 en %q", pair)
		}
		items = append(items, dsptoken.Item{S: slot, Q: qty})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no se parsearon items de %q", s)
	}
	return items, nil
}

func randomJTI() string {
	var b [5]byte
	if _, err := rand.Read(b[:]); err != nil {
		// fallback determinista muy improbable de alcanzar
		return "ord_00000000"
	}
	return "ord_" + hex.EncodeToString(b[:])
}

// loadPrivate carga la privada desde la env DSP_PRIVATE_KEY o desde el archivo.
func loadPrivate(path string) (ed25519.PrivateKey, error) {
	if env := os.Getenv("DSP_PRIVATE_KEY"); env != "" {
		return dsptoken.DecodePrivate(env)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer la llave privada (%s ni env DSP_PRIVATE_KEY): %w", path, err)
	}
	return dsptoken.DecodePrivate(string(data))
}

func loadPublic(path string) (ed25519.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer la llave pública %s: %w", path, err)
	}
	return dsptoken.DecodePublic(string(data))
}

func loadToken(token, in string) (string, error) {
	if token != "" {
		return strings.TrimSpace(token), nil
	}
	if in != "" {
		data, err := os.ReadFile(in)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	}
	return "", fmt.Errorf("se requiere -token o -in")
}

func writeFile(path string, data []byte, perm os.FileMode) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, perm)
}
