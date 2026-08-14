package kit

// Escritor de PDF mínimo, sin dependencias nuevas. Las piezas del kit son
// geometría vectorial pura (rectángulos, círculos, arcos y texto), así que
// escribir el PDF a mano sale más barato que arrastrar una librería: nos deja
// además controlar las dos cosas que la imprenta necesita y ninguna librería de
// Go expone bien — **capas** (contenido opcional) y un **color plano con nombre**
// para el kiss-cut.
//
// Convenciones:
//   - Se dibuja en MILÍMETROS y con la **y hacia abajo**, igual que los SVG del
//     kit, para que el mismo razonamiento de maquetación sirva en los dos sitios.
//     La matriz de la página hace la conversión a puntos y el volteo.
//   - El texto compensa ese volteo con su propia matriz (`1 0 0 -1`), si no
//     saldría cabeza abajo.
//   - El flujo de contenido va SIN comprimir: son ~100 KB y así el archivo se
//     puede abrir en un editor de texto para auditar qué se mandó a imprimir.

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
)

// ptPerMM convierte milímetros a puntos PostScript (1 pt = 1/72 de pulgada).
const ptPerMM = 72.0 / 25.4

// Nombres de recurso de las dos fuentes. Son base-14 (van en todo lector sin
// incrustar): sustitutos de las de marca, que son de pago y no están en el repo.
//   - fontDisplay ← Archivo 900 (titulares) y Space Grotesk 700 (texto).
//   - fontMonoBold ← IBM Plex Mono (datos y labels).
//
// La imprenta debe pasarlas a curvas si quiere la tipografía real (igual que en
// los SVG del kit); las métricas de abajo son las de estas dos, no las de marca.
const (
	fontDisplay = "F1" // Helvetica-Bold
	// La mono va SIEMPRE en negrita: Courier a secas tiene el trazo demasiado
	// fino para vinilo y a cuerpos pequeños sobre fondo oscuro el impreso se lo
	// come. Por eso no se declara la redonda: el archivo solo lleva lo que usa.
	fontMonoBold = "F2" // Courier-Bold
)

// kissCutName es el nombre del color plano de las guías de corte. Las máquinas
// de corte (Roland/Summa y compañía) buscan una **separación con nombre**, no un
// color de proceso: si las guías fueran magenta de cuatricromía se imprimirían
// como tinta en vez de leerse como trazado de corte.
const kissCutName = "KissCut"

// pdf acumula el contenido de una página de una sola hoja.
type pdf struct {
	w, h  float64 // tamaño de página en mm
	arte  bytes.Buffer
	corte bytes.Buffer
	cur   *bytes.Buffer
}

func newPDF(wmm, hmm float64) *pdf {
	p := &pdf{w: wmm, h: hmm}
	p.cur = &p.arte
	return p
}

// capaArte y capaCorte eligen a qué capa van los siguientes trazos. Son dos
// grupos de contenido opcional del PDF, así que la imprenta puede apagar el
// corte para ver solo el arte (y al revés) sin editar el archivo.
func (p *pdf) capaArte()  { p.cur = &p.arte }
func (p *pdf) capaCorte() { p.cur = &p.corte }

func (p *pdf) op(format string, a ...any) {
	fmt.Fprintf(p.cur, format, a...)
	p.cur.WriteByte('\n')
}

// num formatea una longitud para el PDF con precisión de micra: más decimales
// solo engordan el archivo (la imprenta no corta por debajo de 0,1 mm).
func num(v float64) string {
	return strconv.FormatFloat(math.Round(v*1000)/1000, 'f', -1, 64)
}

