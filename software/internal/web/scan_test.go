package web

// Pruebas del escáner de QR (GET /scan, ADR-025). No tocan la base de datos: la
// página no consulta máquinas (de eso se encarga /m/{id}).

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// noRedirect es un cliente que NO sigue redirecciones, para poder inspeccionar
// el 303 y su Location.
func noRedirect() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

// drenar consume y cierra el cuerpo. Cerrarlo sin leerlo corta la conexión a
// media respuesta y el servidor registra un error de escritura que no viene al
// caso (aquí lo que se comprueba es el status y el Location).
func drenar(res *http.Response) {
	_, _ = io.Copy(io.Discard, res.Body)
	res.Body.Close()
}

func TestScanSeSirve(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Routes())
	defer srv.Close()

	res, body := get(t, srv.URL+"/scan")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /scan: status %d, se esperaba 200", res.StatusCode)
	}
	html := string(body)
	for _, want := range []string{
		"Apunta al QR de la máquina", // copy de la mira
		`<video id="cam"`,            // stream en vivo
		`class="scanmira"`,           // esquinas + punto verde
		"Escribir el ID manualmente", // fallback manual
		"Permitir cámara",            // fallback sin permiso
		`<label for="mid">`,          // etiqueta visible (accesibilidad)
		`/static/vendor/jsQR.js`,     // jsQR autohospedado
		`aria-live="polite"`,         // mensajes anunciados
	} {
		if !strings.Contains(html, want) {
			t.Errorf("GET /scan: falta %q en el HTML", want)
		}
	}
}

// El escáner decide a dónde navega el cliente: un <script> de un tercero ahí
// sería el peor sitio posible para una dependencia externa. jsQR va embebido.
func TestScanNoCargaScriptsExternos(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Routes())
	defer srv.Close()

	_, body := get(t, srv.URL+"/scan")
	for _, prohibido := range []string{
		"src=\"http://", "src=\"//", "cdn.jsdelivr.net", "unpkg.com", "cdnjs",
	} {
		if strings.Contains(string(body), prohibido) {
			t.Errorf("GET /scan: carga un recurso externo (%q); jsQR debe ir autohospedado", prohibido)
		}
	}
	// El único src="https:// admitido es el de fuentes, que va en <link>, no en <script>.
	if strings.Contains(string(body), `<script src="https://`) {
		t.Error("GET /scan: hay un <script> apuntando a un dominio externo")
	}
}

// Los bloques que se ocultan con [hidden] llevan display de clase, y un display
// de clase gana al del navegador: sin la regla explícita el aviso de "sin
// cámara" se ve siempre (pasó de verdad al revisar el render).
func TestScanOcultaElFallbackAlInicio(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Routes())
	defer srv.Close()

	_, body := get(t, srv.URL+"/scan")
	html := string(body)
	if !strings.Contains(html, `<div class="scanfallback" id="fallback" hidden>`) {
		t.Error("el aviso de cámara debería empezar oculto")
	}
	if !strings.Contains(html, ".scanfallback[hidden]{display:none;}") {
		t.Error("falta la regla .scanfallback[hidden]; el aviso se vería aunque tenga el atributo")
	}
}

func TestJsQRSeSirveAutohospedado(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Routes())
	defer srv.Close()

	res, body := get(t, srv.URL+"/static/vendor/jsQR.js")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET jsQR.js: status %d, se esperaba 200", res.StatusCode)
	}
	if len(body) < 100_000 || !strings.Contains(string(body), "jsQR") {
		t.Errorf("jsQR.js no parece la librería (%d bytes)", len(body))
	}
	// Apache-2.0: la licencia viaja con la copia.
	if res, _ := get(t, srv.URL+"/static/vendor/jsQR.LICENSE.txt"); res.StatusCode != http.StatusOK {
		t.Errorf("falta la licencia de jsQR: status %d", res.StatusCode)
	}
}

// --- Fallback manual (?m=…): la misma validación que hace el JS, en el servidor,
// para que funcione sin JavaScript y para que nadie pueda colar un destino ajeno.

func TestScanManualRedirigeConIDValido(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Routes())
	defer srv.Close()
	cli := noRedirect()

	casos := []struct{ escrito, quiere string }{
		{"M001", "/m/M001"},
		{"m001", "/m/M001"},   // el catálogo usa mayúsculas
		{" M001 ", "/m/M001"}, // el usuario pega con espacios
		{"m 001", "/m/M001"},  // …o los teclea
		{"M0012345", "/m/M0012345"},
	}
	for _, c := range casos {
		res, err := cli.Get(srv.URL + "/scan?m=" + urlQuery(c.escrito))
		if err != nil {
			t.Fatalf("GET /scan?m=%q: %v", c.escrito, err)
		}
		drenar(res)
		if res.StatusCode != http.StatusSeeOther {
			t.Errorf("m=%q: status %d, se esperaba 303", c.escrito, res.StatusCode)
			continue
		}
		if got := res.Header.Get("Location"); got != c.quiere {
			t.Errorf("m=%q: Location %q, se esperaba %q", c.escrito, got, c.quiere)
		}
	}
}

