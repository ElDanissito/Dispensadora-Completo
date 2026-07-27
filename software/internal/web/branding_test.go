package web

// Pruebas del cableado de la marca (identidad-visual-v1 §9): los assets se
// sirven desde el binario, el <head> declara favicon/manifiesto y las metas
// sociales llevan URL absolutas. No tocan la base de datos.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// get pide una ruta al servidor de prueba y devuelve respuesta y cuerpo.
func get(t *testing.T, url string) (*http.Response, []byte) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("leyendo %s: %v", url, err)
	}
	return res, body
}

func TestAssetsDeMarcaSeSirven(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Routes())
	defer srv.Close()

	// /favicon.ico va aparte: el navegador lo pide a la raíz aunque exista el
	// <link rel="icon">, y sin ruta propia caería en el 404 en HTML.
	for _, path := range []string{
		"/static/brand/favicon.ico",
		"/static/brand/favicon-16.png",
		"/static/brand/favicon-32.png",
		"/static/brand/apple-touch-icon.png",
		"/static/brand/icon-192.png",
		"/static/brand/icon-512.png",
		"/static/brand/social-1200x630.png",
		"/static/brand/wordmark-oscuro.svg",
		"/favicon.ico",
	} {
		res, body := get(t, srv.URL+path)
		if res.StatusCode != http.StatusOK {
			t.Errorf("%s: status %d, se esperaba 200", path, res.StatusCode)
			continue
		}
		if len(body) == 0 {
			t.Errorf("%s: cuerpo vacío", path)
		}
	}
}

func TestManifestWebmanifest(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Routes())
	defer srv.Close()

	res, body := get(t, srv.URL+"/static/manifest.webmanifest")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d, se esperaba 200", res.StatusCode)
	}
	// Con el Content-Type equivocado el navegador descarta el manifiesto.
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/manifest+json") {
		t.Errorf("Content-Type %q, se esperaba application/manifest+json", ct)
	}

	var m struct {
		Name       string `json:"name"`
		Theme      string `json:"theme_color"`
		Background string `json:"background_color"`
		Icons      []struct {
			Src   string `json:"src"`
			Sizes string `json:"sizes"`
		} `json:"icons"`
	}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("manifiesto no es JSON válido: %v", err)
	}
	if m.Name != "GRABI" {
		t.Errorf("name = %q, se esperaba GRABI", m.Name)
	}
	if m.Theme != "#0A0E0C" || m.Background != "#0A0E0C" {
		t.Errorf("colores = %q/%q, se esperaba #0A0E0C en ambos", m.Theme, m.Background)
	}
	sizes := map[string]string{}
	for _, ic := range m.Icons {
		sizes[ic.Sizes] = ic.Src
	}
	for _, want := range []string{"192x192", "512x512"} {
		src, ok := sizes[want]
		if !ok {
			t.Errorf("falta el ícono %s en el manifiesto", want)
			continue
		}
		if res, _ := get(t, srv.URL+src); res.StatusCode != http.StatusOK {
			t.Errorf("ícono %s (%s): status %d", want, src, res.StatusCode)
		}
	}
}

func TestHeadDeMarcaEnLaHome(t *testing.T) {
	// Base canónica determinista: el entorno podría traer otra.
	t.Setenv("GRABI_SITE_URL", "")
	srv := httptest.NewServer(newTestServer(t).Routes())
	defer srv.Close()

	res, body := get(t, srv.URL+"/")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d, se esperaba 200", res.StatusCode)
	}
	html := string(body)

	img := siteBaseDefault + socialImagePath
	for _, want := range []string{
		`<link rel="icon" href="/static/brand/favicon.ico"`,
		`href="/static/brand/favicon-32.png"`,
		`<link rel="apple-touch-icon" href="/static/brand/apple-touch-icon.png">`,
		`<link rel="manifest" href="/static/manifest.webmanifest">`,
		`<meta property="og:type" content="website">`,
		`<meta property="og:title" content="GRABI`,
		`<meta property="og:url" content="` + siteBaseDefault + `/">`,
		`<meta property="og:image" content="` + img + `">`,
		`<meta name="twitter:card" content="summary_large_image">`,
		`<meta name="twitter:image" content="` + img + `">`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("el <head> de la home no contiene %s", want)
		}
	}
	// og:description no puede quedar vacío: es lo que se lee al compartir.
	if !strings.Contains(html, `<meta property="og:description" content="`+metaDescDefault+`">`) {
		t.Error("falta og:description con la descripción por defecto")
	}
}
