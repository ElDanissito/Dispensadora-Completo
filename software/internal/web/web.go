// Package web sirve la página pública por máquina (GET /m/{id}) y un panel de
// administración mínimo (crear máquinas, cargar productos/precios/stock, ver
// órdenes). Front ligero con html/template server-rendered (ADR-002); sin JS
// pesado para que /m/{id} cargue rápido en el celular del cliente.
package web

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"dispensadoras/software/internal/config"
	"dispensadoras/software/internal/dsptoken"
	"dispensadoras/software/internal/qr"
	"dispensadoras/software/internal/store"
)

// DefaultPayWindow es la ventana de validez de pago por defecto (spec §3):
// tiempo que el cliente tiene para transferir tras crear la orden.
const DefaultPayWindow = 15 * time.Minute

// Configuración de sesión del panel (ui-web-v1 §5): cookie propia, no Basic Auth.
const (
	sessionCookie = "grabi_session"
	sessionTTL    = 12 * time.Hour
)

// Configuración de subida de imágenes (ui-web-v1 §3.1).
const maxImageBytes = 5 << 20 // 5 MiB

// extPorTipo son los tipos de imagen aceptados (Content-Type detectado → extensión).
var extPorTipo = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

//go:embed templates/*.html
var tmplFS embed.FS

// bogota es la zona horaria del piloto (Colombia, UTC-5, sin DST).
var bogota = time.FixedZone("COT", -5*3600)

// funcs expuestas a las plantillas.
var funcs = template.FuncMap{
	// cop formatea pesos colombianos: 2500 → "$2.500".
	"cop": func(v int64) string {
		s := strconv.FormatInt(v, 10)
		neg := ""
		if v < 0 {
			neg, s = "-", s[1:]
		}
		var out []byte
		for i, d := range []byte(s) {
			if i > 0 && (len(s)-i)%3 == 0 {
				out = append(out, '.')
			}
			out = append(out, d)
		}
		return neg + "$" + string(out)
	},
	// ts formatea un epoch en hora de Colombia.
	"ts": func(sec int64) string {
		return time.Unix(sec, 0).In(bogota).Format("2006-01-02 15:04")
	},
}

// Server agrupa las dependencias de los handlers.
type Server struct {
	st        *store.Store
	tmpl      *template.Template
	adminUser string
	adminPass string
	priv      ed25519.PrivateKey // llave privada de firma; nil ⇒ no se pueden emitir QR
	allowSim  bool               // habilita el atajo de pruebas /simular-pago (nunca en prod pública)
	payWindow time.Duration      // ventana de validez de pago de las órdenes
	uploadDir string             // carpeta de datos donde se guardan las fotos subidas (git-ignored)
	uniqueAmt bool               // true = fallback por monto único; false = monto exacto + nombre (ADR-018)

	mu       sync.Mutex           // protege sessions
	sessions map[string]time.Time // token de sesión → expiración (ui-web-v1 §5)
}

// New construye el servidor. adminUser/adminPass son las credenciales del panel
// (ui-web-v1 §5: página de login propia + sesión por cookie, no Basic Auth).
// priv es la llave privada Ed25519 con la que se firman los tokens; si es nil, la
// emisión del QR (al conciliar el pago) responde con aviso. allowSim habilita el
// atajo de pruebas POST /m/{id}/simular-pago (déjalo en false en producción, spec
// §8). payWindow ≤ 0 usa DefaultPayWindow. uploadDir es la carpeta donde se
// guardan las fotos de producto (se crea si no existe; git-ignored).
func New(st *store.Store, adminUser, adminPass string, priv ed25519.PrivateKey, allowSim bool, payWindow time.Duration, uploadDir string, uniqueAmt bool) (*Server, error) {
	// Cada página se compone de base.html + su propia plantilla "content".
	// Parseamos todo junto: base define "base"; cada archivo redefine "content",
	// así que renderizamos clonando y parseando la página concreta bajo demanda.
	base, err := template.New("base").Funcs(funcs).ParseFS(tmplFS, "templates/base.html")
	if err != nil {
		return nil, err
	}
	if payWindow <= 0 {
		payWindow = DefaultPayWindow
	}
	if uploadDir == "" {
		uploadDir = "data/uploads"
	}
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return nil, fmt.Errorf("creando carpeta de subidas %s: %w", uploadDir, err)
	}
	s := &Server{st: st, tmpl: base, adminUser: adminUser, adminPass: adminPass,
		priv: priv, allowSim: allowSim, payWindow: payWindow, uploadDir: uploadDir,
		uniqueAmt: uniqueAmt, sessions: make(map[string]time.Time)}
	return s, nil
}

