package kit

// Hoja de imposición: las seis piezas de vinilo de UNA máquina colocadas en un
// solo pliego, a escala 1:1 y con las guías de kiss-cut en su propia capa, lista
// para mandar a la imprenta sin que nadie tenga que maquetar nada.
//
// Por qué un pliego y no seis archivos: la imprenta de vinilo cobra por área
// mínima. Seis archivos sueltos son seis mínimos; un pliego es uno. Y el
// kiss-cut deja los stickers cortados por el contorno pero pegados al respaldo,
// así que llegan en una sola lámina que se va despegando pieza por pieza.
//
// El pliego NO lleva QRs de otras máquinas (ADR-027): lo que se imprime aquí
// solo sirve para esta máquina.

import (
	"strconv"
	"strings"
)

// Medidas del pliego y de la retícula, en milímetros.
const (
	// ImpAncho es el ancho del pliego: el mínimo de facturación de la imprenta.
	ImpAncho = 1000.0
	// ImpAlto son 320 mm y no los 300 del mínimo porque las alturas de las
	// piezas ya suman 300 exactos (180 de la fila de arriba + 70 de la cabecera
	// + 50 de la placa): a 300 no cabrían ni el margen ni las separaciones. 320
	// es el alto más cercano que respeta la retícula, y sigue por encima del
	// mínimo de la imprenta, así que no cambia el precio del pedido.
	ImpAlto = 320.0
	// ImpMargen es el aire al borde del pliego (la zona que las cuchillas no
	// pisan). ImpGap es la separación entre piezas vecinas.
	ImpMargen = 5.0
	ImpGap    = 3.0
	// ImpRadio redondea la esquina del kiss-cut: una esquina en punto no se
	// despega bien del respaldo y se levanta con el uso.
	ImpRadio = 3.0
	// ImpCorte es el grosor del trazado de corte.
	ImpCorte = 0.25
)

// Medidas físicas de cada pieza (mm). Son las que se imprimen: cambiarlas es
// cambiar el sticker, no la maquetación.
const (
	impWrapAncho = 450.0
	impWrapAlto  = 180.0
	impPasosW    = 80.0
	impPasosH    = 180.0
	impCabAncho  = 280.0
	impCabAlto   = 70.0
	impPlacaW    = 250.0
	impPlacaH    = 50.0
	impQRLado    = 100.0
)

// CopySinEfectivo es el argumento de venta, línea a línea y con el punto final,
// tal cual está en instrucciones.svg: la tercera va en verde. Es copy de marca,
// no se reescribe ni se recompone con separadores.
//
// Vive aquí porque el pliego lo compone pieza a pieza; instrucciones.svg lo
// lleva literal dentro de su plantilla. Que no se separen lo vigila
// TestElCopyDelWrapDerechoEsElDeInstruccionesSVG.
var CopySinEfectivo = [3]string{"Sin efectivo.", "Sin datáfono.", "Solo tu celular."}

// PasosInstrucciones son los tres pasos tal como los parte instrucciones.svg:
// dos líneas por paso, sin título suelto. Es el guion que lee quien nunca ha
// comprado en una GRABI.
var PasosInstrucciones = [3][2]string{
	{"Escanea el QR de", "la máquina"},
	{"Paga con Bre-B", "desde tu banco"},
	{"Muestra el QR y", "agárralo"},
}

// Geometría del wrap derecho, que replica instrucciones.svg.
const (
	// barraAncho es la barra verde vertical del canto. 6 mm en una pieza de 450
	// es la misma proporción que los 4 mm de instrucciones.svg sobre 300.
	barraAncho = 6.0
	// divisorX es el filete que separa el argumento de venta de los tres pasos.
	divisorX = 252.0
)

// PiezaImp es una pieza ya colocada en el pliego. Ancho/Alto son las medidas
// FÍSICAS del sticker y (X,Y) su esquina superior izquierda, ambas en mm.
type PiezaImp struct {
	Nombre      string
	Ancho, Alto float64
	X, Y        float64
}

// piezaImp añade a la pieza colocada cómo se dibuja.
type piezaImp struct {
	PiezaImp
	dibuja func(p *pdf, c PiezaImp, m Machine, mat [][]bool)
}

