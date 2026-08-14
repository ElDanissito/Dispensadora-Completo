package web

// Pruebas del kit físico en el panel: los archivos del QR y las calcomanías solo
// existen para el admin autenticado, y el QR que se sirve tiene que poder
// escanearse de verdad (si no, la máquina queda inservible con un sticker bonito).

import (
	"archive/zip"
	"bytes"
	"context"
	"image/png"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/makiuchi-d/gozxing"
	zxqr "github.com/makiuchi-d/gozxing/qrcode"

	"dispensadoras/software/internal/kit"
)

// Sin sesión no sale ni un archivo. Y responde 401, no el 303 al login: quien
// pide un .svg/.png/.zip recibiría HTML disfrazado de imagen.
func TestKitFisicoExigeSesion(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t).Routes())
	defer srv.Close()

	sinRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	for _, path := range []string{
		"/admin/machines/M001/qr.svg",
		"/admin/machines/M001/qr.png",
		"/admin/machines/M001/kit.zip",
		"/admin/machines/M001/kit-imposicion.pdf",
	} {
		res, err := sinRedirect.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized {
			t.Errorf("GET %s sin sesión = %d, se esperaba 401", path, res.StatusCode)
		}
	}
}

// clienteConSesion entra al panel y devuelve un cliente con la cookie puesta.
func clienteConSesion(t *testing.T, baseURL string) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	c := &http.Client{Jar: jar}
	res, err := c.PostForm(baseURL+"/admin/login", url.Values{"user": {"admin"}, "pass": {"secreto"}})
	if err != nil {
		t.Fatalf("POST /admin/login: %v", err)
	}
	res.Body.Close()
	return c
}

// pide hace un GET autenticado y devuelve la respuesta y el cuerpo.
func pide(t *testing.T, c *http.Client, u string) (*http.Response, []byte) {
	t.Helper()
	res, err := c.Get(u)
	if err != nil {
		t.Fatalf("GET %s: %v", u, err)
	}
	defer res.Body.Close()
	var b bytes.Buffer
	if _, err := b.ReadFrom(res.Body); err != nil {
		t.Fatalf("leyendo %s: %v", u, err)
	}
	return res, b.Bytes()
}

// decodificaQR devuelve el texto y el nivel de corrección del QR de un PNG.
func decodificaQR(t *testing.T, pngBytes []byte) (texto, ecc string) {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("PNG inválido: %v", err)
	}
	bmp, err := gozxing.NewBinaryBitmap(gozxing.NewHybridBinarizer(gozxing.NewLuminanceSourceFromImage(img)))
	if err != nil {
		t.Fatalf("bitmap: %v", err)
	}
	res, err := zxqr.NewQRCodeReader().Decode(bmp, nil)
	if err != nil {
		t.Fatalf("el QR servido no se pudo decodificar: %v", err)
	}
	ecc, _ = res.GetResultMetadata()[gozxing.ResultMetadataType_ERROR_CORRECTION_LEVEL].(string)
	return res.GetText(), ecc
}

// con sesión: el QR se sirve, se escanea y apunta a la página de la máquina.
func TestKitFisicoSirveElQR(t *testing.T) {
	s, st := newTestServerConBase(t)
	if err := st.CreateMachine(context.Background(), "M001", "Palmira", "k1", 4); err != nil {
		t.Fatalf("CreateMachine: %v", err)
	}
	srv := httptest.NewServer(s.Routes())
	defer srv.Close()
	c := clienteConSesion(t, srv.URL)

	res, body := pide(t, c, srv.URL+"/admin/machines/M001/qr.png?size=512")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("qr.png = %d, se esperaba 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "image/png" {
		t.Errorf("qr.png Content-Type = %q", ct)
	}
	texto, ecc := decodificaQR(t, body)
	want := s.site + "/m/M001"
	if texto != want {
		t.Errorf("el QR servido decodifica %q, se esperaba %q", texto, want)
	}
	if ecc != "H" {
		t.Errorf("el QR servido trae ECC %q, se esperaba H", ecc)
	}

	res, body = pide(t, c, srv.URL+"/admin/machines/M001/qr.svg")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("qr.svg = %d, se esperaba 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "image/svg+xml") {
		t.Errorf("qr.svg Content-Type = %q", ct)
	}
	if !strings.Contains(string(body), "<svg") || !strings.Contains(string(body), `width="512"`) {
		t.Error("qr.svg no parece un SVG con el tamaño por defecto")
	}

	// ?size= manda, pero acotado: nada de PNG gigantes ni QR ilegibles.
	for _, caso := range []struct {
		query string
		want  string
	}{
		{"?size=1024", `width="1024"`},
		{"?size=1", `width="128"`},      // por debajo del mínimo
		{"?size=99999", `width="2048"`}, // por encima del máximo
		{"?size=abc", `width="512"`},    // basura ⇒ por defecto
	} {
		_, b := pide(t, c, srv.URL+"/admin/machines/M001/qr.svg"+caso.query)
		if !strings.Contains(string(b), caso.want) {
			t.Errorf("qr.svg%s no contiene %s", caso.query, caso.want)
		}
	}

	// Una máquina que no existe no genera papelería fantasma.
	res, _ = pide(t, c, srv.URL+"/admin/machines/M999/qr.png")
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("qr.png de una máquina inexistente = %d, se esperaba 404", res.StatusCode)
	}
}

