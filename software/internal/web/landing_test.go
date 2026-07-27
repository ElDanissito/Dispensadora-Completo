package web

// Pruebas de la landing pública (landing-v2). La home y las validaciones del
// formulario no consultan el store, así que se prueban con store nil; guardar el
// lead sí toca la base, y esa prueba requiere el Postgres de pruebas.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"dispensadoras/software/internal/store"
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

// newTestServerConBase construye un Server con el Postgres de pruebas
// (TEST_DATABASE_URL) ya limpio. Se omite el test si no hay base disponible.
func newTestServerConBase(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL no definido; se omite (requiere Postgres de pruebas)")
	}
	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.ResetForTest(context.Background()); err != nil {
		t.Fatalf("reset: %v", err)
	}
	s, err := New(st, "admin", "secreto", nil, false, 0, t.TempDir(), false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s, st
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

// leadOK son los cuatro campos válidos del formulario B2B.
var leadOK = url.Values{"name": {"Ana"}, "space_type": {"conjunto"}, "city": {"Cali"}, "phone": {"300 123 4567"}}

// leadCon copia leadOK cambiando un campo (para probar cada validación).
func leadCon(k, v string) url.Values {
	out := url.Values{}
	for key, vals := range leadOK {
		out[key] = vals
	}
	out.Set(k, v)
	return out
}

// postLead envía el formulario sin seguir la redirección de éxito.
func postLead(t *testing.T, baseURL string, form url.Values) *http.Response {
	t.Helper()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	res, err := client.PostForm(baseURL+"/interesados", form)
	if err != nil {
		t.Fatalf("POST /interesados: %v", err)
	}
	return res
}

// Las validaciones y el honeypot rechazan (o absorben) el envío ANTES de tocar la
// base, así que este test corre sin Postgres.
func TestInteresadoValidaAntesDeGuardar(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Routes())
	defer srv.Close()
	post := func(form url.Values) *http.Response { return postLead(t, srv.URL, form) }
	with := leadCon

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

	// El honeypot lleno se acepta en silencio (no se avisa al bot) y no se guarda
	// nada: por eso responde 303 aunque no haya base detrás.
	res = post(with("website", "http://spam"))
	if res.StatusCode != http.StatusSeeOther {
		t.Errorf("honeypot: status = %d, se esperaba 303", res.StatusCode)
	}
	res.Body.Close()
}

// Un lead válido se persiste en la tabla `leads` (landing-v1 §5) y el visitante
// termina en el estado de éxito.
func TestInteresadoSeGuardaEnLaBase(t *testing.T) {
	s, st := newTestServerConBase(t)
	srv := httptest.NewServer(s.Routes())
	defer srv.Close()

	res := postLead(t, srv.URL, leadOK)
	defer res.Body.Close()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("lead válido: status = %d, se esperaba 303", res.StatusCode)
	}
	if loc := res.Header.Get("Location"); !strings.HasPrefix(loc, "/?gracias=1") {
		t.Errorf("lead válido: Location = %q", loc)
	}

	leads, err := st.ListLeads(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListLeads: %v", err)
	}
	if len(leads) != 1 {
		t.Fatalf("esperaba 1 lead guardado, obtuve %d", len(leads))
	}
	got := leads[0]
	if got.Name != "Ana" || got.SpaceType != "conjunto" || got.City != "Cali" ||
		got.WhatsApp != "300 123 4567" || got.Source != "landing" || got.CreatedAt == 0 {
		t.Fatalf("lead guardado inesperado: %+v", got)
	}

	// Un tipo de espacio fuera de la lista no llega a la base.
	bad := postLead(t, srv.URL, leadCon("space_type", "casa-del-arbol"))
	bad.Body.Close()
	if bad.StatusCode != http.StatusBadRequest {
		t.Errorf("tipo inválido: status = %d, se esperaba 400", bad.StatusCode)
	}
	if leads, err = st.ListLeads(context.Background(), 10); err != nil {
		t.Fatalf("ListLeads: %v", err)
	} else if len(leads) != 1 {
		t.Errorf("un lead inválido se guardó: %d filas", len(leads))
	}

	// El honeypot tampoco deja rastro en la base.
	hp := postLead(t, srv.URL, leadCon("website", "http://spam"))
	hp.Body.Close()
	if leads, err = st.ListLeads(context.Background(), 10); err != nil {
		t.Fatalf("ListLeads: %v", err)
	} else if len(leads) != 1 {
		t.Errorf("el honeypot guardó un lead: %d filas", len(leads))
	}
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
