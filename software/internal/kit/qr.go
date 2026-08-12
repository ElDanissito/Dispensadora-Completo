// Package kit genera el material físico imprimible de una máquina: el QR que se
// pega en el frente (con la marca compacta encima) y las piezas de calcomanía.
//
// Todo se genera AL VUELO desde el `machine_id`: nada se persiste en disco. El
// QR lleva corrección de errores **nivel H** (Reed–Solomon alto, ~30% de
// redundancia) precisamente porque encima se sobrepone el símbolo de marca; sin
// ese margen el código dejaría de leerse.
//
// Esto es capa nueva de admin: no toca el contrato del token, la conciliación ni
// el flujo público /m/{id}. El QR de la máquina codifica una URL (la página de
// la tienda), no un token firmado.
package kit

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strconv"
	"strings"

	"github.com/skip2/go-qrcode"
)

// Colores de marca (identidad-visual-v1 §4). Se repiten aquí como literales
// porque las piezas son archivos para imprenta, no HTML: no hay variables CSS.
const (
	ColorBG      = "#0A0E0C" // fondo kiosko
	ColorSurface = "#0D1210" // superficie (paneles)
	ColorFG      = "#EAF4EE" // texto sobre oscuro
	ColorMuted   = "#8FA79A"
	ColorAccent  = "#3BE87F" // verde GRABI
	ColorInk     = "#05130B" // tinta sobre claro/verde
	ColorLine    = "#26332B"
	ColorLine2   = "#3A4A40"
	// ColorGhost es la marca gigante de fondo del banner: apenas más clara que el
	// panel. Más contraste y compite con el titular; menos y desaparece al imprimir.
	ColorGhost = "#151E19"
)

// Tagline oficial (identidad-visual-v1 §1). No se traduce ni se reescribe.
const Tagline = "Escanea, paga, agárralo."

// logoFrac es el lado del cuadro blanco central como fracción del lado del
// símbolo (sin la zona de silencio). 0,40 ⇒ el logo tapa 0,40² ≈ 16% del área
// del símbolo (≈10% del área total con la zona de silencio), holgadamente por
// debajo del tope de 20% y muy por debajo del ~30% que recupera el nivel H.
const logoFrac = 0.40

// quietZone son los módulos de margen blanco que go-qrcode ya incluye en
// Bitmap() a cada lado del símbolo.
const quietZone = 4

// Matrix devuelve la matriz de módulos del QR de `content` con ECC nivel H,
// zona de silencio incluida. true = módulo oscuro. Se expone para que las
// pruebas comprueben sobre los mismos datos que se pintan.
func Matrix(content string) ([][]bool, error) {
	q, err := qrcode.New(content, qrcode.Highest)
	if err != nil {
		return nil, fmt.Errorf("generando QR: %w", err)
	}
	return q.Bitmap(), nil
}

// logoBox devuelve el lado (en módulos) y el desplazamiento del cuadro blanco
// central para una matriz de lado n. El lado se fuerza IMPAR igual que el
// símbolo, así el cuadro queda centrado en módulos exactos y no parte ninguno
// por la mitad (un módulo a medias confunde al decodificador más que uno tapado).
func logoBox(n int) (side, off int) {
	symbol := n - 2*quietZone
	side = int(logoFrac * float64(symbol))
	if side%2 != symbol%2 {
		side--
	}
	if side < 1 {
		side = 1
	}
	return side, (n - side) / 2
}

// --- Símbolo de marca (identidad-visual-v1 §3: punto en visor de escaneo) ---
//
// Misma geometría que static/brand/mark.svg, en un lienzo de 100×100: cuatro
// esquinas de visor (trazos con remate redondo) y el punto verde en el centro.

type seg struct{ x1, y1, x2, y2 float64 }

var markSegs = []seg{
	{41, 24, 24, 24}, {24, 24, 24, 41}, // esquina superior izquierda
	{59, 24, 76, 24}, {76, 24, 76, 41}, // superior derecha
	{41, 76, 24, 76}, {24, 76, 24, 59}, // inferior izquierda
	{59, 76, 76, 76}, {76, 76, 76, 59}, // inferior derecha
}