// --- color ---
//
// El arte va en RGB con los HEX EXACTOS de la guía de marca, no en CMYK, y esto
// es deliberado (ADR-027):
//
//   - Convertir a CMYK aquí sería una conversión CIEGA (no hay perfil ICC en el
//     repo, y no puede haberlo: son licenciados). Probado: `#0A0E0C` sale como
//     29C/0M/14,5Y/94,5K, que un visor con gestión de color pinta gris azulado en
//     vez de casi negro, y el verde `#3BE87F` se vuelve turquesa.
//   - Peor aún, esa conversión es IRREVERSIBLE: el RIP pasa el DeviceCMYK a
//     plancha tal cual, sin corregirlo. En cambio el DeviceRGB lo convierte él,
//     con el perfil del MATERIAL — y para vinilo (látex/eco-solvente) ese gamut
//     es más ancho que SWOP, así que reproduce el verde mejor que nosotros.
//   - Y así el pliego especifica el MISMO color que los SVG del kit.zip: la pieza
//     impresa desde el ZIP y desde el pliego salen iguales.

// color3 devuelve las tres componentes 0..1 de un hex "#RRGGBB".
func (p *pdf) color3(hex string) string {
	c := hexColor(hex)
	return fmt.Sprintf("%s %s %s", num(float64(c.R)/255), num(float64(c.G)/255), num(float64(c.B)/255))
}

func (p *pdf) setFill(hex string)   { p.op("%s rg", p.color3(hex)) }
func (p *pdf) setStroke(hex string) { p.op("%s RG", p.color3(hex)) }

// --- formas ---

// rect pinta un rectángulo relleno. Con la y hacia abajo, alto positivo crece
// hacia abajo desde (x,y).
func (p *pdf) rect(x, y, w, h float64, hex string) {
	p.setFill(hex)
	p.op("%s %s %s %s re f", num(x), num(y), num(w), num(h))
}

// kappa es el factor para aproximar un cuarto de círculo con una bézier cúbica.
const kappa = 0.5522847498307936

// roundRectPath traza (sin pintar) un rectángulo de esquinas redondeadas.
func (p *pdf) roundRectPath(x, y, w, h, r float64) {
	r = math.Min(r, math.Min(w, h)/2)
	k := r * kappa
	x1, y1 := x+w, y+h
	p.op("%s %s m", num(x+r), num(y))
	p.op("%s %s l", num(x1-r), num(y))
	p.op("%s %s %s %s %s %s c", num(x1-r+k), num(y), num(x1), num(y+r-k), num(x1), num(y+r))
	p.op("%s %s l", num(x1), num(y1-r))
	p.op("%s %s %s %s %s %s c", num(x1), num(y1-r+k), num(x1-r+k), num(y1), num(x1-r), num(y1))
	p.op("%s %s l", num(x+r), num(y1))
	p.op("%s %s %s %s %s %s c", num(x+r-k), num(y1), num(x), num(y1-r+k), num(x), num(y1-r))
	p.op("%s %s l", num(x), num(y+r))
	p.op("%s %s %s %s %s %s c", num(x), num(y+r-k), num(x+r-k), num(y), num(x+r), num(y))
	p.op("h")
}

func (p *pdf) roundRect(x, y, w, h, r float64, hex string) {
	p.setFill(hex)
	p.roundRectPath(x, y, w, h, r)
	p.op("f")
}

func (p *pdf) circle(cx, cy, r float64, hex string) {
	p.setFill(hex)
	k := r * kappa
	p.op("%s %s m", num(cx+r), num(cy))
	p.op("%s %s %s %s %s %s c", num(cx+r), num(cy+k), num(cx+k), num(cy+r), num(cx), num(cy+r))
	p.op("%s %s %s %s %s %s c", num(cx-k), num(cy+r), num(cx-r), num(cy+k), num(cx-r), num(cy))
	p.op("%s %s %s %s %s %s c", num(cx-r), num(cy-k), num(cx-k), num(cy-r), num(cx), num(cy-r))
	p.op("%s %s %s %s %s %s c", num(cx+k), num(cy-r), num(cx+r), num(cy-k), num(cx+r), num(cy))
	p.op("f")
}

// --- marca compacta (identidad-visual-v1 §3) ---

// markCorners son las cuatro esquinas del visor como polilíneas, en el lienzo de
// 100×100 de la marca. Van como polilínea (no como segmentos sueltos) para que
// el vértice lo remate la unión redonda, igual que en el SVG.
var markCorners = [4][3][2]float64{
	{{41, 24}, {24, 24}, {24, 41}},
	{{59, 24}, {76, 24}, {76, 41}},
	{{41, 76}, {24, 76}, {24, 59}},
	{{59, 76}, {76, 76}, {76, 59}},
}

