package kit

// Lo que estas pruebas protegen: un QR bonito que no se lee es papel tirado a la
// basura y una máquina que nadie puede usar. Por eso el QR generado se DECODIFICA
// de verdad (gozxing, un decodificador independiente del generador) y se
// comprueba que devuelve la URL original con la marca ya sobrepuesta.

import (
	"archive/zip"
	"bytes"
	"image/png"
	"strings"
	"testing"

	"github.com/makiuchi-d/gozxing"
	zxqr "github.com/makiuchi-d/gozxing/qrcode"
)

const urlPrueba = "https://grabi.napi.lat/m/M001"

// decodifica lee un PNG como lo haría un celular y devuelve el texto del QR más
// el nivel de corrección de errores que trae el símbolo.
func decodifica(t *testing.T, pngBytes []byte) (texto, ecc string) {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("PNG inválido: %v", err)
	}
	src := gozxing.NewLuminanceSourceFromImage(img)
	bmp, err := gozxing.NewBinaryBitmap(gozxing.NewHybridBinarizer(src))
	if err != nil {
		t.Fatalf("bitmap: %v", err)
	}
	res, err := zxqr.NewQRCodeReader().Decode(bmp, nil)
	if err != nil {
		t.Fatalf("el QR no se pudo decodificar: %v", err)
	}
	ecc, _ = res.GetResultMetadata()[gozxing.ResultMetadataType_ERROR_CORRECTION_LEVEL].(string)
	return res.GetText(), ecc
}

// El QR con el logo encima sigue siendo escaneable, a varios tamaños y para
// distintos ids. Es LA prueba del kit: si esto falla, no se imprime nada.
func TestQRConLogoSigueSiendoEscaneable(t *testing.T) {
	for _, u := range []string{
		"https://grabi.napi.lat/m/M001",
		"https://grabi.napi.lat/m/M0042",
		"https://grabi.napi.lat/m/M123456",
	} {
		for _, px := range []int{256, 512, 1024} {
			b, err := PNG(u, px)
			if err != nil {
				t.Fatalf("PNG(%s, %d): %v", u, px, err)
			}
			texto, ecc := decodifica(t, b)
			if texto != u {
				t.Errorf("PNG(%s, %d) decodifica %q", u, px, texto)
			}
			if ecc != "H" {
				t.Errorf("PNG(%s, %d): ECC %q, se esperaba H", u, px, ecc)
			}
		}
	}
}

// El nivel H no es un detalle estético: es el margen de redundancia que permite
// tapar el centro con la marca. Se comprueba también sobre la matriz cruda.
func TestNivelDeCorreccionEsH(t *testing.T) {
	b, err := PNG(urlPrueba, 512)
	if err != nil {
		t.Fatalf("PNG: %v", err)
	}
	if _, ecc := decodifica(t, b); ecc != "H" {
		t.Fatalf("ECC = %q, se esperaba H (Reed-Solomon alto)", ecc)
	}
}

// El logo tapa módulos: si crece de más, el QR deja de leerse aunque en pantalla
// se vea bien. El tope acordado es 20% del área.
func TestElLogoNoTapaMasDelVeintePorCiento(t *testing.T) {
	m, err := Matrix(urlPrueba)
	if err != nil {
		t.Fatalf("Matrix: %v", err)
	}
	n := len(m)
	side, off := logoBox(n)
	simbolo := n - 2*quietZone

	// Contra el símbolo (el peor caso: la zona de silencio no lleva datos).
	if frac := float64(side*side) / float64(simbolo*simbolo); frac > 0.20 {
		t.Errorf("el logo tapa %.1f%% del símbolo, el tope es 20%%", frac*100)
	}
	// Y contra el área total del QR tal como se imprime.
	if frac := float64(side*side) / float64(n*n); frac > 0.20 {
		t.Errorf("el logo tapa %.1f%% del área total, el tope es 20%%", frac*100)
	}
	// Centrado exacto: si el cuadro no cae en módulos enteros, parte módulos por
	// la mitad y eso confunde al lector más que taparlos.
	if off*2+side != n {
		t.Errorf("el cuadro del logo no está centrado: off=%d side=%d n=%d", off, side, n)
	}
}