// --- Sesiones del panel (ui-web-v1 §5) ---

// newSession crea un token de sesión aleatorio y lo registra con su expiración.
func (s *Server) newSession() string {
	var b [24]byte
	_, _ = rand.Read(b[:])
	tok := hex.EncodeToString(b[:])
	s.mu.Lock()
	s.sessions[tok] = time.Now().Add(sessionTTL)
	s.mu.Unlock()
	return tok
}

// validSession indica si el token existe y no ha expirado (limpia los vencidos).
func (s *Server) validSession(tok string) bool {
	if tok == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.sessions[tok]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(s.sessions, tok)
		return false
	}
	return true
}

func (s *Server) dropSession(tok string) {
	s.mu.Lock()
	delete(s.sessions, tok)
	s.mu.Unlock()
}

// Routes registra todas las rutas y devuelve el handler raíz.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Pública. El flujo real: elegir productos → POST /pagar (crea orden PENDIENTE
	// con monto único) → pantalla de pago que consulta el estado hasta que la
	// conciliación por correo confirme el abono y emita el QR (spec §8).
	mux.HandleFunc("GET /m/{id}", s.handleMachinePublic)
	mux.HandleFunc("POST /m/{id}/pagar", s.handlePagar)
	mux.HandleFunc("GET /m/{id}/orden/{jti}/estado", s.handleEstadoOrden)

	// Atajo de pruebas (firma el QR sin pago real). Solo si allowSim (spec §8).
	mux.HandleFunc("POST /m/{id}/simular-pago", s.handleSimularPago)

	// Fotos de producto subidas por el admin (ui-web-v1 §3.1). Servidas estáticas.
	fs := http.StripPrefix("/uploads/", http.FileServer(http.Dir(s.uploadDir)))
	mux.Handle("GET /uploads/", fs)

	// Login del panel (ui-web-v1 §5): página propia + sesión por cookie.
	mux.HandleFunc("GET /admin/login", s.handleLoginForm)
	mux.HandleFunc("POST /admin/login", s.handleLoginSubmit)
	mux.HandleFunc("POST /admin/logout", s.handleLogout)

	// Admin (protegido por sesión; sin sesión → redirige al login).
	mux.Handle("GET /admin", s.auth(http.HandlerFunc(s.handleAdminDashboard)))
	mux.Handle("POST /admin/machines", s.auth(http.HandlerFunc(s.handleCreateMachine)))
	mux.Handle("GET /admin/m/{id}", s.auth(http.HandlerFunc(s.handleAdminMachine)))
	// CRUD de productos por máquina (ui-web-v1 §3): crear / editar / eliminar.
	mux.Handle("POST /admin/m/{id}/products", s.auth(http.HandlerFunc(s.handleCreateProduct)))
	mux.Handle("GET /admin/m/{id}/slot/{slot}/edit", s.auth(http.HandlerFunc(s.handleEditSlotForm)))
	mux.Handle("POST /admin/m/{id}/slot/{slot}", s.auth(http.HandlerFunc(s.handleUpdateSlot)))
	mux.Handle("POST /admin/m/{id}/slot/{slot}/delete", s.auth(http.HandlerFunc(s.handleDeleteSlot)))
	mux.Handle("GET /admin/orders", s.auth(http.HandlerFunc(s.handleAdminOrders)))

	// Raíz → panel.
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	})
	return mux
}

// --- render ---

// page es el envoltorio que reciben todas las plantillas.
type page struct {
	Title string
	Admin bool // muestra la navegación de administración
	Data  any
}

