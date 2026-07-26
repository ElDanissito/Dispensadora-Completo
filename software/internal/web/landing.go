package web

// Landing pública (home en GET /) + recepción del formulario de interesados,
// según `especificaciones/landing-v1.md`. Es una capa NUEVA: no toca el flujo
// /m/{id} → conciliación → QR ni el contrato del token.

import (
	"html/template"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"dispensadoras/software/internal/qr"
)

// Rate-limit ligero del formulario (landing-v1 §3): sin CAPTCHA, solo un tope
// por IP en memoria. Basta para el piloto (un solo proceso).
const (
	leadMaxPerWindow = 5
	leadWindow       = 10 * time.Minute
)

// Topes de longitud: el formulario es público, no confiamos en el cliente.
const (
	maxLeadEmail   = 254
	maxLeadPhone   = 32
	maxLeadName    = 120
	maxLeadMessage = 1000
)

// emailRe es una validación de formato deliberadamente laxa (algo@algo.algo):
// el objetivo es atajar errores de dedo, no rechazar correos válidos raros.
var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// leadForm son los campos del formulario de interesados (landing-v1 §3):
// correo y celular obligatorios; nombre y mensaje opcionales.
type leadForm struct {
	Name    string
	Email   string
	Phone   string
	Message string
}

// handleLanding sirve la home pública de la marca (landing-v1 §1). Es también el
// destino del botón "volver a inicio" de las páginas de error/404 (§6).
func (s *Server) handleLanding(w http.ResponseWriter, r *http.Request) {
	// ?gracias=1 es el destino del POST tras guardar (patrón POST→redirect→GET,
	// así un refresco no reenvía el formulario).
	s.renderLanding(w, r, http.StatusOK, leadForm{}, "", r.URL.Query().Get("gracias") == "1")
}

// renderLanding pinta la landing. `sent` muestra el estado de éxito en lugar del
// formulario; `errMsg` muestra el aviso de validación conservando lo escrito.
func (s *Server) renderLanding(w http.ResponseWriter, r *http.Request, status int, f leadForm, errMsg string, sent bool) {
	// QR ilustrativo de la maqueta del hero: apunta a esta misma web (NO es un
	// token de dispensado, solo la imagen de la pantalla de compra).
	var demoQR template.URL
	if uri, err := qr.DataURI(siteURL(r), 240); err == nil {
		demoQR = template.URL(uri)
	}
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	s.render(w, "landing.html", page{
		Title:   "GRABI · Escanea, paga, agárralo.",
		Landing: true,
		Data: struct {
			Form   leadForm
			Error  string
			Sent   bool
			Year   int
			DemoQR template.URL
		}{f, errMsg, sent, time.Now().In(bogota).Year(), demoQR},
	})
}

// handleInteresado recibe el formulario de la landing (landing-v1 §3).
//
// TODO(leads): la tabla `leads` (spec §5) y la sección "Interesados" del panel
// (§4) llegan en el commit siguiente. Hasta entonces el lead NO se persiste en
// la base: se registra en el log del servidor para no perderlo (revisar con
// `docker compose logs app`). Al existir la tabla, basta reemplazar el log por
// el insert; el resto del flujo (validación, honeypot, rate-limit) ya está.
func (s *Server) handleInteresado(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderLanding(w, r, http.StatusBadRequest, leadForm{}, "No pudimos leer el formulario. Inténtalo de nuevo.", false)
		return
	}
	f := leadForm{
		Name:    clip(r.FormValue("name"), maxLeadName),
		Email:   clip(r.FormValue("email"), maxLeadEmail),
		Phone:   clip(r.FormValue("phone"), maxLeadPhone),
		Message: clip(r.FormValue("message"), maxLeadMessage),
	}

	// Honeypot: solo un bot rellena un campo que las personas no ven. Se responde
	// como si todo hubiera salido bien, sin registrar nada.
	if strings.TrimSpace(r.FormValue("website")) != "" {
		http.Redirect(w, r, "/?gracias=1#contacto", http.StatusSeeOther)
		return
	}

	if !emailRe.MatchString(f.Email) {
		s.renderLanding(w, r, http.StatusBadRequest, f, "Revisa el correo: debe tener el formato tu@correo.com.", false)
		return
	}
	if n := len(digitsOf(f.Phone)); n < 7 || n > 15 {
		s.renderLanding(w, r, http.StatusBadRequest, f, "Revisa el celular: escríbelo con al menos 7 dígitos.", false)
		return
	}
	if !s.allowLead(clientIP(r), time.Now()) {
		s.renderLanding(w, r, http.StatusTooManyRequests, f,
			"Recibimos varios envíos desde tu conexión. Espera unos minutos e inténtalo de nuevo.", false)
		return
	}

	// PII: se registra lo mínimo para poder contactar (landing-v1 §3).
	log.Printf("interesado (landing) source=landing email=%q phone=%q name=%q message=%q",
		f.Email, f.Phone, f.Name, f.Message)

	http.Redirect(w, r, "/?gracias=1#contacto", http.StatusSeeOther)
}

// allowLead aplica el tope por IP y registra el intento. Devuelve false si la IP
// ya superó leadMaxPerWindow envíos dentro de la ventana.
func (s *Server) allowLead(ip string, now time.Time) bool {
	s.leadMu.Lock()
	defer s.leadMu.Unlock()
	cutoff := now.Add(-leadWindow)
	// Limpieza oportunista de todas las IPs: el mapa no crece sin control.
	for k, hits := range s.leadHits {
		fresh := hits[:0]
		for _, t := range hits {
			if t.After(cutoff) {
				fresh = append(fresh, t)
			}
		}
		if len(fresh) == 0 {
			delete(s.leadHits, k)
		} else {
			s.leadHits[k] = fresh
		}
	}
	if len(s.leadHits[ip]) >= leadMaxPerWindow {
		return false
	}
	s.leadHits[ip] = append(s.leadHits[ip], now)
	return true
}

// clientIP obtiene la IP del cliente teniendo en cuenta el reverse-proxy Caddy
// (X-Forwarded-For); cae a RemoteAddr en local.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(xff)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// siteURL reconstruye la URL pública de esta web (para el QR ilustrativo).
func siteURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// clip normaliza un campo del formulario: sin espacios sobrantes y con tope de
// caracteres (por runas, para no cortar una tilde a medias).
func clip(v string, max int) string {
	v = strings.TrimSpace(v)
	if rs := []rune(v); len(rs) > max {
		v = strings.TrimSpace(string(rs[:max]))
	}
	return v
}

// digitsOf deja solo los dígitos (para contar los del celular).
func digitsOf(v string) string {
	var b strings.Builder
	for _, r := range v {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
