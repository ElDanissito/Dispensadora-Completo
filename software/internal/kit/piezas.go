package kit

// Piezas imprimibles del kit físico de una máquina. Son plantillas de texto que
// producen SVG en milímetros (1 unidad = 1 mm), así lo que ve la imprenta es el
// tamaño real. El QR va incrustado como SVG anidado, nunca como imagen enlazada:
// el archivo tiene que poder abrirse solo.

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/template"
)

// SiteDefault es la base pública del sitio (ADR-019). Se puede sobreescribir
// desde el servidor (GRABI_SITE_URL) para pruebas.
const SiteDefault = "https://grabi.napi.lat"

// PNGZipSize es el lado del qr.png que va en el ZIP: suficiente para pegarlo en
// una pieza digital sin que se vea pixelado.
const PNGZipSize = 1024

// Machine son los datos de una máquina que necesitan las piezas.
type Machine struct {
	ID    string // identificador, ej. "M001"
	Place string // punto/ciudad donde vive la máquina (el "nombre" del panel)
	Site  string // base del sitio; "" ⇒ SiteDefault
}

// URL devuelve lo que codifica el QR: la página pública de la máquina.
func (m Machine) URL() string {
	site := strings.TrimRight(m.Site, "/")
	if site == "" {
		site = SiteDefault
	}
	return site + "/m/" + m.ID
}

// short devuelve la URL sin esquema, para imprimirla legible bajo el QR.
func (m Machine) short() string {
	u := m.URL()
	u = strings.TrimPrefix(u, "https://")
	return strings.TrimPrefix(u, "http://")
}

// datos es lo que reciben las plantillas, con todo el texto ya escapado para XML.
type datos struct {
	ID      string
	Place   string
	Short   string
	Tagline string
	Tag1    string // el tagline partido en dos líneas para las piezas angostas
	Tag2    string
	QR      string // fragmento SVG del QR, ya posicionado
	// Colores de marca, para no repetir literales en cada plantilla.
	BG, FG, Muted, Accent, Line string
}

func (m Machine) datos(qr string) datos {
	// El tagline se parte en dos líneas para las piezas angostas, pero no se
	// reescribe (identidad-visual-v1 §1). Si algún día cambia y no tiene por
	// dónde partirse, cae entero en la primera línea.
	tag1, tag2 := Tagline, ""
	if a, b, ok := strings.Cut(Tagline, " paga, "); ok {
		tag1, tag2 = a+" paga,", b
	}
	return datos{
		ID: xmlEsc(m.ID), Place: xmlEsc(m.Place), Short: xmlEsc(m.short()),
		Tagline: xmlEsc(Tagline), Tag1: xmlEsc(tag1), Tag2: xmlEsc(tag2), QR: qr,
		BG: ColorBG, FG: ColorFG, Muted: ColorMuted, Accent: ColorAccent, Line: ColorLine,
	}
}

// Marca devuelve el símbolo compacto (visor + punto) del tamaño pedido, en las
// unidades de la pieza que lo llama.
func (d datos) Marca(size float64) string {
	return fmt.Sprintf(`<svg width="%s" height="%s" viewBox="0 0 100 100" overflow="visible">`+
		`<path d="M41 24 L24 24 L24 41 M59 24 L76 24 L76 41 M41 76 L24 76 L24 59 M59 76 L76 76 L76 59" `+
		`fill="none" stroke="%s" stroke-width="7.5" stroke-linecap="round" stroke-linejoin="round"/>`+
		`<circle cx="50" cy="50" r="13" fill="%s"/></svg>`, f(size), f(size), d.FG, d.Accent)
}

// xmlEsc escapa el texto que entra en las piezas. El nombre de la máquina lo
// escribe el admin: sin escapar, un `&` o un `<` rompería el SVG. Las plantillas
// son text/template (html/template mangla el SVG), así que el escape es aquí.
var escapador = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")

func xmlEsc(s string) string { return escapador.Replace(s) }

// qrFragment devuelve el QR como SVG anidado dentro del cuadrado (x,y,size) en
// las unidades (mm) de la pieza que lo contiene.
func qrFragment(content string, x, y, size float64) (string, error) {
	m, err := Matrix(content)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	writeQRBody(&b, m, x, y, size)
	return b.String(), nil
}

