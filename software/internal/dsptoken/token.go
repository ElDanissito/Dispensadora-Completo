// Package dsptoken implementa la emisión (firma) y verificación del token de
// dispensado descrito en especificaciones/contrato-token.md (v2).
//
// El servidor firma; la máquina (ESP32) verifica offline. Este paquete es la
// referencia canónica que el firmware debe replicar bit a bit: mismo formato
// JWS compacto, mismo orden de validaciones (§5) y mismos códigos de error (§7).
//
// Cambio v1 → v2 (ADR-006): el payload se adelgaza; se eliminan `iss` (constante,
// implícita) e `iat` (auditoría, se guarda solo en el servidor). El emisor sigue
// registrando esos datos en su BD; simplemente no viajan en el QR.
package dsptoken

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
)

// Constantes del contrato v2.
const (
	AlgEdDSA   = "EdDSA" // único alg aceptado (anti-downgrade)
	TypDSP     = "DSP"   // tipo de token: dispensado
	DefaultKID = "k1"    // id de llave por defecto
	DefaultTTL = 300     // ventana de expiración recomendada: 5 min
)

// b64 es base64url SIN padding, como exige el contrato §4.
var b64 = base64.RawURLEncoding

// Header del JWS. El orden de los campos es fijo para que el JSON compacto sea
// determinista: {"alg":"EdDSA","typ":"DSP","kid":"k1"}.
type Header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid"`
}

// Item de la orden: slot (s) y cantidad (q). Nombres cortos para minimizar el
// tamaño del QR (contrato §3 y §6).
type Item struct {
	S int `json:"s"` // slot físico
	Q int `json:"q"` // cantidad (≥1)
}

// Payload del token (v2). Orden de campos fijo para JSON determinista:
// {"mid":...,"jti":...,"exp":...,"items":[...]} (contrato §3 y §4).
// `iss` e `iat` se eliminaron en v2 (ADR-006).
type Payload struct {
	Mid   string `json:"mid"`
	Jti   string `json:"jti"`
	Exp   int64  `json:"exp"`
	Items []Item `json:"items"`
}

// Code es el resultado de la verificación. "OK" o uno de los códigos del §7.
type Code string

const (
	OK           Code = "OK"
	Malformed    Code = "MALFORMED"     // no son 3 partes / JSON inválido / alg|typ no aceptados
	BadSignature Code = "BAD_SIGNATURE" // firma Ed25519 inválida
	UnknownKey   Code = "UNKNOWN_KEY"   // no hay llave pública para el kid
	WrongMachine Code = "WRONG_MACHINE" // mid != machine_id de esta máquina
	Expired      Code = "EXPIRED"       // now > exp
	AlreadyUsed  Code = "ALREADY_USED"  // jti ya consumido
)

// DefaultHeader devuelve el header estándar v1 para el kid dado.
func DefaultHeader(kid string) Header {
	if kid == "" {
		kid = DefaultKID
	}
	return Header{Alg: AlgEdDSA, Typ: TypDSP, Kid: kid}
}

// Sign construye y firma un token según el contrato §4.
//
//	signing_input = b64url(header) + "." + b64url(payload)
//	firma         = Ed25519_sign(priv, signing_input)   (64 bytes)
//	token         = signing_input + "." + b64url(firma)
func Sign(priv ed25519.PrivateKey, h Header, p Payload) (string, error) {
	hb, err := json.Marshal(h)
	if err != nil {
		return "", err
	}
	pb, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	signingInput := b64.EncodeToString(hb) + "." + b64.EncodeToString(pb)
	sig := ed25519.Sign(priv, []byte(signingInput))
	return signingInput + "." + b64.EncodeToString(sig), nil
}

// KeyStore mapea un kid a su llave pública (la máquina guarda kid → pública).
type KeyStore interface {
	PublicKey(kid string) (ed25519.PublicKey, bool)
}

// MapKeyStore es una implementación en memoria de KeyStore.
type MapKeyStore map[string]ed25519.PublicKey

func (m MapKeyStore) PublicKey(kid string) (ed25519.PublicKey, bool) {
	k, ok := m[kid]
	return k, ok
}

// UsedStore registra los jti ya consumidos (memoria no volátil en la máquina).
type UsedStore interface {
	IsUsed(jti string) bool
	MarkUsed(jti string)
}