// piezasImp coloca las seis piezas. La retícula se CALCULA a partir de márgenes
// y separaciones en vez de llevar coordenadas a mano: si mañana cambia una
// medida, el pliego se recoloca solo y no queda una pieza pisando a otra.
//
// Layout (decidido con Daniel, identidad-visual-v1 §8):
//
//	fila de arriba : wrap izquierdo · wrap derecho · panel de 3 pasos
//	fila de abajo  : cabecera + placa apiladas, y el QR a su derecha
func piezasImp() []piezaImp {
	fila1 := ImpMargen
	fila2 := fila1 + impWrapAlto + ImpGap

	x := ImpMargen
	izq := PiezaImp{"wrap-izquierdo", impWrapAncho, impWrapAlto, x, fila1}
	x += impWrapAncho + ImpGap
	der := PiezaImp{"wrap-derecho", impWrapAncho, impWrapAlto, x, fila1}
	x += impWrapAncho + ImpGap
	pasos := PiezaImp{"instrucciones-3-pasos", impPasosW, impPasosH, x, fila1}

	cab := PiezaImp{"cabecera-grabi", impCabAncho, impCabAlto, ImpMargen, fila2}
	placa := PiezaImp{"placa", impPlacaW, impPlacaH, ImpMargen, fila2 + impCabAlto + ImpGap}
	qr := PiezaImp{"qr", impQRLado, impQRLado, ImpMargen + impCabAncho + ImpGap, fila2}

	return []piezaImp{
		{izq, dibujaWrapIzq},
		{der, dibujaWrapDer},
		{pasos, dibujaPasos},
		{cab, dibujaCabecera},
		{placa, dibujaPlaca},
		{qr, dibujaQRPieza},
	}
}

// PiezasImposicion devuelve las seis piezas del pliego con su medida y posición.
// Se expone para que las pruebas comprueben la retícula sobre la misma tabla que
// se dibuja, y para poder listarla en el panel sin duplicar números.
func PiezasImposicion() []PiezaImp {
	ps := piezasImp()
	out := make([]PiezaImp, len(ps))
	for i, p := range ps {
		out[i] = p.PiezaImp
	}
	return out
}

// Imposicion devuelve el pliego completo en PDF.
func (m Machine) Imposicion() ([]byte, error) {
	mat, err := Matrix(m.URL())
	if err != nil {
		return nil, err
	}
	p := newPDF(ImpAncho, ImpAlto)
	piezas := piezasImp()

	p.capaArte()
	for _, pz := range piezas {
		pz.dibuja(p, pz.PiezaImp, m, mat)
	}

	// Las guías van en su propia capa y en un color PLANO llamado "KissCut": el
	// plóter de corte busca una separación con nombre, no un magenta de
	// cuatricromía (que se imprimiría como tinta encima del arte).
	p.capaCorte()
	p.op("q /%s CS 1 SCN %s w", kissCutName, num(ImpCorte))
	for _, pz := range piezas {
		p.roundRectPath(pz.X, pz.Y, pz.Ancho, pz.Alto, ImpRadio)
		p.op("S")
	}
	p.op("Q")

	return p.documento("GRABI " + m.ID + " · hoja de imposición"), nil
}

// --- piezas ---

// fondoKiosko pinta el fondo oscuro y los dos filetes verdes de la familia
// (identidad-visual-v1 §8, addendum de ADR-026).
func fondoKiosko(p *pdf, c PiezaImp, filete float64) {
	p.rect(c.X, c.Y, c.Ancho, c.Alto, ColorBG)
	p.rect(c.X, c.Y, c.Ancho, filete, ColorAccent)
	p.rect(c.X, c.Y+c.Alto-filete, c.Ancho, filete, ColorAccent)
}

// taglineLineas parte el tagline en sus tres palabras. Si algún día deja de
// tener tres, cae entero en la primera línea antes que reescribirlo.
func taglineLineas() [3]string {
	if w := strings.Fields(Tagline); len(w) == 3 {
		return [3]string{w[0], w[1], w[2]}
	}
	return [3]string{Tagline}
}

// dibujaWrapIzq es el wrap izquierdo (450×180): el tagline en tres líneas con la
// última en verde, más la marca compacta en su cuadro verde.
func dibujaWrapIzq(p *pdf, c PiezaImp, _ Machine, _ [][]bool) {
	fondoKiosko(p, c, 2.5)
	p.fantasma(c.X+300, c.Y+34, 112)
	p.badge(c.X+28, c.Y+20, 34)

	maxW := c.Ancho - 2*28
	colores := [3]string{ColorFG, ColorFG, ColorAccent}
	for i, s := range taglineLineas() {
		if s == "" {
			continue
		}
		p.text(c.X+28, c.Y+90+float64(i)*34, fitSize(fontDisplay, 34, maxW, s),
			fontDisplay, colores[i], s)
	}
}

