package kit

// Pruebas de la hoja de imposición. Lo que hay que garantizar es lo que le
// cuesta dinero a la empresa si sale mal: que las piezas midan EXACTAMENTE lo
// que dice la especificación, que quepan en el pliego sin pisarse, que las guías
// de corte estén sobre el borde de cada pieza, y que el QR del PDF se pueda
// escanear de verdad.
//
// Nada de esto se comprueba mirando el generador: se PARSEA el PDF ya escrito y
// se trabaja sobre sus operadores. El QR además se rasteriza desde esos
// operadores y se decodifica con gozxing (un lector independiente), igual que
// hace el kit con el PNG (ADR-026).

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/makiuchi-d/gozxing"
	zxqr "github.com/makiuchi-d/gozxing/qrcode"
)

// piezasEsperadas son las medidas físicas del acuerdo con Daniel
// (identidad-visual-v1 §8). Se escriben a mano, sin derivarlas del código: si
// alguien cambia una constante del paquete, esta tabla es la que tiene que
// protestar.
var piezasEsperadas = []struct {
	nombre      string
	ancho, alto float64
}{
	{"wrap-izquierdo", 450, 180},
	{"wrap-derecho", 450, 180},
	{"instrucciones-3-pasos", 80, 180},
	{"cabecera-grabi", 280, 70},
	{"placa", 250, 50},
	{"qr", 100, 100},
}

func TestImposicionTieneLasSeisPiezasConSusMedidas(t *testing.T) {
	got := PiezasImposicion()
	if len(got) != len(piezasEsperadas) {
		t.Fatalf("el pliego trae %d piezas, se esperaban %d", len(got), len(piezasEsperadas))
	}
	for i, want := range piezasEsperadas {
		if got[i].Nombre != want.nombre {
			t.Errorf("pieza %d se llama %q, se esperaba %q", i, got[i].Nombre, want.nombre)
		}
		if got[i].Ancho != want.ancho || got[i].Alto != want.alto {
			t.Errorf("%s mide %g×%g mm, se esperaba %g×%g",
				got[i].Nombre, got[i].Ancho, got[i].Alto, want.ancho, want.alto)
		}
	}
}

// La retícula: nada se sale del pliego, nada se pisa, y las separaciones son las
// acordadas. Un solapamiento de 1 mm no se ve en pantalla y arruina la lámina.
func TestImposicionRespetaMargenesYSeparaciones(t *testing.T) {
	piezas := PiezasImposicion()
	for _, p := range piezas {
		if p.X < ImpMargen || p.Y < ImpMargen {
			t.Errorf("%s empieza en (%g,%g): pisa el margen de %g mm", p.Nombre, p.X, p.Y, ImpMargen)
		}
		if p.X+p.Ancho > ImpAncho-ImpMargen || p.Y+p.Alto > ImpAlto-ImpMargen {
			t.Errorf("%s acaba en (%g,%g) y no cabe en un pliego de %g×%g con margen %g",
				p.Nombre, p.X+p.Ancho, p.Y+p.Alto, ImpAncho, ImpAlto, ImpMargen)
		}
	}
	for i, a := range piezas {
		for _, b := range piezas[i+1:] {
			// Se comprueba contra el rectángulo CRECIDO media separación por
			// lado: si con ese aire aún se solapan, están más juntas de ImpGap.
			h := ImpGap / 2
			if a.X-h < b.X+b.Ancho+h && b.X-h < a.X+a.Ancho+h &&
				a.Y-h < b.Y+b.Alto+h && b.Y-h < a.Y+a.Alto+h {
				t.Errorf("%s y %s están a menos de %g mm", a.Nombre, b.Nombre, ImpGap)
			}
		}
	}
}

// --- lectura del PDF generado ---

// El pliego se genera una vez por prueba con este id, no con el de producción:
// así se ve que el arte se personaliza por máquina y no está cableado.
const idPrueba = "M042"

func pliegoDePrueba(t *testing.T) (Machine, []byte) {
	t.Helper()
	m := Machine{ID: idPrueba, Place: "Cali · prueba", Site: "https://grabi.napi.lat"}
	b, err := m.Imposicion()
	if err != nil {
		t.Fatalf("Imposicion(): %v", err)
	}
	return m, b
}

var reStream = regexp.MustCompile(`(?s)stream\r?\n(.*?)endstream`)