// Los tamaños de letra van holgados a propósito: si la imprenta no tiene Archivo
// ni IBM Plex Mono, el sustituto (Arial Black, Courier) es más ancho, y una línea
// que se sale del arte no se nota hasta que el vinilo ya está impreso.
var piezas = template.Must(template.New("piezas").Parse(`
{{define "sticker-frente"}}<svg xmlns="http://www.w3.org/2000/svg" width="100mm" height="140mm" viewBox="0 0 100 140">
  <rect width="100" height="140" rx="6" fill="{{.BG}}"/>
  <rect x="1.5" y="1.5" width="97" height="137" rx="4.5" fill="none" stroke="{{.Line}}" stroke-width="0.5"/>

  <text x="50" y="20" text-anchor="middle" font-family="Archivo, Arial Black, sans-serif" font-weight="900" font-size="14" letter-spacing="0.4" fill="{{.FG}}">GRABI<tspan fill="{{.Accent}}">.</tspan></text>
  <text x="50" y="27.5" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="3.2" letter-spacing="0.8" fill="{{.Muted}}">PAGA CON BRE-B · SIN EFECTIVO</text>

  <rect x="17" y="34" width="66" height="66" rx="5" fill="#FFFFFF"/>
  {{.QR}}

  <text x="50" y="112" text-anchor="middle" font-family="Archivo, Arial Black, sans-serif" font-weight="900" font-size="10" fill="{{.FG}}">{{.Tag1}}</text>
  <text x="50" y="123" text-anchor="middle" font-family="Archivo, Arial Black, sans-serif" font-weight="900" font-size="10" fill="{{.FG}}">{{.Tag2}}</text>
  <text x="50" y="131" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="4" fill="{{.Accent}}">{{.Short}}</text>
  <text x="50" y="136.5" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="2.8" letter-spacing="0.8" fill="{{.Muted}}">GRABI {{.ID}}</text>
</svg>
{{end}}

{{define "placa"}}<svg xmlns="http://www.w3.org/2000/svg" width="90mm" height="30mm" viewBox="0 0 90 30">
  <rect width="90" height="30" rx="3" fill="{{.BG}}"/>
  <rect width="3" height="30" rx="1.5" fill="{{.Accent}}"/>
  <text x="9" y="15" font-family="Archivo, Arial Black, sans-serif" font-weight="900" font-size="8" letter-spacing="0.3" fill="{{.FG}}">GRABI {{.ID}}</text>
  <text x="9" y="22" font-family="IBM Plex Mono, monospace" font-size="3.4" letter-spacing="0.4" fill="{{.Muted}}">· {{.Place}}</text>
  <g transform="translate(70,8)">{{.Marca 14}}</g>
</svg>
{{end}}

{{define "wrap-lateral"}}<svg xmlns="http://www.w3.org/2000/svg" width="60mm" height="400mm" viewBox="0 0 60 400">
  <rect width="60" height="400" fill="{{.BG}}"/>
  <rect x="0" y="0" width="60" height="5" fill="{{.Accent}}"/>
  <g transform="translate(16,22)">{{.Marca 28}}</g>
  <text transform="translate(36,362) rotate(-90)" font-family="Archivo, Arial Black, sans-serif" font-weight="900" font-size="15" fill="{{.FG}}">{{.Tagline}}</text>
  <text transform="translate(50,362) rotate(-90)" font-family="IBM Plex Mono, monospace" font-size="4.6" letter-spacing="1.2" fill="{{.Muted}}">{{.Short}}</text>
  <rect x="0" y="395" width="60" height="5" fill="{{.Accent}}"/>
</svg>
{{end}}
`))

// pieza renderiza una plantilla con el QR ya incrustado (si la pieza lo lleva).
func (m Machine) pieza(name string, qr string) ([]byte, error) {
	var buf bytes.Buffer
	if err := piezas.ExecuteTemplate(&buf, name, m.datos(qr)); err != nil {
		return nil, fmt.Errorf("render de %s: %w", name, err)
	}
	return bytes.TrimSpace(buf.Bytes()), nil
}