// render compone base.html con la plantilla `name` (que define "content").
func (s *Server) render(w http.ResponseWriter, name string, p page) {
	t, err := s.tmpl.Clone()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := t.ParseFS(tmplFS, "templates/"+name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base", p); err != nil {
		// El header ya pudo haberse enviado; solo registramos vía el error visible.
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// --- Sesión de admin (ui-web-v1 §5) ---

// auth protege las rutas del panel: si no hay una sesión válida por cookie,
// redirige a la página de login (no Basic Auth del navegador).
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil || !s.validSession(c.Value) {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleLoginForm muestra la página de login (o redirige al panel si ya hay sesión).
func (s *Server) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil && s.validSession(c.Value) {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	s.renderLogin(w, false)
}

// handleLoginSubmit valida credenciales (comparación en tiempo constante) y, si
// son correctas, crea la sesión y fija la cookie. Credenciales por variable de
// entorno (ADMIN_USER/ADMIN_PASS), nunca hardcodeadas (ui-web-v1 §5, CLAUDE.md §4).
func (s *Server) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulario inválido", http.StatusBadRequest)
		return
	}
	user, pass := r.FormValue("user"), r.FormValue("pass")
	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(s.adminUser)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(s.adminPass)) == 1
	if !userOK || !passOK {
		w.WriteHeader(http.StatusUnauthorized)
		s.renderLogin(w, true)
		return
	}
	tok := s.newSession()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil, // en producción (TLS) la cookie solo viaja por HTTPS
		Expires:  time.Now().Add(sessionTTL),
	})
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// handleLogout cierra la sesión (borra el token y expira la cookie).
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.dropSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1,
	})
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

func (s *Server) renderLogin(w http.ResponseWriter, failed bool) {
	s.render(w, "admin_login.html", page{
		Title: "Ingresar · GRABI",
		Data:  struct{ Failed bool }{failed},
	})
}

// --- Handlers ---

func (s *Server) handleMachinePublic(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := s.st.GetMachine(r.Context(), id)
	if err != nil {
		s.notFound(w, "Máquina no encontrada")
		return
	}
	cat, err := s.st.Catalog(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "machine_public.html", page{
		Title: "Máquina " + m.ID,
		Admin: false,
		Data: struct {
			Machine   *store.Machine
			Catalog   []store.CatalogRow
			PayerMode bool // pide el nombre de quien paga (ADR-018)
		}{m, cat, !s.uniqueAmt},
	})
}

// buildItems lee las cantidades del formulario (qty_<slot>), las valida contra el
// catálogo (slots existentes y stock suficiente) y devuelve los items del token,
// las líneas de la orden y el total base (suma de precios). Es compartida por el
// flujo real (/pagar) y el atajo de pruebas (/simular-pago).
func (s *Server) buildItems(r *http.Request, cat []store.CatalogRow) ([]dsptoken.Item, []store.OrderItem, int64, error) {
	if err := r.ParseForm(); err != nil {
		return nil, nil, 0, errors.New("formulario inválido")
	}
	var items []dsptoken.Item
	var orderItems []store.OrderItem
	var total int64
	for _, row := range cat {
		qty, _ := strconv.Atoi(r.FormValue(fmt.Sprintf("qty_%d", row.Slot)))
		if qty <= 0 {
			continue
		}
		if qty > row.Stock {
			return nil, nil, 0, fmt.Errorf("slot %d (%s): pediste %d pero hay %d en stock",
				row.Slot, row.ProductName, qty, row.Stock)
		}
		items = append(items, dsptoken.Item{S: row.Slot, Q: qty})
		orderItems = append(orderItems, store.OrderItem{Slot: row.Slot, Qty: qty, PriceCOP: row.PriceCOP})
		total += row.PriceCOP * int64(qty)
	}
	if len(items) == 0 {
		return nil, nil, 0, errors.New("selecciona al menos un producto")
	}
	return items, orderItems, total, nil
}