// flujo devuelve el flujo de contenido del PDF. Va sin comprimir a propósito
// (ver pdf.go), así que se puede leer tal cual.
func flujo(t *testing.T, doc []byte) string {
	t.Helper()
	mm := reStream.FindSubmatch(doc)
	if mm == nil {
		t.Fatal("el PDF no trae flujo de contenido")
	}
	return string(mm[1])
}

func TestImposicionEsUnPDFConLaPaginaDelTamanoDelPliego(t *testing.T) {
	_, doc := pliegoDePrueba(t)
	if !bytes.HasPrefix(doc, []byte("%PDF-1.")) {
		t.Fatalf("el archivo no empieza por la cabecera de un PDF: %.16q", doc)
	}
	if !bytes.HasSuffix(doc, []byte("%%EOF\n")) {
		t.Error("el PDF no termina en el marcador de fin de archivo")
	}
	// MediaBox y TrimBox en puntos, con el tamaño del pliego a escala 1:1.
	caja := fmt.Sprintf("[0 0 %s %s]", num(ImpAncho*ptPerMM), num(ImpAlto*ptPerMM))
	for _, clave := range []string{"/MediaBox " + caja, "/TrimBox " + caja, "/BleedBox " + caja} {
		if !bytes.Contains(doc, []byte(clave)) {
			t.Errorf("el PDF no declara %s", clave)
		}
	}
	// Las guías van en su propia capa y en el color plano que busca el plóter.
	for _, clave := range []string{
		"/Type /OCG", "corte kiss-cut", "/OC /ocCorte BDC",
		"[/Separation /" + kissCutName + " /DeviceCMYK",
	} {
		if !bytes.Contains(doc, []byte(clave)) {
			t.Errorf("el PDF no trae %q (¿se perdió la capa de corte?)", clave)
		}
	}
	// El pliego se personaliza por máquina: el id tiene que aparecer en el arte.
	if !strings.Contains(flujo(t, doc), "("+idPrueba+")") {
		t.Errorf("el arte del pliego no imprime el id %q", idPrueba)
	}
}

// El mismo id tiene que dar el mismo archivo byte a byte: si no, no se puede
// comparar lo que se mandó a imprimir con lo que genera el servidor hoy.
func TestImposicionEsReproducible(t *testing.T) {
	_, a := pliegoDePrueba(t)
	_, b := pliegoDePrueba(t)
	if !bytes.Equal(a, b) {
		t.Error("dos pliegos de la misma máquina salen distintos (¿fecha o mapa recorrido?)")
	}
}

// reCorte captura el trazado de corte de cada pieza: el `m` inicial del
// rectángulo redondeado y la esquina opuesta, que salen del mismo roundRectPath.
var (
	reOpNum   = `(-?[\d.]+)`
	reMoveTo  = regexp.MustCompile(reOpNum + ` ` + reOpNum + ` m`)
	reLineTo  = regexp.MustCompile(reOpNum + ` ` + reOpNum + ` l`)
	reRect    = regexp.MustCompile(reOpNum + ` ` + reOpNum + ` ` + reOpNum + ` ` + reOpNum + ` re`)
	reSetFill = regexp.MustCompile(reOpNum + ` ` + reOpNum + ` ` + reOpNum + ` rg`)
)

// Las guías de corte tienen que caer sobre el borde EXACTO de cada pieza: es el
// contrato con la imprenta. Se miden sobre los operadores de la capa de corte.
func TestImposicionGuiasDeCorteSobreElBordeDeCadaPieza(t *testing.T) {
	_, doc := pliegoDePrueba(t)
	c := flujo(t, doc)
	i := strings.Index(c, "/OC /ocCorte BDC")
	if i < 0 {
		t.Fatal("no hay capa de corte en el flujo")
	}
	corte := c[i:]

	if !strings.Contains(corte, num(ImpCorte)+" w") {
		t.Errorf("el trazado de corte no va a %g mm de grosor", ImpCorte)
	}
	if !strings.Contains(corte, "/"+kissCutName+" CS 1 SCN") {
		t.Error("el trazado de corte no usa el color plano KissCut a tinta plena")
	}

	// Cada pieza deja un trazado; se reconstruye su caja envolvente a partir de
	// los puntos y se compara con la pieza. El radio de la esquina hace que el
	// primer punto esté a ImpRadio del vértice, así que se comparan cajas.
	trazos := strings.Split(corte, " m\n")
	if len(trazos)-1 != len(piezasEsperadas) {
		t.Fatalf("la capa de corte trae %d trazados, se esperaban %d", len(trazos)-1, len(piezasEsperadas))
	}
	for i, p := range PiezasImposicion() {
		// Se recompone el trazado i (el `m` va al final del trozo anterior).
		trazo := reMoveTo.FindString(trazos[i]+" m") + "\n" + trazos[i+1]
		x0, y0, x1, y1 := caja(trazo)
		for _, cmp := range []struct {
			que        string
			got, quiso float64
		}{
			{"borde izquierdo", x0, p.X},
			{"borde superior", y0, p.Y},
			{"borde derecho", x1, p.X + p.Ancho},
			{"borde inferior", y1, p.Y + p.Alto},
		} {
			if math.Abs(cmp.got-cmp.quiso) > 0.01 {
				t.Errorf("guía de corte de %s: %s en %g mm, se esperaba %g",
					p.Nombre, cmp.que, cmp.got, cmp.quiso)
			}
		}
	}
}

