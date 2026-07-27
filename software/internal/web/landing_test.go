package web

// Pruebas de la landing pública (landing-v2). No tocan la base de datos: la
// home y el formulario de interesados no consultan el store, así que el
// servidor se construye con store nil.

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// newTestServer construye un Server sin base de datos (suficiente para las
// rutas públicas que no consultan el store).
func newTestServer(t *testing.T) *Server {
	t.Helper()
	s, err := New(nil, "admin", "secreto", nil, false, 0, t.TempDir(), false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func TestLandingEsLaHome(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Routes())
	defer srv.Close()

	// Sin seguir redirecciones: la raíz debe responder 200, no un 303 al panel.
	res, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}).Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET / = %d, se esperaba 200 (¿sigue redirigiendo a /admin?)", res.StatusCode)
	}
	body := readAll(t, res)
	for _, want := range []string{
		"Escanea, paga,", "agárralo.",
		"Cuatro pasos y listo",   // cómo funciona (teléfono sticky)
		"Ten tu propia GRABI",    // sección B2B (dueño del punto)
		"Comprar aquí es seguro", // por qué GRABI (quien compra)
		"Piloto activo en Cali",  // chip del hero + estado del piloto
		`href="#piloto"`,         // el chip del hero enlaza al estado del piloto
		"Quiero una máquina →",   // CTA primario del hero
		"Ver cómo funciona ↓",    // CTA secundario del hero
		`href="#negocio"`,        // enlace del nav a la sección B2B
		`action="/interesados"`,  // formulario de leads
		`name="space_type"`,      // select de tipo de espacio
	} {
		if !strings.Contains(body, want) {
			t.Errorf("la landing no contiene %q", want)
		}
	}
	// Las cuatro pantallas del recorrido deben estar en la página (sticky + móvil).
	for _, want := range []string{"Apunta al QR pegado en la máquina", "Pagar con Bre-B →",
		"Esperando tu pago…", "Tu QR está listo"} {
		if !strings.Contains(body, want) {
			t.Errorf("falta la pantalla con %q", want)
		}
	}
	// Sin GRABI_WHATSAPP configurado no se publica ningún contacto inventado.
	if strings.Contains(body, "wa.me/") {
		t.Error("se muestra el botón de WhatsApp sin número configurado")
	}
}

func TestNotFoundLlevaAInicio(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Routes())
	defer srv.Close()

	res, err := http.Get(srv.URL + "/ruta-que-no-existe")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, se esperaba 404", res.StatusCode)
	}
	body := readAll(t, res)
	if !strings.Contains(body, `href="/"`) || !strings.Contains(body, "Volver a inicio") {
		t.Error("el 404 no tiene el botón 'Volver a inicio' apuntando a /")
	}
}

func TestInteresadoValidaYAcepta(t *testing.T) {
	s := newTestServer(t)
	srv := httptest.NewServer(s.Routes())
	defer srv.Close()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	post := func(form url.Values) *http.Response {
		t.Helper()
		res, err := client.PostForm(srv.URL+"/interesados", form)
		if err != nil {
			t.Fatalf("POST /interesados: %v", err)
		}
		return res
	}

	ok := url.Values{"name": {"Ana"}, "space_type": {"conjunto"}, "city": {"Cali"}, "phone": {"300 123 4567"}}
	with := func(k, v string) url.Values {
		out := url.Values{}
		for key, vals := range ok {
			out[key] = vals
		}
		out.Set(k, v)
		return out
	}

	// Tipo de espacio fuera de la lista → 400 (el select no es una promesa).
	res := post(with("space_type", "casa-del-arbol"))
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("tipo inválido: status = %d, se esperaba 400", res.StatusCode)
	}
	if body := readAll(t, res); !strings.Contains(body, "Elige el tipo de espacio") || !strings.Contains(body, "300 123 4567") {
		t.Error("tipo inválido: falta el aviso o se perdió lo escrito")
	}
	res.Body.Close()

	// WhatsApp demasiado corto → 400.
	res = post(with("phone", "300"))
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("WhatsApp corto: status = %d, se esperaba 400", res.StatusCode)
	}
	res.Body.Close()

	// Nombre y ciudad son obligatorios.
	for _, campo := range []string{"name", "city"} {
		res = post(with(campo, ""))
		if res.StatusCode != http.StatusBadRequest {
			t.Errorf("%s vacío: status = %d, se esperaba 400", campo, res.StatusCode)
		}
		res.Body.Close()
	}

	// Válido → redirección al estado de éxito.
	res = post(ok)
	if res.StatusCode != http.StatusSeeOther {
		t.Errorf("lead válido: status = %d, se esperaba 303", res.StatusCode)
	}
	if loc := res.Header.Get("Location"); !strings.HasPrefix(loc, "/?gracias=1") {
		t.Errorf("lead válido: Location = %q", loc)
	}
	res.Body.Close()

	// El honeypot lleno se acepta en silencio (no se avisa al bot).
	res = post(with("website", "http://spam"))
	if res.StatusCode != http.StatusSeeOther {
		t.Errorf("honeypot: status = %d, se esperaba 303", res.StatusCode)
	}
	res.Body.Close()
}

func TestGraciasMuestraElEstadoDeExito(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Routes())
	defer srv.Close()
	res, err := http.Get(srv.URL + "/?gracias=1")
	if err != nil {
		t.Fatalf("GET /?gracias=1: %v", err)
	}
	defer res.Body.Close()
	body := readAll(t, res)
	if !strings.Contains(body, "¡Gracias! Te contactamos pronto.") {
		t.Error("no se muestra el estado de éxito con ?gracias=1")
	}
	if strings.Contains(body, `name="space_type"`) {
		t.Error("el formulario sigue visible tras enviar")
	}
}

func TestRateLimitDeInteresados(t *testing.T) {
	s := newTestServer(t)
	now := time.Now()
	for i := 0; i < leadMaxPerWindow; i++ {
		if !s.allowLead("10.0.0.1", now) {
			t.Fatalf("el envío %d debería permitirse", i+1)
		}
	}
	if s.allowLead("10.0.0.1", now) {
		t.Error("se permitió un envío por encima del tope de la ventana")
	}
	if !s.allowLead("10.0.0.2", now) {
		t.Error("el tope debe ser por IP, no global")
	}
	// Pasada la ventana, la IP vuelve a poder enviar (y el mapa se limpia).
	if !s.allowLead("10.0.0.1", now.Add(leadWindow+time.Minute)) {
		t.Error("tras la ventana el envío debería permitirse")
	}
}

func readAll(t *testing.T, res *http.Response) string {
	t.Helper()
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := res.Body.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			break
		}
	}
	return sb.String()
}