// MemUsed es una implementación en memoria de UsedStore (para pruebas/simulador).
type MemUsed map[string]bool

func (m MemUsed) IsUsed(jti string) bool { return m[jti] }
func (m MemUsed) MarkUsed(jti string)    { m[jti] = true }

// VerifyParams reúne el estado local de la máquina para verificar.
type VerifyParams struct {
	Keys      KeyStore  // mapa kid → pública
	MachineID string    // id de ESTA máquina
	Now       int64     // hora del RTC (epoch s)
	Used      UsedStore // registro de jti usados; nil ⇒ se omite el chequeo de reuso
}

// Result es el resultado de Verify. Header/Payload se rellenan tan pronto como
// se logran decodificar, para facilitar auditoría aunque la verificación falle.
type Result struct {
	Code    Code
	Header  *Header
	Payload *Payload
}

// Verify aplica EXACTAMENTE el orden de validaciones del contrato §5. Es la
// referencia que el firmware debe reproducir para dar resultados idénticos.
//
// Nota sobre alg/typ: el contrato §2 exige rechazar cualquier alg distinto de
// "EdDSA" (anti-downgrade). No hay un código dedicado en la tabla §7, así que se
// reporta como MALFORMED ("QR no válido"), que es el efecto correcto de cara al
// usuario.
func Verify(token string, vp VerifyParams) Result {
	// 1. Separar en 3 partes.
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Result{Code: Malformed}
	}

	// 2. Decodificar header; exigir alg=="EdDSA", typ=="DSP"; obtener kid.
	hb, err := b64.DecodeString(parts[0])
	if err != nil {
		return Result{Code: Malformed}
	}
	var h Header
	if err := json.Unmarshal(hb, &h); err != nil {
		return Result{Code: Malformed}
	}
	if h.Alg != AlgEdDSA || h.Typ != TypDSP {
		return Result{Code: Malformed, Header: &h}
	}

	// 3. Buscar la llave pública para ese kid.
	pub, ok := vp.Keys.PublicKey(h.Kid)
	if !ok {
		return Result{Code: UnknownKey, Header: &h}
	}

	// 4. Verificar la firma Ed25519 sobre (parte1 + "." + parte2).
	sig, err := b64.DecodeString(parts[2])
	if err != nil {
		return Result{Code: Malformed, Header: &h}
	}
	signingInput := parts[0] + "." + parts[1]
	if !ed25519.Verify(pub, []byte(signingInput), sig) {
		return Result{Code: BadSignature, Header: &h}
	}

	// 5. Decodificar payload.
	pb, err := b64.DecodeString(parts[1])
	if err != nil {
		return Result{Code: Malformed, Header: &h}
	}
	var p Payload
	if err := json.Unmarshal(pb, &p); err != nil {
		return Result{Code: Malformed, Header: &h}
	}

	// 6. mid (v2: ya no hay chequeo de iss, eliminado en ADR-006).
	if p.Mid != vp.MachineID {
		return Result{Code: WrongMachine, Header: &h, Payload: &p}
	}
	// 7. exp.
	if vp.Now > p.Exp {
		return Result{Code: Expired, Header: &h, Payload: &p}
	}
	// 8. jti no usado.
	if vp.Used != nil && vp.Used.IsUsed(p.Jti) {
		return Result{Code: AlreadyUsed, Header: &h, Payload: &p}
	}
	// 9. Marcar jti como usado ANTES de dispensar (aquí: solo registro).
	if vp.Used != nil {
		vp.Used.MarkUsed(p.Jti)
	}

	return Result{Code: OK, Header: &h, Payload: &p}
}

// --- Codificación de llaves para archivos (base64 estándar, legible) ---

// EncodePublic serializa una llave pública (32 bytes) a base64 estándar.
func EncodePublic(pub ed25519.PublicKey) string {
	return base64.StdEncoding.EncodeToString(pub)
}

// DecodePublic parsea una llave pública desde base64 estándar.
func DecodePublic(s string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, err
	}
	return ed25519.PublicKey(raw), nil
}

// EncodePrivate serializa una llave privada (64 bytes) a base64 estándar.
func EncodePrivate(priv ed25519.PrivateKey) string {
	return base64.StdEncoding.EncodeToString(priv)
}

// DecodePrivate parsea una llave privada desde base64 estándar.
func DecodePrivate(s string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, err
	}
	return ed25519.PrivateKey(raw), nil
}
