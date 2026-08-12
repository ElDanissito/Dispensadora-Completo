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
	// El tagline en tres líneas ("Escanea," / "paga," / "agárralo."), como en el
	// mockup: las dos primeras en blanco y la última en verde.
	Tag1, Tag2, Tag3 string
	// Colores de marca, para no repetir literales en cada plantilla.
	BG, FG, Muted, Accent, Line string
}

func (m Machine) datos() datos {
	// El tagline se parte por palabras, no se reescribe (identidad-visual-v1 §1).
	// Si algún día deja de tener tres palabras, cae entero en la primera línea.
	tags := [3]string{Tagline}
	if w := strings.Fields(Tagline); len(w) == 3 {
		tags = [3]string{w[0], w[1], w[2]}
	}
	return datos{
		ID: xmlEsc(m.ID), Place: xmlEsc(m.Place), Short: xmlEsc(m.short()),
		Tagline: xmlEsc(Tagline),
		Tag1:    xmlEsc(tags[0]), Tag2: xmlEsc(tags[1]), Tag3: xmlEsc(tags[2]),
		BG: ColorBG, FG: ColorFG, Muted: ColorMuted, Accent: ColorAccent, Line: ColorLine,
	}
}

// Marca es el símbolo compacto suelto: visor claro + punto verde. Va sobre el
// fondo oscuro de la pieza (identidad-visual-v1 §3).
func (d datos) Marca(size float64) string {
	return marcaSVG(size, "", d.FG, d.Accent)
}

// MarcaBadge es el símbolo dentro del cuadro verde. Sobre el verde acento
// siempre va tinta oscura, nunca clara (identidad-visual-v1 §4).
func (d datos) MarcaBadge(size float64) string {
	return marcaSVG(size, d.Accent, ColorInk, ColorInk)
}

// MarcaFantasma es la marca gigante de fondo: apenas más clara que el panel, para
// que dé textura sin competir con el titular.
func (d datos) MarcaFantasma(size float64) string {
	return marcaSVG(size, "", ColorGhost, ColorGhost)
}