// dibujaWrapDer es el wrap derecho (450×180): la réplica de instrucciones.svg
// (mockup de Daniel) reproporcionada al formato del wrap.
//
// A la izquierda el argumento de venta —tres líneas CON punto, la última en
// verde, y el dominio en mono debajo—; a la derecha los tres pasos numerados en
// círculos verdes, separados por un filete. La barra verde va de canto a canto
// en el borde izquierdo, no como filetes horizontales.
func dibujaWrapDer(p *pdf, c PiezaImp, m Machine, _ [][]bool) {
	p.rect(c.X, c.Y, c.Ancho, c.Alto, ColorBG)
	// La barra va de canto a canto: con márgenes se lee como un filete suelto en
	// vez de como el borde de la pieza (ADR-026 addendum).
	p.rect(c.X, c.Y, barraAncho, c.Alto, ColorAccent)
	// La fantasma va abajo a la derecha, DETRÁS de los pasos (por eso se pinta
	// antes) y entera dentro del arte: recortada por el borde se lee como un
	// error de montaje y no como marca de agua.
	p.fantasma(c.X+366, c.Y+98, 72)

	// --- izquierda: el argumento de venta ---
	x := c.X + 34
	maxW := divisorX - 34 - 16
	p.badge(x, c.Y+16, 30)
	colores := [3]string{ColorFG, ColorFG, ColorAccent}
	for i, s := range CopySinEfectivo {
		p.text(x, c.Y+84+float64(i)*30, fitSize(fontDisplay, 28, maxW, s), fontDisplay, colores[i], s)
	}
	// El dominio remata el bloque: quien lee el wrap ya sabe a dónde ir aunque no
	// tenga el QR delante.
	p.text(x, c.Y+166, fitSize(fontMonoBold, 9, maxW, m.host()), fontMonoBold, ColorMuted, m.host())

	// --- filete divisor ---
	p.rect(c.X+divisorX, c.Y+22, 0.6, 136, ColorLine2)

	// --- derecha: los tres pasos ---
	cx, tx := c.X+284, c.X+308
	tmax := c.Ancho - 308 - 18
	for i, pa := range PasosInstrucciones {
		cy := c.Y + 52 + float64(i)*44
		p.circle(cx, cy, 13, ColorAccent)
		p.textCentro(cx, cy+capAlto*14/2, 14, fontDisplay, ColorInk, strconv.Itoa(i+1))
		for j, linea := range pa {
			p.text(tx, cy-2+float64(j)*16, fitSize(fontDisplay, 13, tmax, linea), fontDisplay, ColorFG, linea)
		}
	}
}

// pasosImp son los tres pasos del panel vertical. Mismo guion que
// instrucciones.svg (es lo que lee quien nunca ha comprado en una GRABI), pero
// con el detalle partido para el ancho de esta pieza, que es estrecha.
var pasosImp = [3]struct {
	Titulo string
	Sub    []string
}{
	{"ESCANEA", []string{"el QR de la máquina"}},
	{"PAGA", []string{"con Bre-B", "desde tu banco"}},
	{"MUESTRA", []string{"el QR y agárralo"}},
}

// Ritmo vertical del panel de pasos. Se maqueta EN FLUJO (cada elemento empuja
// al siguiente) y no con posiciones fijas, porque el paso 2 lleva una línea de
// detalle más que los otros: con una retícula fija o quedaría descuadrado o
// habría que dejarle a todos el hueco del más alto.
const (
	pasoMargen      = 5.0
	pasoArranque    = 13.0
	pasoRadio       = 8.5
	pasoNumero      = 11.0
	pasoAireCirculo = 6.0
	pasoTitulo      = 11.5
	pasoAireTitulo  = 4.5
	pasoSub         = 5.4
	pasoLeadingSub  = 6.5
	pasoAireSep     = 5.5
	pasoSepAncho    = 50.0
	pasoSepGrosor   = 0.5
)