func TestKitFisicoDescargaElZip(t *testing.T) {
	s, st := newTestServerConBase(t)
	if err := st.CreateMachine(context.Background(), "M001", "Palmira", "k1", 4); err != nil {
		t.Fatalf("CreateMachine: %v", err)
	}
	srv := httptest.NewServer(s.Routes())
	defer srv.Close()

	res, body := pide(t, clienteConSesion(t, srv.URL), srv.URL+"/admin/machines/M001/kit.zip")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("kit.zip = %d, se esperaba 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/zip" {
		t.Errorf("kit.zip Content-Type = %q", ct)
	}
	// Sin Content-Disposition el navegador lo abriría en vez de descargarlo.
	if cd := res.Header.Get("Content-Disposition"); !strings.Contains(cd, `filename="grabi-M001-kit.zip"`) {
		t.Errorf("kit.zip Content-Disposition = %q", cd)
	}

	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("el kit.zip no es un ZIP válido: %v", err)
	}
	nombres := map[string]bool{}
	for _, f := range zr.File {
		nombres[f.Name] = true
	}
	for _, want := range kit.ZipFiles {
		if !nombres[want] {
			t.Errorf("el kit.zip no trae %q (tiene %v)", want, nombres)
		}
	}
}

// La hoja de imposición: un PDF de verdad, con el pliego a escala 1:1 y las seis
// piezas. El contenido lo verifica en detalle internal/kit; aquí se comprueba lo
// que es del endpoint (tipo, descarga, 404) más que el pliego llegue completo.
func TestKitFisicoDescargaLaImposicion(t *testing.T) {
	s, st := newTestServerConBase(t)
	if err := st.CreateMachine(context.Background(), "M001", "Palmira", "k1", 4); err != nil {
		t.Fatalf("CreateMachine: %v", err)
	}
	srv := httptest.NewServer(s.Routes())
	defer srv.Close()
	c := clienteConSesion(t, srv.URL)

	res, body := pide(t, c, srv.URL+"/admin/machines/M001/kit-imposicion.pdf")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("kit-imposicion.pdf = %d, se esperaba 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("kit-imposicion.pdf Content-Type = %q", ct)
	}
	// Sin Content-Disposition el navegador lo abre en una pestaña; lo que hay que
	// hacer con este archivo es reenviarlo a la imprenta.
	if cd := res.Header.Get("Content-Disposition"); !strings.Contains(cd, `filename="grabi-M001-imposicion.pdf"`) {
		t.Errorf("kit-imposicion.pdf Content-Disposition = %q", cd)
	}
	if !bytes.HasPrefix(body, []byte("%PDF-1.")) {
		t.Fatalf("el cuerpo no es un PDF: %.16q", body)
	}

	// El endpoint sirve EXACTAMENTE el pliego de esta máquina, sin recortarlo ni
	// recomponerlo: así las pruebas de geometría de internal/kit (seis piezas con
	// sus medidas exactas, guías de corte y QR escaneable) valen también aquí.
	quiso, err := kit.Machine{ID: "M001", Place: "Palmira", Site: s.site}.Imposicion()
	if err != nil {
		t.Fatalf("kit.Imposicion(): %v", err)
	}
	if !bytes.Equal(body, quiso) {
		t.Errorf("el PDF servido (%d bytes) no es el pliego de M001 (%d bytes)", len(body), len(quiso))
	}

	// Una máquina que no existe no genera papelería fantasma.
	res, _ = pide(t, c, srv.URL+"/admin/machines/M999/kit-imposicion.pdf")
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("kit-imposicion.pdf de una máquina inexistente = %d, se esperaba 404", res.StatusCode)
	}
}

// El bloque vive DENTRO del detalle de la máquina, no en una sección aparte.
func TestElDetalleDeLaMaquinaMuestraElKitFisico(t *testing.T) {
	s, st := newTestServerConBase(t)
	if err := st.CreateMachine(context.Background(), "M001", "Palmira", "k1", 4); err != nil {
		t.Fatalf("CreateMachine: %v", err)
	}
	srv := httptest.NewServer(s.Routes())
	defer srv.Close()

	_, body := pide(t, clienteConSesion(t, srv.URL), srv.URL+"/admin/m/M001")
	html := string(body)
	for _, want := range []string{
		"Kit físico",
		"Descargar QR",
		"Descargar kit de stickers (.zip)",
		`href="/admin/machines/M001/qr.svg?size=1024"`,
		`href="/admin/machines/M001/kit.zip"`,
		"Hoja para imprenta (.pdf)",
		`href="/admin/machines/M001/kit-imposicion.pdf"`,
		`src="/admin/machines/M001/qr.svg?size=180"`, // previsualización del QR
		"1000 × 320 mm a escala 1:1",                 // medida del pliego, desde las constantes
	} {
		if !strings.Contains(html, want) {
			t.Errorf("el detalle de la máquina no contiene %q", want)
		}
	}
	// La placa ya NO se previsualiza (decisión de Daniel, 2026-08-13): se descarga
	// con el resto de piezas. Si vuelve a colarse el SVG incrustado, esto avisa.
	for _, noQuiere := range []string{`width="90mm" height="30mm"`, "kitplaca"} {
		if strings.Contains(html, noQuiere) {
			t.Errorf("el detalle de la máquina sigue incrustando la placa (%q)", noQuiere)
		}
	}
}