// marca pinta el símbolo compacto (visor + punto) en el cuadrado (x,y,size).
func (p *pdf) marca(x, y, size float64, trazo, punto string) {
	s := size / 100
	p.op("q 1 J 1 j")
	p.setStroke(trazo)
	p.op("%s w", num(markStroke*s))
	for _, esq := range markCorners {
		p.op("%s %s m", num(x+esq[0][0]*s), num(y+esq[0][1]*s))
		p.op("%s %s l", num(x+esq[1][0]*s), num(y+esq[1][1]*s))
		p.op("%s %s l", num(x+esq[2][0]*s), num(y+esq[2][1]*s))
	}
	p.op("S")
	p.op("Q")
	p.circle(x+50*s, y+50*s, markDotR*s, punto)
}

// badge es la marca dentro del cuadro verde. Sobre el verde acento siempre va
// tinta oscura, nunca clara (identidad-visual-v1 §4).
func (p *pdf) badge(x, y, size float64) {
	p.roundRect(x, y, size, size, size*0.22, ColorAccent)
	p.marca(x, y, size, ColorInk, ColorInk)
}

// fantasma es la marca gigante de fondo: apenas más clara que el panel.
func (p *pdf) fantasma(x, y, size float64) {
	p.marca(x, y, size, ColorGhost, ColorGhost)
}

// --- QR ---

// drawQR pinta el QR (fondo blanco + módulos + cuadro con la marca) dentro del
// cuadrado (x,y,size). Es el equivalente vectorial de writeQRBody: mismos
// módulos, mismo cuadro impar centrado y misma marca encima.
func (p *pdf) drawQR(m [][]bool, x, y, size float64) {
	n := len(m)
	u := size / float64(n)
	p.rect(x, y, size, size, "#FFFFFF")

	side, off := logoBox(n)
	p.setFill(ColorInk)
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			if !m[r][c] {
				continue
			}
			// Los módulos bajo el cuadro de la marca no se pintan: van tapados
			// igual y el archivo pesa menos.
			if r >= off && r < off+side && c >= off && c < off+side {
				continue
			}
			p.op("%s %s %s %s re", num(x+float64(c)*u), num(y+float64(r)*u), num(u), num(u))
		}
	}
	p.op("f")

	bx, by, bs := x+float64(off)*u, y+float64(off)*u, float64(side)*u
	p.roundRect(bx, by, bs, bs, bs*0.16, "#FFFFFF")
	p.marca(bx+bs*markPad, by+bs*markPad, bs*(1-2*markPad), ColorInk, ColorAccent)
}

// --- texto ---

// text escribe una línea con la línea base en (x,y). La matriz de texto vuelve
// a voltear la y para compensar el volteo de la página.
func (p *pdf) text(x, y, size float64, font, hex, s string) {
	p.setFill(hex)
	p.op("BT /%s %s Tf 1 0 0 -1 %s %s Tm (%s) Tj ET",
		font, num(size), num(x), num(y), pdfString(s))
}

// textCentro escribe centrado horizontalmente en cx.
func (p *pdf) textCentro(cx, y, size float64, font, hex, s string) {
	p.text(cx-textWidth(font, size, s)/2, y, size, font, hex, s)
}

// pdfTexto codifica una cadena de METADATOS (título, nombre de capa) como
// literal UTF-16BE con marca de orden. Los literales de la página van en
// WinAnsi porque así lo declara la fuente, pero los metadatos los interpreta el
// lector, y sin la marca los toma como PDFDocEncoding y parte las tildes.
func pdfTexto(s string) string {
	var b strings.Builder
	b.WriteString("FEFF")
	for _, r := range utf16.Encode([]rune(s)) {
		fmt.Fprintf(&b, "%04X", r)
	}
	return "<" + b.String() + ">"
}

