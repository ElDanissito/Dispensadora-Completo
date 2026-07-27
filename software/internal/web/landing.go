package web

// Landing pública (home en GET /) + recepción del formulario B2B de interesados,
// según `especificaciones/landing-v2.md`. Es una capa NUEVA: no toca el flujo
// /m/{id} → conciliación → QR ni el contrato del token.

import (
	"html/template"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"dispensadoras/software/internal/config"
	"dispensadoras/software/internal/qr"
	"dispensadoras/software/internal/store"
)

// leadSource es el origen que se guarda en `leads.source`: hoy el único canal es
// el formulario de la landing; la columna deja la puerta abierta a otros.
const leadSource = "landing"

// Rate-limit ligero del formulario (landing-v2 §5): sin CAPTCHA, solo un tope
// por IP en memoria. Basta para el piloto (un solo proceso).
const (
	leadMaxPerWindow = 5
	leadWindow       = 10 * time.Minute
)

// Topes de longitud: el formulario es público, no confiamos en el cliente.
const (
	maxLeadPhone = 32
	maxLeadName  = 120
	maxLeadCity  = 80
)

// spaceTypes son los tipos de espacio del formulario B2B (valor → etiqueta). El
// servidor solo acepta estas claves: un select no es una promesa, es una lista.
var spaceTypes = []struct{ Key, Label string }{
	{"conjunto", "Conjunto residencial"},
	{"oficina", "Oficina / coworking"},
	{"negocio", "Negocio / local comercial"},
	{"otro", "Otro"},
}

func validSpaceType(v string) bool {
	for _, st := range spaceTypes {
		if st.Key == v {
			return true
		}
	}
	return false
}

// spaceTypeLabel traduce la clave guardada en `leads.space_type` a su etiqueta
// legible (formulario y panel de interesados). Si la clave no está en la lista
// —dato viejo o insertado a mano— se muestra tal cual, sin ocultarla.
func spaceTypeLabel(key string) string {
	for _, st := range spaceTypes {
		if st.Key == key {
			return st.Label
		}
	}
	return key
}

// leadForm son los campos del formulario B2B de la landing (landing-v2 §5):
// nombre, tipo de espacio, ciudad y WhatsApp, todos obligatorios. Reemplaza el
// formulario de correo+celular de landing-v1 §3: el canal real es WhatsApp.
type leadForm struct {
	Name      string
	SpaceType string
	City      string
	Phone     string
}

// SpaceTypeLabel devuelve la etiqueta legible del tipo elegido (para la vista).
func (f leadForm) SpaceTypeLabel() string { return spaceTypeLabel(f.SpaceType) }

// handleLanding sirve la home pública de la marca (landing-v2 §1). Es también el
// destino del botón "volver a inicio" de las páginas de error/404.
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
			Form        leadForm
			Error       string
			Sent        bool
			Year        int
			DemoQR      template.URL
			SpaceTypes  []struct{ Key, Label string }
			WhatsApp    string // "" ⇒ no se muestra el botón de WhatsApp
			WhatsAppMsg string
		}{
			Form: f, Error: errMsg, Sent: sent, Year: time.Now().In(bogota).Year(),
			DemoQR: demoQR, SpaceTypes: spaceTypes,
			WhatsApp:    config.WhatsApp(),
			WhatsAppMsg: url.QueryEscape("Hola, quiero saber más sobre tener una máquina GRABI"),
		},
	})
}

// handleInteresado recibe el formulario B2B de la landing (landing-v2 §5) y
// persiste el lead en la tabla `leads` (landing-v1 §5). Antes de existir la tabla
// el lead solo se escribía en el log; ahora la base es la fuente de verdad y el
// log queda como rastro sin PII. La sección "Interesados" del panel (landing-v1
// §4) sigue pendiente: hasta entonces los leads se consultan en la base.
func (s *Server) handleInteresado(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderLanding(w, r, http.StatusBadRequest, leadForm{}, "No pudimos leer el formulario. Inténtalo de nuevo.", false)
		return
	}
	f := leadForm{
		Name:      clip(r.FormValue("name"), maxLeadName),
		SpaceType: strings.TrimSpace(r.FormValue("space_type")),
		City:      clip(r.FormValue("city"), maxLeadCity),
		Phone:     clip(r.FormValue("phone"), maxLeadPhone),
	}

	// Honeypot: solo un bot rellena un campo que las personas no ven. Se responde
	// como si todo hubiera salido bien, sin registrar nada.
	if strings.TrimSpace(r.FormValue("website")) != "" {
		http.Redirect(w, r, "/?gracias=1#contacto", http.StatusSeeOther)
		return
	}

	if len([]rune(f.Name)) < 2 {
		s.renderLanding(w, r, http.StatusBadRequest, f, "Escribe tu nombre para saber cómo llamarte.", false)
		return
	}
	if !validSpaceType(f.SpaceType) {
		s.renderLanding(w, r, http.StatusBadRequest, f, "Elige el tipo de espacio donde iría la máquina.", false)
		return
	}
	if len([]rune(f.City)) < 2 {
		s.renderLanding(w, r, http.StatusBadRequest, f, "Escribe la ciudad del espacio.", false)
		return
	}
	if n := len(digitsOf(f.Phone)); n < 7 || n > 15 {
		s.renderLanding(w, r, http.StatusBadRequest, f, "Revisa el WhatsApp: escríbelo con al menos 7 dígitos.", false)
		return
	}
	if !s.allowLead(clientIP(r), time.Now()) {
		s.renderLanding(w, r, http.StatusTooManyRequests, f,
			"Recibimos varios envíos desde tu conexión. Espera unos minutos e inténtalo de nuevo.", false)
		return
	}

	// Persistir el lead (PII mínima: lo necesario para contactar, landing-v2 §5).
	id, err := s.st.CreateLead(r.Context(), store.Lead{
		Name: f.Name, SpaceType: f.SpaceType, City: f.City, WhatsApp: f.Phone,
		Source: leadSource,
	})
	if err != nil {
		// Si la base falla, el lead no se pierde: queda en el log (con PII, como
		// antes de existir la tabla) y se le pide reintentar en vez de mentirle.
		log.Printf("guardando interesado: %v — lead source=%s name=%q space_type=%q city=%q whatsapp=%q",
			err, leadSource, f.Name, f.SpaceType, f.City, f.Phone)
		s.renderLanding(w, r, http.StatusInternalServerError, f,
			"No pudimos guardar tus datos. Inténtalo de nuevo en un momento.", false)
		return
	}
	// Rastro operativo sin PII: el nombre y el WhatsApp ya están en la base.
	log.Printf("interesado guardado id=%d source=%s space_type=%q city=%q", id, leadSource, f.SpaceType, f.City)

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