// caja devuelve la envolvente de los puntos `m`/`l` de un trazado.
func caja(trazo string) (x0, y0, x1, y1 float64) {
	x0, y0 = math.Inf(1), math.Inf(1)
	x1, y1 = math.Inf(-1), math.Inf(-1)
	for _, re := range []*regexp.Regexp{reMoveTo, reLineTo} {
		for _, mm := range re.FindAllStringSubmatch(trazo, -1) {
			x, y := flt(mm[1]), flt(mm[2])
			x0, y0 = math.Min(x0, x), math.Min(y0, y)
			x1, y1 = math.Max(x1, x), math.Max(y1, y)
		}
	}
	return x0, y0, x1, y1
}

func flt(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// --- el QR del PDF se escanea de verdad ---

// El QR es lo único del pliego que puede dejar la máquina inservible: si sale
// mal, el sticker queda bonito y nadie puede comprar. Así que no se comprueba
// que el generador "haya llamado a drawQR": se REPRODUCEN los rectángulos que el
// PDF pinta dentro de la pieza del QR, se rasterizan y se decodifican con un
// lector ajeno.
func TestImposicionElQRDelPDFSeEscanea(t *testing.T) {
	m, doc := pliegoDePrueba(t)

	var pieza PiezaImp
	for _, p := range PiezasImposicion() {
		if p.Nombre == "qr" {
			pieza = p
		}
	}
	if pieza.Nombre == "" {
		t.Fatal("el pliego no trae la pieza del QR")
	}

	img := rasteriza(t, flujo(t, doc), pieza, 4)
	texto, ecc := decodificaImagen(t, img)
	if want := m.URL(); texto != want {
		t.Errorf("el QR del pliego decodifica %q, se esperaba %q", texto, want)
	}
	// Nivel H: sin ese margen de redundancia el símbolo de marca encima taparía
	// datos irrecuperables (ADR-026).
	if ecc != "H" {
		t.Errorf("el QR del pliego trae ECC %q, se esperaba H", ecc)
	}
}

// rasteriza reproduce los rellenos del flujo de contenido que caen dentro de la
// pieza, a px píxeles por milímetro. Es un intérprete de juguete: solo entiende
// `rg` (color de relleno) y `re`+`f` (rectángulo), que es exactamente de lo que
// está hecho un QR. Los pinta EN ORDEN, así que el cuadro blanco de la marca
// tapa los módulos igual que en el archivo real.
func rasteriza(t *testing.T, flujo string, p PiezaImp, px float64) *image.Gray {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, int(p.Ancho*px), int(p.Alto*px)))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	gris := color.Gray{Y: 255}
	pintados := 0
	for _, linea := range strings.Split(flujo, "\n") {
		if mm := reSetFill.FindStringSubmatch(linea); mm != nil {
			// Luminancia aproximada: al lector solo le importa claro/oscuro.
			gris = color.Gray{Y: uint8(255 * (0.299*flt(mm[1]) + 0.587*flt(mm[2]) + 0.114*flt(mm[3])))}
			continue
		}
		for _, mm := range reRect.FindAllStringSubmatch(linea, -1) {
			x, y, w, h := flt(mm[1]), flt(mm[2]), flt(mm[3]), flt(mm[4])
			if x < p.X || y < p.Y || x+w > p.X+p.Ancho || y+h > p.Y+p.Alto {
				continue // rectángulo de otra pieza
			}
			r := image.Rect(
				int(math.Round((x-p.X)*px)), int(math.Round((y-p.Y)*px)),
				int(math.Round((x-p.X+w)*px)), int(math.Round((y-p.Y+h)*px)))
			draw.Draw(img, r, image.NewUniform(gris), image.Point{}, draw.Src)
			pintados++
		}
	}
	if pintados < 100 {
		t.Fatalf("solo se reprodujeron %d rectángulos dentro de la pieza del QR: "+
			"el flujo no tiene el QR donde se esperaba", pintados)
	}
	return img
}