// pdfString escapa y recodifica una cadena Go (UTF-8) a WinAnsiEncoding, que es
// lo que declaran las fuentes. Cubre Latin-1 completo, que es todo lo que usan
// las piezas (tildes y eñes); cualquier otro rune cae a '?' antes que romper el
// archivo.
func pdfString(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\\' || r == '(' || r == ')':
			b.WriteByte('\\')
			b.WriteByte(byte(r))
		case r < 0x80 || (r >= 0xA0 && r <= 0xFF):
			b.WriteByte(byte(r))
		default:
			b.WriteByte('?')
		}
	}
	return b.String()
}

// anchoHB son los anchos de Helvetica-Bold (métricas AFM, milésimas de em) de
// los códigos 32..126. Se llevan a mano porque la fuente NO se incrusta: sin
// estos números no se puede centrar un texto ni comprobar que una línea cabe, y
// una línea que se sale no se nota hasta que el vinilo ya está impreso.
var anchoHB = [95]int16{
	278, 333, 474, 556, 556, 889, 722, 238, 333, 333, 389, 584, 278, 333, 278, 278, // 32-47
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 333, 333, 584, 584, 584, 611, // 48-63
	975, 722, 722, 722, 722, 667, 611, 778, 722, 278, 556, 722, 611, 833, 722, 778, // 64-79
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 333, 278, 333, 584, 556, // 80-95
	333, 556, 611, 556, 611, 556, 333, 611, 611, 278, 278, 556, 278, 889, 611, 611, // 96-111
	611, 611, 389, 556, 333, 611, 556, 778, 556, 556, 500, 389, 280, 389, 584, // 112-126
}

// anchoHBAcento son los anchos de los caracteres acentuados que usan las piezas.
// Un acento no cambia el ancho del glifo: son los mismos de la letra base.
var anchoHBAcento = map[rune]int16{
	'á': 556, 'é': 556, 'í': 278, 'ó': 611, 'ú': 611, 'ñ': 611, 'ü': 611,
	'Á': 722, 'É': 667, 'Í': 278, 'Ó': 778, 'Ú': 722, 'Ñ': 722,
	'·': 278, '¡': 333, '¿': 611, '°': 400, '«': 556, '»': 556,
}

// textWidth devuelve el ancho de una línea en las mismas unidades que size.
func textWidth(font string, size float64, s string) float64 {
	total := 0
	for _, r := range s {
		total += int(glifo(font, r))
	}
	return float64(total) / 1000 * size
}

func glifo(font string, r rune) int16 {
	if font == fontMonoBold {
		return 600 // Courier es de paso fijo, y la negrita mide lo mismo
	}
	if r >= 32 && r <= 126 {
		return anchoHB[r-32]
	}
	if w, ok := anchoHBAcento[r]; ok {
		return w
	}
	return 556 // desconocido: el ancho de la '?' con la que se sustituye
}

// fitSize baja el cuerpo hasta que la línea quepa en maxW. Es el cinturón de
// seguridad de la maquetación: los cuerpos se eligen a ojo sobre la medida de la
// pieza, y este recorte garantiza que ninguna línea se salga del arte.
func fitSize(font string, size, maxW float64, s string) float64 {
	if w := textWidth(font, size, s); w > maxW && w > 0 {
		return size * maxW / w
	}
	return size
}

// --- ensamblado del archivo ---

// Números de objeto. Fijos porque el documento es siempre la misma estructura:
// una página, dos fuentes, dos capas y un color plano.
const (
	objCatalog = 1
	objPages   = 2
	objPage    = 3
	objContent = 4
	objFontD   = 5
	objFontM   = 6
	objOCArte  = 7
	objOCCorte = 8
	objKissCS  = 9
	objTint    = 10
	objInfo    = 11
	objCount   = 11
)

// ref escribe una referencia indirecta a un objeto. Se usa SIEMPRE en vez del
// número a pelo: los enlaces entre objetos son una decena, y basta con meter uno
// nuevo en medio para que un literal olvidado apunte a otra cosa y el PDF se
// abra roto sin que ninguna prueba se entere.
func ref(n int) string { return strconv.Itoa(n) + " 0 R" }