// StickerFrente es la calcomanía principal del frente de la máquina (100×140 mm):
// marca + QR + tagline.
func (m Machine) StickerFrente() ([]byte, error) {
	// El QR ocupa 60 mm dentro del panel blanco de 66 mm (3 mm de aire por lado).
	qr, err := qrFragment(m.URL(), 20, 37, 60)
	if err != nil {
		return nil, err
	}
	return m.pieza("sticker-frente", qr)
}

// Placa es la plaquita identificadora de la máquina (90×30 mm).
func (m Machine) Placa() ([]byte, error) { return m.pieza("placa", "") }

// WrapLateral es la tira del costado de la máquina (60×400 mm).
func (m Machine) WrapLateral() ([]byte, error) { return m.pieza("wrap-lateral", "") }

// leeme explica a la imprenta lo que no se ve en los archivos.
const leeme = `KIT FÍSICO — GRABI %s (%s)
QR: %s

CONTENIDO
  qr.svg              El QR solo, vectorial. Úsalo para imprenta.
  qr.png              El mismo QR a %d px. Para piezas digitales (WhatsApp, redes).
  sticker-frente.svg  Calcomanía del frente, 100 x 140 mm.
  placa.svg           Plaquita identificadora, 90 x 30 mm.
  wrap-lateral.svg    Tira del costado, 60 x 400 mm.

IMPRESIÓN
  · Los SVG están en milímetros a tamaño real: imprime al 100%%, sin "ajustar a la página".
  · El QR NO se puede estirar, recortar ni cambiar de color. El blanco alrededor
    (zona de silencio) es parte del código: si lo recortas deja de leerse.
  · Tamaño mínimo del QR impreso: 35 mm de lado. En el frente va a 60 mm.
  · Tipografías: Archivo (900) para titulares e IBM Plex Mono para los datos. Si la
    imprenta no las tiene, conviértelas a curvas antes de enviar el archivo.
  · Material sugerido para el frente: vinilo con laminado mate (el brillo dificulta
    el escaneo bajo luz directa).

VERIFICA ANTES DE PEGAR
  Escanea el QR impreso con la cámara del celular: debe abrir %s
`

// leemeBytes arma el LEEME.txt del ZIP. El tamaño del PNG se calcula, no se
// asume: el lado real se redondea a un múltiplo entero de módulos.
func (m Machine) leemeBytes() []byte {
	px := PNGZipSize
	if mat, err := Matrix(m.URL()); err == nil && len(mat) > 0 {
		px = (PNGZipSize / len(mat)) * len(mat)
	}
	return []byte(fmt.Sprintf(leeme, m.ID, m.Place, m.URL(), px, m.URL()))
}

// ZipFiles es el orden y los nombres de las piezas dentro del kit.zip.
var ZipFiles = []string{"qr.svg", "qr.png", "sticker-frente.svg", "placa.svg", "wrap-lateral.svg", "LEEME.txt"}

// WriteZip escribe el kit completo de la máquina como ZIP. Se genera al vuelo en
// cada petición: no hay nada que guardar ni que invalidar si cambia la marca.
func (m Machine) WriteZip(w io.Writer) error {
	svg, err := SVG(m.URL(), 1024)
	if err != nil {
		return err
	}
	pngBytes, err := PNG(m.URL(), PNGZipSize)
	if err != nil {
		return err
	}
	frente, err := m.StickerFrente()
	if err != nil {
		return err
	}
	placa, err := m.Placa()
	if err != nil {
		return err
	}
	wrap, err := m.WrapLateral()
	if err != nil {
		return err
	}
	contenido := map[string][]byte{
		"qr.svg": svg, "qr.png": pngBytes, "sticker-frente.svg": frente,
		"placa.svg": placa, "wrap-lateral.svg": wrap, "LEEME.txt": m.leemeBytes(),
	}

	zw := zip.NewWriter(w)
	for _, name := range ZipFiles {
		f, err := zw.Create(name)
		if err != nil {
			return fmt.Errorf("zip %s: %w", name, err)
		}
		if _, err := f.Write(contenido[name]); err != nil {
			return fmt.Errorf("zip %s: %w", name, err)
		}
	}
	return zw.Close()
}