const (
	markStroke = 7.5 // grosor del visor en el lienzo de 100
	markDotR   = 13  // radio del punto verde
	// markPad es el aire entre el cuadro blanco y la marca. Va pequeño porque el
	// propio lienzo de la marca ya reserva margen (el visor va de 24 a 76).
	markPad = 0.04
)

// --- SVG ---

// SVG devuelve el QR como SVG (unidades = módulos, así el archivo escala a
// cualquier tamaño de impresión sin perder nitidez). px fija los atributos
// width/height para la previsualización en pantalla; el viewBox manda.
func SVG(content string, px int) ([]byte, error) {
	m, err := Matrix(content)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" role="img" aria-label="QR de la máquina">`,
		len(m), len(m), px, px)
	b.WriteString("\n")
	writeQRBody(&b, m, 0, 0, float64(len(m)))
	b.WriteString("</svg>\n")
	return []byte(b.String()), nil
}

// writeQRBody pinta el QR (fondo blanco + módulos + marca) dentro del cuadrado
// (x,y,size) del SVG que lo contiene. Se reutiliza en las calcomanías.
func writeQRBody(b *strings.Builder, m [][]bool, x, y, size float64) {
	n := len(m)
	u := size / float64(n) // lado de un módulo en unidades del contenedor
	fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" fill="#FFFFFF"/>`+"\n", f(x), f(y), f(size), f(size))

	side, off := logoBox(n)
	// Los módulos van con crispEdges: sin él, el antialiasing deja costuras
	// blancas entre módulos vecinos y el lector pierde contraste.
	b.WriteString(`<g shape-rendering="crispEdges" fill="` + ColorInk + `">` + "\n")
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			if !m[r][c] {
				continue
			}
			// Los módulos bajo el logo no se pintan: van tapados igual y así el
			// archivo pesa menos.
			if r >= off && r < off+side && c >= off && c < off+side {
				continue
			}
			fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s"/>`,
				f(x+float64(c)*u), f(y+float64(r)*u), f(u), f(u))
		}
		b.WriteString("\n")
	}
	b.WriteString("</g>\n")

	// Cuadro blanco OPACO + marca compacta encima.
	bx, by, bs := x+float64(off)*u, y+float64(off)*u, float64(side)*u
	fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="#FFFFFF"/>`+"\n",
		f(bx), f(by), f(bs), f(bs), f(bs*0.16))
	writeMark(b, bx+bs*markPad, by+bs*markPad, bs*(1-2*markPad), ColorInk)
}

// writeMark pinta el símbolo compacto (visor + punto) en el cuadrado (x,y,size).
func writeMark(b *strings.Builder, x, y, size float64, stroke string) {
	fmt.Fprintf(b, `<svg x="%s" y="%s" width="%s" height="%s" viewBox="0 0 100 100" overflow="visible">`,
		f(x), f(y), f(size), f(size))
	b.WriteString(`<path d="M41 24 L24 24 L24 41 M59 24 L76 24 L76 41 M41 76 L24 76 L24 59 M59 76 L76 76 L76 59" fill="none" stroke="` +
		stroke + `" stroke-width="7.5" stroke-linecap="round" stroke-linejoin="round"/>`)
	b.WriteString(`<circle cx="50" cy="50" r="13" fill="` + ColorAccent + `"/>`)
	b.WriteString("</svg>\n")
}

// f formatea un número para SVG sin ceros de más.
func f(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// --- PNG ---

// PNG devuelve el QR rasterizado. El lado real se redondea a un múltiplo entero
// del número de módulos: con un tamaño de módulo fraccionario unos módulos
// saldrían de 3 px y otros de 4 y el lector pierde el paso.
func PNG(content string, px int) ([]byte, error) {
	m, err := Matrix(content)
	if err != nil {
		return nil, err
	}
	n := len(m)
	scale := px / n
	if scale < 1 {
		scale = 1
	}
	side := scale * n
	img := image.NewRGBA(image.Rect(0, 0, side, side))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	ink := hexColor(ColorInk)
	boxSide, off := logoBox(n)
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			if !m[r][c] {
				continue
			}
			if r >= off && r < off+boxSide && c >= off && c < off+boxSide {
				continue
			}
			draw.Draw(img, image.Rect(c*scale, r*scale, (c+1)*scale, (r+1)*scale),
				image.NewUniform(ink), image.Point{}, draw.Src)
		}
	}

	bx, bs := float64(off*scale), float64(boxSide*scale)
	drawRoundRect(img, bx, bx, bs, bs, bs*0.16, color.White)
	drawMark(img, bx+bs*markPad, bx+bs*markPad, bs*(1-2*markPad), ink)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("codificando PNG: %w", err)
	}
	return buf.Bytes(), nil
}