// contenido arma el flujo completo: la matriz de página (mm y y hacia abajo) y
// las dos capas envueltas en su marca de contenido opcional.
func (p *pdf) contenido() string {
	var b strings.Builder
	b.WriteString("q\n")
	// mm → pt y volteo vertical: el origen pasa a la esquina superior izquierda.
	fmt.Fprintf(&b, "%s 0 0 %s 0 %s cm\n", num(ptPerMM), num(-ptPerMM), num(p.h*ptPerMM))
	b.WriteString("/OC /ocArte BDC\n")
	b.WriteString(p.arte.String())
	b.WriteString("EMC\n/OC /ocCorte BDC\n")
	b.WriteString(p.corte.String())
	b.WriteString("EMC\nQ\n")
	return b.String()
}

// documento serializa el PDF completo.
func (p *pdf) documento(titulo string) []byte {
	content := p.contenido()
	wpt, hpt := num(p.w*ptPerMM), num(p.h*ptPerMM)
	// MediaBox = TrimBox: el pliego se corta por dentro (kiss-cut), así que no
	// hay demasía que recortar y el borde del archivo ES el borde del material.
	caja := fmt.Sprintf("[0 0 %s %s]", wpt, hpt)

	capas := "[" + ref(objOCArte) + " " + ref(objOCCorte) + "]"
	objs := map[int]string{
		objCatalog: "<< /Type /Catalog /Pages " + ref(objPages) +
			" /OCProperties << /OCGs " + capas + " /D << /Order " + capas + " /ON " + capas + " >> >> >>",
		objPages: "<< /Type /Pages /Kids [" + ref(objPage) + "] /Count 1 >>",
		objPage: "<< /Type /Page /Parent " + ref(objPages) + " /MediaBox " + caja + " /TrimBox " + caja +
			" /BleedBox " + caja + " /CropBox " + caja +
			" /Resources << /Font << /" + fontDisplay + " " + ref(objFontD) +
			" /" + fontMonoBold + " " + ref(objFontM) + " >>" +
			" /ColorSpace << /" + kissCutName + " " + ref(objKissCS) + " >>" +
			" /Properties << /ocArte " + ref(objOCArte) + " /ocCorte " + ref(objOCCorte) + " >> >>" +
			" /Contents " + ref(objContent) + " >>",
		objContent: fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content),
		objFontD:   "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>",
		objFontM:   "<< /Type /Font /Subtype /Type1 /BaseFont /Courier-Bold /Encoding /WinAnsiEncoding >>",
		objOCArte:  "<< /Type /OCG /Name " + pdfTexto("GRABI · arte") + " >>",
		objOCCorte: "<< /Type /OCG /Name " + pdfTexto("GRABI · corte kiss-cut") + " >>",
		objKissCS:  "[/Separation /" + kissCutName + " /DeviceCMYK " + ref(objTint) + "]",
		// El color plano interpola de nada (0) a magenta pleno (1): así el
		// trazado se VE magenta en pantalla pero sale como separación aparte.
		objTint: "<< /FunctionType 2 /Domain [0 1] /C0 [0 0 0 0] /C1 [0 1 0 0] /N 1 >>",
		// Sin /CreationDate a propósito: el mismo id de máquina tiene que dar
		// SIEMPRE el mismo archivo (byte a byte), si no las pruebas y el diff
		// con lo que se mandó a imprimir no sirven de nada.
		objInfo: "<< /Title " + pdfTexto(titulo) + " /Creator (GRABI) /Producer (GRABI kit) /Trapped /False >>",
	}

	var out bytes.Buffer
	out.WriteString("%PDF-1.5\n%\xE2\xE3\xCF\xD3\n")
	offsets := make([]int, objCount+1)
	for i := 1; i <= objCount; i++ {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i, objs[i])
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", objCount+1)
	for i := 1; i <= objCount; i++ {
		fmt.Fprintf(&out, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R /Info %d 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		objCount+1, objInfo, xref)
	return out.Bytes()
}