// El SVG es lo que va a imprenta: tiene que pintar exactamente los mismos
// módulos que el PNG que sí se decodifica, más el cuadro blanco de la marca.
func TestSVGPintaLosMismosModulosQueLaMatriz(t *testing.T) {
	m, err := Matrix(urlPrueba)
	if err != nil {
		t.Fatalf("Matrix: %v", err)
	}
	n := len(m)
	side, off := logoBox(n)
	oscuros := 0
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			if m[r][c] && !(r >= off && r < off+side && c >= off && c < off+side) {
				oscuros++
			}
		}
	}

	svg, err := SVG(urlPrueba, 512)
	if err != nil {
		t.Fatalf("SVG: %v", err)
	}
	s := string(svg)
	// rects = módulos oscuros + fondo blanco + cuadro blanco del logo.
	if got, want := strings.Count(s, "<rect"), oscuros+2; got != want {
		t.Errorf("el SVG tiene %d <rect>, se esperaban %d (%d módulos + fondo + cuadro del logo)", got, want, oscuros)
	}
	for _, want := range []string{
		`viewBox="0 0 41 41"`,          // unidades = módulos, no píxeles
		`width="512"`,                  // el tamaño pedido llega a los atributos
		`shape-rendering="crispEdges"`, // sin esto el antialias deja costuras
		`fill="#FFFFFF"`,               // el cuadro del logo es OPACO
		ColorAccent,                    // el punto verde de la marca
	} {
		if !strings.Contains(s, want) {
			t.Errorf("el SVG no contiene %q", want)
		}
	}
}

// El ZIP es lo que descarga Daniel: si falta una pieza, se entera en la imprenta.
func TestZipTraeLasPiezasEsperadas(t *testing.T) {
	m := Machine{ID: "M001", Place: "Palmira"}
	var buf bytes.Buffer
	if err := m.WriteZip(&buf); err != nil {
		t.Fatalf("WriteZip: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("ZIP inválido: %v", err)
	}

	dentro := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("abriendo %s: %v", f.Name, err)
		}
		var b bytes.Buffer
		_, _ = b.ReadFrom(rc)
		rc.Close()
		dentro[f.Name] = b.Bytes()
	}
	for _, name := range ZipFiles {
		if len(dentro[name]) == 0 {
			t.Errorf("falta (o va vacía) la pieza %q en el kit.zip", name)
		}
	}
	if len(dentro) != len(ZipFiles) {
		t.Errorf("el kit.zip trae %d archivos, se esperaban %d", len(dentro), len(ZipFiles))
	}

	// El qr.png del kit se decodifica igual que el que se sirve suelto.
	texto, ecc := decodifica(t, dentro["qr.png"])
	if texto != m.URL() {
		t.Errorf("el qr.png del kit decodifica %q, se esperaba %q", texto, m.URL())
	}
	if ecc != "H" {
		t.Errorf("el qr.png del kit trae ECC %q, se esperaba H", ecc)
	}
	mat, err := Matrix(m.URL())
	if err != nil {
		t.Fatalf("Matrix: %v", err)
	}
	wantPx := (PNGZipSize / len(mat)) * len(mat) // múltiplo entero de módulos
	if img, err := png.Decode(bytes.NewReader(dentro["qr.png"])); err != nil {
		t.Errorf("qr.png ilegible: %v", err)
	} else if b := img.Bounds(); b.Dx() != wantPx || b.Dy() != wantPx {
		t.Errorf("qr.png mide %v, se esperaba %dx%d", b.Size(), wantPx, wantPx)
	}

	// Las piezas SVG llevan la marca, el tagline y los datos de la máquina.
	// El banner del frente: tagline en tres líneas (la última en verde), bajada
	// mono y marca fantasma de fondo.
	frente := string(dentro["sticker-frente.svg"])
	for _, want := range []string{
		"Escanea,", "paga,", "agárralo.",
		"Sin efectivo · sin datáfono · pago Bre-B",
		`width="400mm"`, `height="185mm"`,
		ColorGhost, // la marca fantasma
	} {
		if !strings.Contains(frente, want) {
			t.Errorf("sticker-frente.svg no contiene %q", want)
		}
	}
	// El banner NO lleva QR (decisión de Daniel: el QR se pega aparte, desde
	// qr.svg, a la altura de la mano). Cientos de rects delatarían un QR dentro.
	if n := strings.Count(frente, "<rect"); n > 20 {
		t.Errorf("sticker-frente.svg tiene %d <rect>: parece llevar el QR incrustado", n)
	}
	// La placa va SIN fondo (vinilo transparente): un panel oscuro de lado a lado
	// la delataría.
	placa := string(dentro["placa.svg"])
	for _, want := range []string{"GRABI", "· M001", `width="90mm"`} {
		if !strings.Contains(placa, want) {
			t.Errorf("placa.svg no contiene %q", want)
		}
	}
	if strings.Contains(placa, `width="90" height="30"`) || strings.Contains(placa, ColorBG) {
		t.Error("placa.svg trae fondo: debe ir sin panel, para vinilo transparente")
	}
	// El panel de instrucciones: los tres pasos completos y el argumento de venta.
	pasos := string(dentro["instrucciones.svg"])
	for _, want := range []string{
		"Sin efectivo.", "Sin datáfono.", "Solo tu celular.",
		"Escanea el QR de", "la máquina",
		"Paga con Bre-B", "desde tu banco",
		"Muestra el QR y", "agárralo",
		"grabi.napi.lat", `width="300mm"`, `height="160mm"`,
	} {
		if !strings.Contains(pasos, want) {
			t.Errorf("instrucciones.svg no contiene %q", want)
		}
	}
	// Los tres números, cada uno en su círculo verde. Sin ellos el panel deja de
	// ser una secuencia y pasa a ser una lista suelta.
	if n := strings.Count(pasos, `r="10" fill="`+ColorAccent+`"`); n != 3 {
		t.Errorf("instrucciones.svg tiene %d círculos de paso, se esperaban 3", n)
	}
	// El panel habla del sitio, no de una máquina concreta: es el mismo para todas.
	if strings.Contains(pasos, "M001") {
		t.Error("instrucciones.svg menciona una máquina concreta; debe servir para todas")
	}
}