// Ningún valor del formulario puede sacar al cliente de este sitio ni llevarlo a
// una ruta que no sea la tienda de una máquina.
func TestScanManualRechazaIDInvalido(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Routes())
	defer srv.Close()
	cli := noRedirect()

	for _, malo := range []string{
		"M12",   // menos de 3 dígitos
		"X001",  // otra letra
		"M001A", // sobra texto
		"MM001",
		"001",
		"https://evil.example/m/M001", // URL completa pegada en el campo
		"//evil.example",              // redirect abierto por protocolo relativo
		"/admin",                      // otra ruta del sitio
		"M001/../admin",
		"M001?x=1",
		"<script>alert(1)</script>",
	} {
		res, err := cli.Get(srv.URL + "/scan?m=" + urlQuery(malo))
		if err != nil {
			t.Fatalf("GET /scan?m=%q: %v", malo, err)
		}
		drenar(res)
		if res.StatusCode != http.StatusOK {
			t.Errorf("m=%q: status %d, se esperaba 200 (re-pintar el formulario, NO redirigir)", malo, res.StatusCode)
		}
		if loc := res.Header.Get("Location"); loc != "" {
			t.Errorf("m=%q: redirigió a %q; un ID inválido nunca debe navegar", malo, loc)
		}
	}
}

func TestScanManualMuestraElError(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Routes())
	defer srv.Close()

	res, body := get(t, srv.URL+"/scan?m=M12")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d, se esperaba 200", res.StatusCode)
	}
	html := string(body)
	if !strings.Contains(html, "Ese ID no es válido") {
		t.Error("no se muestra el mensaje de error del ID")
	}
	if !strings.Contains(html, `value="M12"`) {
		t.Error("no se conserva lo que el usuario escribió")
	}
	if !strings.Contains(html, `aria-invalid="true"`) {
		t.Error("el campo con error no está marcado como inválido (accesibilidad)")
	}
	if !strings.Contains(html, "<details class=\"scanmanual\" id=\"manual\" open>") {
		t.Error("el bloque manual debería quedar abierto al volver con error")
	}
}

// El HTML no puede escapar del contexto por lo que escriba el usuario.
func TestScanManualEscapaLoEscrito(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Routes())
	defer srv.Close()

	_, body := get(t, srv.URL+"/scan?m="+urlQuery(`"><script>alert(1)</script>`))
	if strings.Contains(string(body), "<script>alert(1)</script>") {
		t.Error("lo escrito por el usuario se pinta sin escapar")
	}
}

// La regex viaja al JS: si dejara de estar en el HTML, el escáner aceptaría
// cualquier cosa o nada.
func TestScanInyectaLaRegexCompartida(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Routes())
	defer srv.Close()

	_, body := get(t, srv.URL+"/scan")
	// html/template escapa la barra invertida al meterla en una cadena JS.
	if !strings.Contains(string(body), `new RegExp("^M\\d{3,}$")`) {
		t.Error("el JS no recibió machineIDPattern; el escáner y el servidor validarían distinto")
	}
}

// machineIDRe es la fuente de verdad compartida (Go + JS). Tabla explícita para
// que un cambio de forma del id sea una decisión, no un descuido.
func TestMachineIDRe(t *testing.T) {
	validos := []string{"M001", "M999", "M0001", "M123456"}
	invalidos := []string{"", "M", "M1", "M12", "m001", "X001", "M001 ", " M001", "M00A", "1M001", "M001/x", "M001\nM002"}

	for _, v := range validos {
		if !machineIDRe.MatchString(v) {
			t.Errorf("%q debería ser un machine_id válido", v)
		}
	}
	for _, v := range invalidos {
		if machineIDRe.MatchString(v) {
			t.Errorf("%q NO debería ser un machine_id válido", v)
		}
	}
}

// --- Integración con las páginas de error: el reintento lleva al escáner.

func TestPaginasDeErrorLlevanAEscanear(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Routes())
	defer srv.Close()

	// Caso reportado: el cliente teclea grabi.napi.lat/M001 (sin /m/) → 404.
	res, body := get(t, srv.URL+"/M001")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /M001: status %d, se esperaba 404", res.StatusCode)
	}
	html := string(body)
	if !strings.Contains(html, `href="/scan"`) {
		t.Error("el 404 no ofrece volver a escanear")
	}
	if !strings.Contains(html, "Volver a escanear") {
		t.Error("el 404 no tiene el CTA \"Volver a escanear\"")
	}
	if !strings.Contains(html, `class="btn-secondary" href="/"`) {
		t.Error("el 404 perdió el enlace secundario al inicio")
	}
}

// La otra cara del mismo caso: el id no existe en el catálogo. Se renderiza
// directo (no toca la DB) porque el handler que la usa sí la necesitaría.
func TestMaquinaNoEncontradaLlevaAEscanear(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer(t).machineNotFound(rec, "M999")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, se esperaba 404", rec.Code)
	}
	html := rec.Body.String()
	if !strings.Contains(html, `class="btn-primary" href="/scan"`) {
		t.Error("el CTA principal debería ser volver a escanear (/scan)")
	}
	if strings.Contains(html, "history.back()") {
		t.Error("sigue usando history.back(), que no vuelve a ningún escáner")
	}
	if !strings.Contains(html, `class="btn-secondary" href="/"`) {
		t.Error("falta el enlace secundario al inicio")
	}
}

// urlQuery codifica un valor para el query string.
func urlQuery(s string) string { return url.QueryEscape(s) }