// pickDesambiguador elige un desambiguador d ∈ [1,99] tal que (base+d) NO esté ya
// reservado por otra orden PENDIENTE de la misma cuenta (spec §2). Devuelve
// (d, true) o (0, false) si no hay ninguno libre (caso improbable en el piloto).
func pickDesambiguador(base int64, reservados map[int64]bool) (int, bool) {
	for d := 1; d <= 99; d++ {
		if !reservados[base+int64(d)] {
			return d, true
		}
	}
	return 0, false
}

// handlePagar (flujo REAL, spec §8): valida la selección, crea la orden PENDIENTE
// con un MONTO ÚNICO (base + desambiguador) y una ventana de pago, y redirige a la
// pantalla de estado donde el cliente ve cuánto y a qué llave Bre-B transferir. El
// QR NO se emite aquí: solo lo emite la conciliación al confirmar el abono real.
func (s *Server) handlePagar(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.st.GetMachine(r.Context(), id); err != nil {
		s.notFound(w, "Máquina no encontrada")
		return
	}
	cat, err := s.st.Catalog(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, orderItems, total, err := s.buildItems(r, cat)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Nombre de quien hará la transferencia (ADR-018): obligatorio en modo por
	// nombre; es lo que desambigua el pago en la conciliación.
	payerName := strings.TrimSpace(r.FormValue("payer_name"))
	if !s.uniqueAmt && payerName == "" {
		http.Error(w, "escribe el nombre de quien hará la transferencia (obligatorio para confirmar tu pago)", http.StatusBadRequest)
		return
	}

	now := time.Now().Unix()
	// Monto a cobrar: en modo por nombre es el TOTAL EXACTO (redondo); en el
	// fallback legado se le suma un desambiguador de pesos único entre pendientes.
	amount := total
	if s.uniqueAmt {
		reservados, err := s.st.PendingUniqueAmounts(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		d, ok := pickDesambiguador(total, reservados)
		if !ok {
			http.Error(w, "no hay un monto único disponible en este momento; intenta de nuevo en unos minutos", http.StatusServiceUnavailable)
			return
		}
		amount = total + int64(d)
	}

	jti := randomJTI()
	order := store.Order{
		Jti:                jti,
		MachineID:          id,
		TotalCOP:           total,
		UniqueAmount:       amount,
		PayerName:          payerName,
		Status:             "pending",
		Iat:                now,
		Exp:                0, // se fija al pagar (el reloj de exp arranca al pagar, spec §3)
		PayWindowExpiresAt: now + int64(s.payWindow.Seconds()),
		CreatedAt:          now,
		Items:              orderItems,
	}
	if err := s.st.CreateOrder(r.Context(), order); err != nil {
		http.Error(w, "no se pudo crear la orden: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/m/%s/orden/%s/estado", id, jti), http.StatusSeeOther)
}

// handleEstadoOrden muestra el estado de una orden. Es la pantalla que el cliente
// consulta tras /pagar; se auto-refresca (meta refresh, sin JS pesado, ADR-011bis)
// mientras esté PENDIENTE y cambia a QR cuando la conciliación la marca PAGADA.
func (s *Server) handleEstadoOrden(w http.ResponseWriter, r *http.Request) {
	id, jti := r.PathValue("id"), r.PathValue("jti")
	m, err := s.st.GetMachine(r.Context(), id)
	if err != nil {
		s.notFound(w, "Máquina no encontrada")
		return
	}
	o, err := s.st.GetOrder(r.Context(), jti)
	if err != nil || o.MachineID != id {
		s.notFound(w, "Orden no encontrada")
		return
	}

	switch o.Status {
	case "paid", "paid_sim", "dispensed":
		s.renderQR(w, m, o)
	case "ambiguous":
		// Regla de seguridad ADR-018: un pago casó con ≥2 órdenes; no se dispensa.
		// El cliente ve un mensaje de soporte (no un QR ni un error técnico).
		s.render(w, "machine_revision.html", page{
			Title: "Pago en revisión · Máquina " + id,
			Data: struct {
				Machine *store.Machine
			}{m},
		})
	case "expired", "canceled":
		s.render(w, "machine_expirada.html", page{
			Title: "Orden expirada · Máquina " + id,
			Data: struct {
				Machine *store.Machine
			}{m},
		})
	default: // pending
		secondsLeft := o.PayWindowExpiresAt - time.Now().Unix()
		if secondsLeft < 0 {
			secondsLeft = 0
		}
		s.render(w, "machine_pago.html", page{
			Title: "Paga tu compra · Máquina " + id,
			Data: struct {
				Machine       *store.Machine
				Order         *store.Order
				BreBKey       string
				PointOfSale   string
				PayerMode     bool // true = monto exacto + nombre (ADR-018); false = monto único
				Desambiguador int64
				SecondsLeft   int64
				MinutesLeft   int64
			}{
				Machine:       m,
				Order:         o,
				BreBKey:       config.BreBKey(id),
				PointOfSale:   "GRABI " + id, // punto de venta (ADR-014)
				PayerMode:     !s.uniqueAmt,
				Desambiguador: o.UniqueAmount - o.TotalCOP,
				SecondsLeft:   secondsLeft,
				MinutesLeft:   (secondsLeft + 59) / 60,
			},
		})
	}
}

// renderQR muestra el QR de una orden ya pagada (token firmado guardado en la
// orden). Sim indica si fue un pago simulado (para el aviso de la plantilla).
func (s *Server) renderQR(w http.ResponseWriter, m *store.Machine, o *store.Order) {
	if o.Token == "" {
		http.Error(w, "la orden está pagada pero no tiene token emitido (revisar servidor)", http.StatusInternalServerError)
		return
	}
	dataURI, err := qr.DataURI(o.Token, 512)
	if err != nil {
		http.Error(w, "no se pudo generar el QR: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "machine_qr.html", page{
		Title: "Tu QR · Máquina " + m.ID,
		Data: struct {
			Machine   *store.Machine
			Items     []store.OrderItem
			TotalCOP  int64
			Jti       string
			Exp       int64
			Token     string
			TokenLen  int
			QRDataURI template.URL
			Sim       bool
		}{
			Machine: m, Items: o.Items, TotalCOP: o.TotalCOP, Jti: o.Jti, Exp: o.Exp,
			Token: o.Token, TokenLen: len(o.Token), QRDataURI: template.URL(dataURI),
			Sim: o.Status == "paid_sim",
		},
	})
}

// SignOrder implementa concil.Emitter: firma el token v2 de una orden ya conciliada
// (contrato §4). Devuelve el JWS y su `exp`. NO toca la base; la transición
// pending→paid + descuento de stock la aplica la conciliación (store.MarkOrderPaid),
// para que sea atómica.
func (s *Server) SignOrder(ctx context.Context, o store.Order) (string, int64, error) {
	if s.priv == nil {
		return "", 0, errors.New("el servidor no tiene llave de firma cargada")
	}
	kid := dsptoken.DefaultKID
	if m, err := s.st.GetMachine(ctx, o.MachineID); err == nil {
		kid = m.Kid
	}
	exp := time.Now().Unix() + dsptoken.DefaultTTL // el reloj de 5 min arranca al pagar
	items := make([]dsptoken.Item, 0, len(o.Items))
	for _, it := range o.Items {
		items = append(items, dsptoken.Item{S: it.Slot, Q: it.Qty})
	}
	token, err := dsptoken.Sign(s.priv, dsptoken.DefaultHeader(kid), dsptoken.Payload{
		Mid: o.MachineID, Jti: o.Jti, Exp: exp, Items: items,
	})
	if err != nil {
		return "", 0, err
	}
	return token, exp, nil
}

// handleSimularPago (ATAJO DE PRUEBAS, spec §8): firma el QR sin pago real. Solo
// disponible si allowSim (flag/entorno); nunca en la ruta pública de producción.
// La orden queda marcada "paid_sim" (distinguible de un pago real).
func (s *Server) handleSimularPago(w http.ResponseWriter, r *http.Request) {
	if !s.allowSim {
		http.Error(w, "pago simulado deshabilitado (solo pruebas; usa el flujo real /pagar)", http.StatusForbidden)
		return
	}
	id := r.PathValue("id")
	if _, err := s.st.GetMachine(r.Context(), id); err != nil {
		s.notFound(w, "Máquina no encontrada")
		return
	}
	if s.priv == nil {
		http.Error(w, "el servidor no tiene llave de firma cargada; ejecuta 'dsp keygen' o define DSP_PRIVATE_KEY", http.StatusServiceUnavailable)
		return
	}
	cat, err := s.st.Catalog(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, orderItems, total, err := s.buildItems(r, cat)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	now := time.Now().Unix()
	jti := randomJTI()
	// Se crea PENDIENTE y se marca paid_sim con MarkOrderPaidSim (misma transición
	// atómica que un pago real: guarda el token y descuenta stock).
	order := store.Order{
		Jti: jti, MachineID: id, TotalCOP: total, UniqueAmount: total,
		Status: "pending", Iat: now, Exp: 0, PayWindowExpiresAt: now + int64(s.payWindow.Seconds()),
		CreatedAt: now, Items: orderItems,
	}
	if err := s.st.CreateOrder(r.Context(), order); err != nil {
		http.Error(w, "no se pudo crear la orden: "+err.Error(), http.StatusInternalServerError)
		return
	}
	token, exp, err := s.SignOrder(r.Context(), order)
	if err != nil {
		http.Error(w, "no se pudo firmar el token: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := s.st.MarkOrderPaidSim(r.Context(), jti, token, exp, now, "SIMULADO-"+jti); err != nil {
		http.Error(w, "no se pudo marcar la orden: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/m/%s/orden/%s/estado", id, jti), http.StatusSeeOther)
}

func (s *Server) handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	machines, err := s.st.ListMachines(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "admin_dashboard.html", page{
		Title: "Panel · GRABI",
		Admin: true,
		Data: struct {
			Machines []store.Machine
		}{machines},
	})
}

func (s *Server) handleCreateMachine(w http.ResponseWriter, r *http.Request) {
	id, name, kid := r.FormValue("id"), r.FormValue("name"), r.FormValue("kid")
	if id == "" || name == "" {
		http.Error(w, "id y nombre son obligatorios", http.StatusBadRequest)
		return
	}
	if err := s.st.CreateMachine(r.Context(), id, name, kid); err != nil {
		http.Error(w, "no se pudo crear la máquina: "+err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/admin/m/"+id, http.StatusSeeOther)
}

// handleAdminMachine renderiza la máquina en el panel: cuadrícula de productos por
// slot (imagen, nombre, precio, stock, cableado) + formulario de "nuevo producto".
// `flash` (query ?ok=) muestra un banner de confirmación tras una acción.
func (s *Server) handleAdminMachine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := s.st.GetMachine(r.Context(), id)
	if err != nil {
		s.notFound(w, "Máquina no encontrada")
		return
	}
	cat, err := s.st.Catalog(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "admin_machine.html", page{
		Title: "Máquina " + m.ID + " · GRABI",
		Admin: true,
		Data: struct {
			Machine *store.Machine
			Catalog []store.CatalogRow
			Flash   string
		}{m, cat, flashText(r.URL.Query().Get("ok"))},
	})
}

// slotForm son los campos comunes de crear/editar un producto en un slot.
type slotForm struct {
	Slot        int
	Name        string
	Description string
	PriceCOP    int64
	Stock       int
	Wired       bool
}

// parseSlotForm lee y valida el formulario multipart de producto (crear/editar).
func parseSlotForm(r *http.Request) (slotForm, error) {
	var f slotForm
	if err := r.ParseMultipartForm(maxImageBytes); err != nil {
		return f, errors.New("formulario inválido (¿imagen demasiado grande?)")
	}
	f.Name = strings.TrimSpace(r.FormValue("name"))
	f.Description = strings.TrimSpace(r.FormValue("description"))
	slot, err1 := strconv.Atoi(r.FormValue("slot"))
	price, err2 := strconv.ParseInt(r.FormValue("price"), 10, 64)
	stock, err3 := strconv.Atoi(r.FormValue("stock"))
	if err := errors.Join(err1, err2, err3); err != nil {
		return f, errors.New("slot, precio y stock deben ser números")
	}
	f.Slot, f.PriceCOP, f.Stock = slot, price, stock
	f.Wired = r.FormValue("wired") != ""
	if f.Name == "" {
		return f, errors.New("el nombre es obligatorio")
	}
	if slot < 1 || price < 0 || stock < 0 {
		return f, errors.New("slot ≥1, precio y stock ≥0")
	}
	return f, nil
}

// handleCreateProduct crea un producto (con foto opcional) y lo asigna a un slot
// de la máquina, en un solo paso (ui-web-v1 §3).
func (s *Server) handleCreateProduct(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.st.GetMachine(r.Context(), id); err != nil {
		s.notFound(w, "Máquina no encontrada")
		return
	}
	f, err := parseSlotForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Rechazar si el slot ya está ocupado (evita pisar otro producto sin querer).
	if _, err := s.st.GetSlot(r.Context(), id, f.Slot); err == nil {
		http.Error(w, fmt.Sprintf("el slot %d ya tiene un producto; edítalo o elige otro slot", f.Slot), http.StatusConflict)
		return
	}
	imagePath, err := s.saveImage(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	pid, err := s.st.CreateProductDetailed(r.Context(), f.Name, f.Description, imagePath)
	if err != nil {
		http.Error(w, "no se pudo crear el producto: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.st.SetSlot(r.Context(), id, f.Slot, pid, f.PriceCOP, f.Stock); err != nil {
		http.Error(w, "no se pudo asignar el slot: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.st.SetWired(r.Context(), id, f.Slot, f.Wired); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/m/"+id+"?ok=created", http.StatusSeeOther)
}

// handleEditSlotForm muestra el formulario de edición de un producto/slot.
func (s *Server) handleEditSlotForm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := s.st.GetMachine(r.Context(), id)
	if err != nil {
		s.notFound(w, "Máquina no encontrada")
		return
	}
	slot, err := strconv.Atoi(r.PathValue("slot"))
	if err != nil {
		http.Error(w, "slot inválido", http.StatusBadRequest)
		return
	}
	row, err := s.st.GetSlot(r.Context(), id, slot)
	if err != nil {
		s.notFound(w, "Slot no encontrado")
		return
	}
	s.render(w, "admin_product_edit.html", page{
		Title: "Editar producto · " + m.ID,
		Admin: true,
		Data: struct {
			Machine *store.Machine
			Row     *store.CatalogRow
		}{m, row},
	})
}

// handleUpdateSlot actualiza el producto (nombre/desc/foto) y su asignación al
// slot (precio/stock/cableado). Permite MOVER el producto a otro slot: si el slot
// nuevo difiere del original, se libera el viejo (ui-web-v1 §4, aviso de slot).
func (s *Server) handleUpdateSlot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.st.GetMachine(r.Context(), id); err != nil {
		s.notFound(w, "Máquina no encontrada")
		return
	}
	origSlot, err := strconv.Atoi(r.PathValue("slot"))
	if err != nil {
		http.Error(w, "slot inválido", http.StatusBadRequest)
		return
	}
	current, err := s.st.GetSlot(r.Context(), id, origSlot)
	if err != nil {
		s.notFound(w, "Slot no encontrado")
		return
	}
	f, err := parseSlotForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Si cambia de slot, el destino no puede estar ocupado por OTRO producto.
	if f.Slot != origSlot {
		if _, err := s.st.GetSlot(r.Context(), id, f.Slot); err == nil {
			http.Error(w, fmt.Sprintf("el slot %d ya está ocupado; elige otro", f.Slot), http.StatusConflict)
			return
		}
	}
	imagePath, err := s.saveImage(r) // "" si no subió nueva (UpdateProduct conserva la actual)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.st.UpdateProduct(r.Context(), current.ProductID, f.Name, f.Description, imagePath); err != nil {
		http.Error(w, "no se pudo actualizar el producto: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Asignar el slot destino ANTES de liberar el viejo: así el producto nunca
	// queda con 0 referencias entre pasos (DeleteSlot recolecta productos huérfanos
	// y borrar primero dispararía un fallo de FK al reinsertar).
	if err := s.st.SetSlot(r.Context(), id, f.Slot, current.ProductID, f.PriceCOP, f.Stock); err != nil {
		http.Error(w, "no se pudo guardar el slot: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.st.SetWired(r.Context(), id, f.Slot, f.Wired); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if f.Slot != origSlot {
		if err := s.st.DeleteSlot(r.Context(), id, origSlot); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	http.Redirect(w, r, "/admin/m/"+id+"?ok=updated", http.StatusSeeOther)
}

// handleDeleteSlot elimina un producto de un slot (ui-web-v1 §3). El aviso de
// "aún hay stock físico" lo muestra el front antes de enviar (ui-web-v1 §4).
func (s *Server) handleDeleteSlot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	slot, err := strconv.Atoi(r.PathValue("slot"))
	if err != nil {
		http.Error(w, "slot inválido", http.StatusBadRequest)
		return
	}
	if err := s.st.DeleteSlot(r.Context(), id, slot); err != nil {
		http.Error(w, "no se pudo eliminar: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/m/"+id+"?ok=deleted", http.StatusSeeOther)
}

// saveImage guarda la foto subida (campo "image") en uploadDir y devuelve su ruta
// pública (/uploads/<archivo>). Devuelve "" (sin error) si no se subió archivo, o
// error si el tipo no es una imagen soportada o supera el límite de tamaño.
func (s *Server) saveImage(r *http.Request) (string, error) {
	file, hdr, err := r.FormFile("image")
	if err == http.ErrMissingFile || hdr == nil {
		return "", nil // el admin no subió foto (permitido; el front sugiere hacerlo)
	}
	if err != nil {
		return "", errors.New("no se pudo leer la imagen subida")
	}
	defer file.Close()
	if hdr.Size > maxImageBytes {
		return "", fmt.Errorf("la imagen supera el máximo de %d MB", maxImageBytes>>20)
	}

	// Detectar el tipo real por los primeros bytes (no confiar en la extensión).
	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	ctype := http.DetectContentType(head[:n])
	ext, ok := extPorTipo[ctype]
	if !ok {
		return "", fmt.Errorf("tipo de imagen no soportado (%s); usa JPG, PNG, WEBP o GIF", ctype)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", errors.New("no se pudo procesar la imagen")
	}

	var rnd [8]byte
	_, _ = rand.Read(rnd[:])
	fname := hex.EncodeToString(rnd[:]) + ext
	dst, err := os.Create(filepath.Join(s.uploadDir, fname))
	if err != nil {
		return "", fmt.Errorf("no se pudo guardar la imagen: %w", err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, io.LimitReader(file, maxImageBytes)); err != nil {
		return "", fmt.Errorf("no se pudo guardar la imagen: %w", err)
	}
	return "/uploads/" + fname, nil
}

// flashText traduce el código de ?ok= a un texto de confirmación para el banner.
func flashText(code string) string {
	switch code {
	case "created":
		return "Producto creado y asignado al slot."
	case "updated":
		return "Cambios guardados."
	case "deleted":
		return "Producto eliminado del slot."
	default:
		return ""
	}
}

func (s *Server) handleAdminOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := s.st.ListOrders(r.Context(), 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "admin_orders.html", page{
		Title: "Órdenes · Dispensadoras",
		Admin: true,
		Data:  struct{ Orders []store.Order }{orders},
	})
}

func (s *Server) notFound(w http.ResponseWriter, msg string) {
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, "%s", msg)
}

// randomJTI genera un id de orden único (mismo formato que el CLI dsp: "ord_"+
// 5 bytes hex). Es el jti de un solo uso del token (contrato §3).
func randomJTI() string {
	var b [5]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "ord_00000000"
	}
	return "ord_" + hex.EncodeToString(b[:])
}