// El tagline se parte por palabras para las piezas, pero NUNCA se reescribe ni se
// traduce (identidad-visual-v1 §1): las tres líneas juntas tienen que reconstruirlo.
func TestElTaglineSePartePeroNoSeReescribe(t *testing.T) {
	d := Machine{ID: "M001"}.datos()
	if got := d.Tag1 + " " + d.Tag2 + " " + d.Tag3; got != Tagline {
		t.Errorf("las tres líneas dan %q, se esperaba %q", got, Tagline)
	}
}

// El id de la máquina lo escribe el admin y se imprime en la placa: un `&` o un
// `<` sin escapar rompen el SVG y la pieza deja de abrir en cualquier programa de
// diseño.
func TestLosDatosDeLaMaquinaSeEscapan(t *testing.T) {
	m := Machine{ID: `M1 & <script>"x"`, Place: "Palmira"}
	placa, err := m.Placa()
	if err != nil {
		t.Fatalf("Placa: %v", err)
	}
	s := string(placa)
	if strings.Contains(s, "& <") || strings.Contains(s, "<script>") {
		t.Errorf("el id entra sin escapar en el SVG:\n%s", s)
	}
	for _, want := range []string{"&amp;", "&lt;script&gt;", "&quot;x&quot;"} {
		if !strings.Contains(s, want) {
			t.Errorf("falta el escape %q", want)
		}
	}
}

// La URL del QR es la página pública de la máquina, con la base configurable.
func TestURLDeLaMaquina(t *testing.T) {
	if got := (Machine{ID: "M001"}).URL(); got != urlPrueba {
		t.Errorf("URL por defecto = %q, se esperaba %q", got, urlPrueba)
	}
	if got := (Machine{ID: "M007", Site: "http://localhost:8080/"}).URL(); got != "http://localhost:8080/m/M007" {
		t.Errorf("URL con base propia = %q", got)
	}
}

// El PNG se genera a un múltiplo entero del número de módulos: con módulos de
// tamaño fraccionario unos salen de 3 px y otros de 4, y el lector pierde el paso.
func TestElPNGUsaModulosEnteros(t *testing.T) {
	m, err := Matrix(urlPrueba)
	if err != nil {
		t.Fatalf("Matrix: %v", err)
	}
	b, err := PNG(urlPrueba, 500) // 500 no es múltiplo de 41
	if err != nil {
		t.Fatalf("PNG: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("PNG inválido: %v", err)
	}
	if got := img.Bounds().Dx(); got%len(m) != 0 || got > 500 || got < 500-len(m) {
		t.Errorf("lado = %d px con %d módulos; debería ser el mayor múltiplo ≤ 500", got, len(m))
	}
}