// decodificaImagen escanea la imagen y devuelve el texto y el nivel de ECC.
func decodificaImagen(t *testing.T, img image.Image) (texto, ecc string) {
	t.Helper()
	bmp, err := gozxing.NewBinaryBitmap(gozxing.NewHybridBinarizer(gozxing.NewLuminanceSourceFromImage(img)))
	if err != nil {
		t.Fatalf("bitmap: %v", err)
	}
	res, err := zxqr.NewQRCodeReader().Decode(bmp, nil)
	if err != nil {
		t.Fatalf("el QR del pliego no se pudo decodificar: %v", err)
	}
	ecc, _ = res.GetResultMetadata()[gozxing.ResultMetadataType_ERROR_CORRECTION_LEVEL].(string)
	return res.GetText(), ecc
}

// --- copy y tipografía ---

// El copy es de marca: se imprime tal cual, y las medidas de las cajas de texto
// tienen que dar para que ninguna línea se salga del arte.
func TestImposicionCopyDeMarca(t *testing.T) {
	_, doc := pliegoDePrueba(t)
	c := flujo(t, doc)

	l1, l2 := parteCopy(CopyWrapDer)
	// Partido en dos líneas, las palabras y su orden son los del copy original.
	if junto := l1 + " · " + l2; junto != CopyWrapDer {
		t.Errorf("el wrap derecho se parte como %q + %q y eso reescribe el copy %q", l1, l2, CopyWrapDer)
	}
	quiere := []string{l1, l2, "ESCANEA AQUÍ", "GRABI"}
	tag := taglineLineas()
	quiere = append(quiere, tag[:]...)
	for _, pa := range pasosImp {
		quiere = append(quiere, pa.Titulo, pa.L1, pa.L2)
	}
	for _, s := range quiere {
		if !strings.Contains(c, "("+pdfString(s)+")") {
			t.Errorf("el pliego no imprime %q", s)
		}
	}
}

// fitSize es el cinturón de seguridad de la maquetación: si una línea no cabe,
// baja el cuerpo en vez de salirse del arte. Sin esto, una línea larga solo se
// nota cuando el vinilo ya está impreso.
func TestFitSizeNuncaDejaQueUnaLineaSeSalga(t *testing.T) {
	const maxW = 100.0
	for _, s := range []string{"corto", strings.Repeat("MMMM ", 20)} {
		size := fitSize(fontDisplay, 34, maxW, s)
		if w := textWidth(fontDisplay, size, s); w > maxW+0.01 {
			t.Errorf("%q a cuerpo %g mide %g mm, más que los %g permitidos", s, size, w, maxW)
		}
		if size > 34 {
			t.Errorf("fitSize AGRANDÓ el cuerpo de %q a %g", s, size)
		}
	}
}

// El texto de las piezas lleva tildes; en el PDF va en WinAnsi (un byte por
// letra), no en UTF-8, o el lector pintaría dos glifos raros por cada tilde.
func TestPDFStringCodificaEnWinAnsi(t *testing.T) {
	for _, caso := range []struct{ in, want string }{
		{"agárralo.", "ag\xe1rralo."},
		{"máquina", "m\xe1quina"},
		{"AQUÍ", "AQU\xcd"},
		{"a · b", "a \xb7 b"},
		{"(paréntesis)", "\\(par\xe9ntesis\\)"},
		// Fuera de Latin-1 no hay glifo en WinAnsi: mejor un '?' visible que un
		// archivo que el lector no abre.
		{"emoji ✅", "emoji ?"},
	} {
		if got := pdfString(caso.in); got != caso.want {
			t.Errorf("pdfString(%q) = %q, se esperaba %q", caso.in, got, caso.want)
		}
	}
}