// drawMark rasteriza el símbolo compacto en el cuadrado (x,y,size) con
// antialias, para que a tamaños pequeños el visor no salga dentado.
func drawMark(img *image.RGBA, x, y, size float64, stroke color.RGBA) {
	s := size / 100 // unidades del lienzo de la marca → píxeles
	half := markStroke * s / 2
	dot := hexColor(ColorAccent)
	x0, y0 := int(x), int(y)
	x1, y1 := int(math.Ceil(x+size)), int(math.Ceil(y+size))
	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			cx, cy := float64(px)+0.5, float64(py)+0.5
			// Punto verde: va encima del visor.
			if a := cov(markDotR*s - math.Hypot(cx-(x+50*s), cy-(y+50*s))); a > 0 {
				blend(img, px, py, dot, a)
				continue
			}
			d := math.Inf(1)
			for _, sg := range markSegs {
				d = math.Min(d, distSeg(cx, cy, x+sg.x1*s, y+sg.y1*s, x+sg.x2*s, y+sg.y2*s))
			}
			if a := cov(half - d); a > 0 {
				blend(img, px, py, stroke, a)
			}
		}
	}
}

// drawRoundRect pinta un rectángulo de esquinas redondeadas con antialias.
func drawRoundRect(img *image.RGBA, x, y, w, h, r float64, c color.Color) {
	rc := color.RGBAModel.Convert(c).(color.RGBA)
	for py := int(y); py < int(math.Ceil(y+h)); py++ {
		for px := int(x); px < int(math.Ceil(x+w)); px++ {
			cx, cy := float64(px)+0.5, float64(py)+0.5
			// Distancia con signo a un rectángulo redondeado.
			qx := math.Abs(cx-(x+w/2)) - (w/2 - r)
			qy := math.Abs(cy-(y+h/2)) - (h/2 - r)
			d := math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - r
			if a := cov(-d); a > 0 {
				blend(img, px, py, rc, a)
			}
		}
	}
}

// cov convierte una distancia con signo (en píxeles) en cobertura 0..1.
func cov(d float64) float64 {
	return math.Max(0, math.Min(1, d+0.5))
}

func blend(img *image.RGBA, x, y int, c color.RGBA, a float64) {
	if !(image.Point{x, y}).In(img.Bounds()) {
		return
	}
	dst := img.RGBAAt(x, y)
	mix := func(s, d uint8) uint8 { return uint8(float64(s)*a + float64(d)*(1-a) + 0.5) }
	img.SetRGBA(x, y, color.RGBA{mix(c.R, dst.R), mix(c.G, dst.G), mix(c.B, dst.B), 255})
}

// distSeg es la distancia de (px,py) al segmento (ax,ay)-(bx,by).
func distSeg(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	l2 := dx*dx + dy*dy
	if l2 == 0 {
		return math.Hypot(px-ax, py-ay)
	}
	t := math.Max(0, math.Min(1, ((px-ax)*dx+(py-ay)*dy)/l2))
	return math.Hypot(px-(ax+t*dx), py-(ay+t*dy))
}

// hexColor convierte "#RRGGBB" en color.RGBA (los literales son de la guía de
// marca, fijos en el código; una cadena mal formada es un error de programación).
func hexColor(h string) color.RGBA {
	v, err := strconv.ParseUint(strings.TrimPrefix(h, "#"), 16, 32)
	if err != nil {
		panic("kit: color de marca inválido: " + h)
	}
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}
}