// marcaSVG dibuja el símbolo compacto del tamaño pedido, en las unidades de la
// pieza que lo llama. bg vacío ⇒ sin cuadro de fondo.
func marcaSVG(size float64, bg, stroke, dot string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<svg width="%s" height="%s" viewBox="0 0 100 100" overflow="visible">`, f(size), f(size))
	if bg != "" {
		fmt.Fprintf(&b, `<rect width="100" height="100" rx="22" fill="%s"/>`, bg)
	}
	fmt.Fprintf(&b, `<path d="M41 24 L24 24 L24 41 M59 24 L76 24 L76 41 M41 76 L24 76 L24 59 M59 76 L76 76 L76 59" `+
		`fill="none" stroke="%s" stroke-width="7.5" stroke-linecap="round" stroke-linejoin="round"/>`+
		`<circle cx="50" cy="50" r="13" fill="%s"/></svg>`, stroke, dot)
	return b.String()
}

// xmlEsc escapa el texto que entra en las piezas. El nombre de la máquina lo
// escribe el admin: sin escapar, un `&` o un `<` rompería el SVG. Las plantillas
// son text/template (html/template mangla el SVG), así que el escape es aquí.
var escapador = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")

func xmlEsc(s string) string { return escapador.Replace(s) }

// Arte según el mockup de Daniel (2026-08-11): composición apaisada y alineada a
// la IZQUIERDA, filetes verdes arriba y abajo, tagline en tres líneas con la
// última en verde, marca compacta en cuadro verde y marca fantasma de fondo.
//
// Los tamaños de letra van holgados a propósito: si la imprenta no tiene Archivo
// ni IBM Plex Mono, el sustituto (Arial Black, Courier) es más ancho, y una línea
// que se sale del arte no se nota hasta que el vinilo ya está impreso. Por lo
// mismo, el `· {id}` de la placa va como tspan del wordmark y no en una posición
// fija: así no se solapan si la tipografía cambia de ancho.
var piezas = template.Must(template.New("piezas").Parse(`
{{define "sticker-frente"}}<svg xmlns="http://www.w3.org/2000/svg" width="400mm" height="185mm" viewBox="0 0 400 185">
  <rect width="400" height="185" fill="{{.BG}}"/>
  <g transform="translate(268,37)">{{.MarcaFantasma 110}}</g>
  <rect x="0" y="0" width="400" height="2.5" fill="{{.Accent}}"/>
  <rect x="0" y="182.5" width="400" height="2.5" fill="{{.Accent}}"/>

  <g transform="translate(26,20)">{{.MarcaBadge 32}}</g>
  <text x="26" y="83" font-family="Archivo, Arial Black, sans-serif" font-weight="900" font-size="30" fill="{{.FG}}">{{.Tag1}}</text>
  <text x="26" y="112" font-family="Archivo, Arial Black, sans-serif" font-weight="900" font-size="30" fill="{{.FG}}">{{.Tag2}}</text>
  <text x="26" y="141" font-family="Archivo, Arial Black, sans-serif" font-weight="900" font-size="30" fill="{{.Accent}}">{{.Tag3}}</text>
  <text x="26" y="159" font-family="IBM Plex Mono, monospace" font-size="9" letter-spacing="0.6" fill="{{.Muted}}">Sin efectivo · sin datáfono · pago Bre-B</text>
</svg>
{{end}}

{{define "placa"}}<svg xmlns="http://www.w3.org/2000/svg" width="90mm" height="30mm" viewBox="0 0 90 30">
  <g transform="translate(8,10)">{{.Marca 10}}</g>
  <text x="22" y="19" font-family="Archivo, Arial Black, sans-serif" font-weight="900" font-size="10.5" letter-spacing="0.2" fill="{{.FG}}">GRABI<tspan fill="{{.Accent}}">.</tspan><tspan dx="4" font-family="IBM Plex Mono, monospace" font-weight="400" font-size="3.4" letter-spacing="0.4" fill="{{.Muted}}">· {{.ID}}</tspan></text>
</svg>
{{end}}

{{define "wrap-lateral"}}<svg xmlns="http://www.w3.org/2000/svg" width="60mm" height="400mm" viewBox="0 0 60 400">
  <rect width="60" height="400" fill="{{.BG}}"/>
  <rect x="0" y="0" width="60" height="5" fill="{{.Accent}}"/>
  <rect x="0" y="395" width="60" height="5" fill="{{.Accent}}"/>
  <g transform="translate(16,24)">{{.MarcaBadge 28}}</g>
  {{/* En una tira de 60 mm las tres líneas apiladas del banner no caben: el
       tagline va en UNA línea vertical grande, con la última palabra en verde.
       El cuerpo va a 20 para que, si la imprenta sustituye Archivo por una más
       ancha, la línea siga sin chocar con el badge de arriba. */}}
  <text transform="translate(35,380) rotate(-90)" font-family="Archivo, Arial Black, sans-serif" font-weight="900" font-size="20" fill="{{.FG}}">{{.Tag1}} {{.Tag2}} <tspan fill="{{.Accent}}">{{.Tag3}}</tspan></text>
  <text x="30" y="388" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="4" letter-spacing="0.4" fill="{{.Muted}}">{{.Short}}</text>
</svg>
{{end}}
`))

// pieza renderiza una plantilla de calcomanía.
func (m Machine) pieza(name string) ([]byte, error) {
	var buf bytes.Buffer
	if err := piezas.ExecuteTemplate(&buf, name, m.datos()); err != nil {
		return nil, fmt.Errorf("render de %s: %w", name, err)
	}
	return bytes.TrimSpace(buf.Bytes()), nil
}

// StickerFrente es el banner del frente de la máquina (400×185 mm): marca,
// tagline y bajada. NO lleva el QR: el QR se pega aparte, desde qr.svg, a la
// altura de la mano y donde el celular lo pueda encuadrar sin agacharse.
func (m Machine) StickerFrente() ([]byte, error) { return m.pieza("sticker-frente") }

// Placa es la plaquita identificadora de la máquina (90×30 mm). Va SIN fondo:
// se imprime en vinilo transparente o se aplica directa sobre el cuerpo.
func (m Machine) Placa() ([]byte, error) { return m.pieza("placa") }

// WrapLateral es la tira del costado de la máquina (60×400 mm).
func (m Machine) WrapLateral() ([]byte, error) { return m.pieza("wrap-lateral") }

// leeme explica a la imprenta lo que no se ve en los archivos.
const leeme = `KIT FÍSICO — GRABI %s (%s)
QR: %s

CONTENIDO
  qr.svg              El QR solo, vectorial. Es LA pieza que hace vender: imprímela
                      aparte y pégala a la altura de la mano, junto al banner.
  qr.png              El mismo QR a %d px. Para piezas digitales (WhatsApp, redes).
  sticker-frente.svg  Banner del frente, 400 x 185 mm. Sin QR: solo marca y mensaje.
  placa.svg           Plaquita identificadora, 90 x 30 mm. SIN FONDO: va en vinilo
                      transparente (o impresa directa) sobre el cuerpo de la máquina.
  wrap-lateral.svg    Tira del costado, 60 x 400 mm.

IMPRESIÓN
  · Los SVG están en milímetros a tamaño real: imprime al 100%%, sin "ajustar a la página".
  · El QR NO se puede estirar, recortar ni cambiar de color. El blanco alrededor
    (zona de silencio) es parte del código: si lo recortas deja de leerse.
  · Tamaño mínimo del QR impreso: 35 mm de lado. Recomendado: 60 a 90 mm.
  · Tipografías: Archivo (900) para titulares e IBM Plex Mono para los datos. Si la
    imprenta no las tiene, conviértelas a curvas antes de enviar el archivo.
  · Material sugerido: vinilo con laminado mate (el brillo dificulta el escaneo bajo
    luz directa y ensucia la lectura del banner).

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
