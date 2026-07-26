package web

// Pruebas de la landing pública (landing-v1). No tocan la base de datos: la
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
	for _, want := range []string{"Escanea, paga,", "agárralo.", "Tres pasos y listo", "Hecho para confiar", `action="/interesados"`} {
		if !strings.Contains(body, want) {
			t.Errorf("la landing no contiene %q", want)
		}
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

	// Correo inválido → 400 y la landing con el aviso, conservando el celular.
	res := post(url.Values{"email": {"no-es-correo"}, "phone": {"3001234567"}})
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("correo inválido: status = %d, se esperaba 400", res.StatusCode)
	}
	if body := readAll(t, res); !strings.Contains(body, "Revisa el correo") || !strings.Contains(body, "3001234567") {
		t.Error("correo inválido: falta el aviso o se perdió lo escrito")
	}
	res.Body.Close()

	// Celular demasiado corto → 400.
	res = post(url.Values{"email": {"ana@ejemplo.com"}, "phone": {"300"}})
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("celular corto: status = %d, se esperaba 400", res.StatusCode)
	}
	res.Body.Close()

	// Válido → redirección al estado de éxito.
	res = post(url.Values{"email": {"ana@ejemplo.com"}, "phone": {"300 123 4567"}, "name": {"Ana"}})
	if res.StatusCode != http.StatusSeeOther {
		t.Errorf("lead válido: status = %d, se esperaba 303", res.StatusCode)
	}
	if loc := res.Header.Get("Location"); !strings.HasPrefix(loc, "/?gracias=1") {
		t.Errorf("lead válido: Location = %q", loc)
	}
	res.Body.Close()

	// El honeypot lleno se acepta en silencio (no se avisa al bot).
	res = post(url.Values{"email": {"bot@spam.com"}, "phone": {"3001234567"}, "website": {"http://spam"}})
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
	if strings.Contains(body, `name="email"`) {
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