// dibujaPasos es el panel de instrucciones vertical (80×180), según la
// referencia que pasó Daniel (2026-08-13): todo **centrado** y con la jerarquía
// hecha de contraste, no de alineación — número en círculo verde, título en
// display grande y claro, detalle en mono pequeño y atenuado, y un filete corto
// separando cada paso del siguiente.
func dibujaPasos(p *pdf, c PiezaImp, _ Machine, _ [][]bool) {
	p.rect(c.X, c.Y, c.Ancho, c.Alto, ColorSurface)

	cx := c.X + c.Ancho/2
	maxW := c.Ancho - 2*pasoMargen
	y := c.Y + pasoArranque
	for i, pa := range pasosImp {
		// Número en círculo verde, con la tinta oscura encima (nunca clara).
		p.circle(cx, y+pasoRadio, pasoRadio, ColorAccent)
		p.textCentro(cx, y+pasoRadio+capAlto*pasoNumero/2, pasoNumero, fontDisplay, ColorInk, strconv.Itoa(i+1))
		y += 2*pasoRadio + pasoAireCirculo

		st := fitSize(fontDisplay, pasoTitulo, maxW, pa.Titulo)
		p.textCentro(cx, y+capAlto*st, st, fontDisplay, ColorFG, pa.Titulo)
		y += capAlto*st + pasoAireTitulo

		for _, linea := range pa.Sub {
			ss := fitSize(fontMonoBold, pasoSub, maxW, linea)
			p.textCentro(cx, y+capAltoMono*ss, ss, fontMonoBold, ColorFG, linea)
			y += pasoLeadingSub
		}

		// El último paso no lleva filete: cerraría el panel por abajo y se leería
		// como que falta un cuarto paso.
		if i < len(pasosImp)-1 {
			y += pasoAireSep
			p.rect(cx-pasoSepAncho/2, y, pasoSepAncho, pasoSepGrosor, ColorLine2)
			y += pasoAireSep
		}
	}
}

// dibujaCabecera es la cabecera de la máquina (280×70): SOLO el wordmark con el
// punto verde. Sin dominio y sin la bajada de Bre-B — eso ya lo dicen los wraps
// y el panel de pasos, y repetido en la cabecera satura el frente.
func dibujaCabecera(p *pdf, c PiezaImp, _ Machine, _ [][]bool) {
	fondoKiosko(p, c, 2.5)

	size := fitSize(fontDisplay, 60, c.Ancho-2*20, "GRABI.")
	wG := textWidth(fontDisplay, size, "GRABI")
	total := wG + textWidth(fontDisplay, size, ".")
	x := c.X + (c.Ancho-total)/2
	y := c.Y + c.Alto/2 + capAlto*size/2
	p.text(x, y, size, fontDisplay, ColorFG, "GRABI")
	p.text(x+wG, y, size, fontDisplay, ColorAccent, ".")
}

// dibujaPlaca es la plaquita identificadora (250×50): "GRABI {id}".
//
// Dos diferencias con placa.svg, a propósito: aquí VA CON FONDO oscuro (en el
// pliego todas las piezas comparten material, y sobre vinilo blanco sin imprimir
// el texto claro no se vería), y el id va sin el punto del logotipo — "GRABI"
// aquí es texto identificador junto al id, no el wordmark; el wordmark con punto
// es la cabecera.
func dibujaPlaca(p *pdf, c PiezaImp, m Machine, _ [][]bool) {
	fondoKiosko(p, c, 2)

	const marcaSize, aire = 18.0, 8.0
	sTxt := 22.0
	sID := fitSize(fontMonoBold, 16, c.Ancho*0.4, m.ID)
	wTxt := textWidth(fontDisplay, sTxt, "GRABI")
	wID := textWidth(fontMonoBold, sID, m.ID)

	x := c.X + (c.Ancho-(marcaSize+aire+wTxt+aire+wID))/2
	p.marca(x, c.Y+(c.Alto-marcaSize)/2, marcaSize, ColorFG, ColorAccent)
	y := c.Y + c.Alto/2 + capAlto*sTxt/2
	p.text(x+marcaSize+aire, y, sTxt, fontDisplay, ColorFG, "GRABI")
	p.text(x+marcaSize+aire+wTxt+aire, y, sID, fontMonoBold, ColorAccent, m.ID)
}

// dibujaQRPieza es el sticker del QR (100×100): el símbolo con la marca al
// centro y "ESCANEA AQUÍ" debajo. Es LA pieza que hace vender, y la única
// personalizada por máquina junto con la placa.
func dibujaQRPieza(p *pdf, c PiezaImp, _ Machine, mat [][]bool) {
	p.rect(c.X, c.Y, c.Ancho, c.Alto, ColorBG)

	// 74 mm de lado (zona de silencio incluida) ⇒ el símbolo sale muy por encima
	// del mínimo imprimible de 35 mm que exige el LEEME del kit.
	const lado = 74.0
	p.drawQR(mat, c.X+(c.Ancho-lado)/2, c.Y+9, lado)

	const rotulo = "ESCANEA AQUÍ"
	p.textCentro(c.X+c.Ancho/2, c.Y+94, fitSize(fontDisplay, 9, c.Ancho-16, rotulo),
		fontDisplay, ColorAccent, rotulo)
}

// Altura de caja alta de cada fuente, como fracción del cuerpo. Se usa para
// centrar y para maquetar en flujo: sin ella el bloque queda alto, porque la
// línea base no es el centro óptico ni el borde superior del texto.
const (
	capAlto     = 0.718 // Helvetica-Bold
	capAltoMono = 0.562 // Courier
)
