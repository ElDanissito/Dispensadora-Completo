package web

// Pruebas de la sección "Interesados" del panel (landing-v1 §4): la ruta va
// protegida (los leads son PII) y lista lo que hay en la tabla `leads`.

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"dispensadoras/software/internal/store"
)

// La ruta exige sesión: sin cookie válida no se filtra ni un dato personal.
func TestLeadsDelPanelExigenSesion(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Routes())
	defer srv.Close()

	res, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}).Get(srv.URL + "/admin/leads")
	if err != nil {
		t.Fatalf("GET /admin/leads: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, se esperaba 303 al login", res.StatusCode)
	}
	if loc := res.Header.Get("Location"); loc != "/admin/login" {
		t.Errorf("Location = %q, se esperaba /admin/login", loc)
	}
}

func TestLeadsDelPanelListaLosInteresados(t *testing.T) {
	s, st := newTestServerConBase(t)
	ctx := context.Background()
	// Dos leads: el segundo es más reciente y debe salir primero.
	if _, err := st.CreateLead(ctx, store.Lead{
		Name: "Ana Ruiz", SpaceType: "conjunto", City: "Cali", WhatsApp: "3001234567",
		CreatedAt: 1785000000,
	}); err != nil {
		t.Fatalf("CreateLead: %v", err)
	}
	if _, err := st.CreateLead(ctx, store.Lead{
		Name: "Beto Gómez", SpaceType: "oficina", City: "Palmira", WhatsApp: "3109876543",
		Source: "referido", CreatedAt: 1785100000,
	}); err != nil {
		t.Fatalf("CreateLead: %v", err)
	}

	srv := httptest.NewServer(s.Routes())
	defer srv.Close()
	body := getConSesion(t, srv.URL, "/admin/leads")

	// Los seis campos de la lista (landing-v1 §4), con el tipo de espacio legible
	// y el origen como pastilla.
	for _, want := range []string{
		"Interesados en tener una GRABI",
		"Ana Ruiz", "Conjunto residencial", "Cali", "3001234567", "LANDING",
		"Beto Gómez", "Oficina / coworking", "Palmira", "3109876543", "REFERIDO",
		"Fecha", "Tipo de espacio", "WhatsApp", "Origen",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("la lista de interesados no contiene %q", want)
		}
	}
	// Más recientes primero: Beto (más nuevo) antes que Ana.
	if i, j := strings.Index(body, "Beto Gómez"), strings.Index(body, "Ana Ruiz"); i > j {
		t.Error("los leads no salen del más reciente al más antiguo")
	}
	// La navegación del panel (escritorio y móvil) lleva a la sección.
	if strings.Count(body, `href="/admin/leads"`) < 2 {
		t.Error("falta la entrada de navegación en el sidebar y/o en la barra inferior")
	}
}

// Sin leads, la sección muestra el estado vacío en vez de una tabla en blanco.
func TestLeadsDelPanelSinInteresados(t *testing.T) {
	s, _ := newTestServerConBase(t)
	srv := httptest.NewServer(s.Routes())
	defer srv.Close()

	body := getConSesion(t, srv.URL, "/admin/leads")
	if !strings.Contains(body, "Todavía no hay interesados") {
		t.Error("no se muestra el estado vacío")
	}
	if strings.Contains(body, "Tipo de espacio") {
		t.Error("se pinta la cabecera de la tabla sin filas")
	}
}

// getConSesion entra al panel con las credenciales de prueba y devuelve el cuerpo
// de `path` ya autenticado.
func getConSesion(t *testing.T, baseURL, path string) string {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	client := &http.Client{Jar: jar}
	login, err := client.PostForm(baseURL+"/admin/login",
		url.Values{"user": {"admin"}, "pass": {"secreto"}})
	if err != nil {
		t.Fatalf("POST /admin/login: %v", err)
	}
	login.Body.Close()
	res, err := client.Get(baseURL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d, se esperaba 200", path, res.StatusCode)
	}
	return readAll(t, res)
}
