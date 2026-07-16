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
	"net/http"
	"strconv"
	"time"

	"dispensadoras/software/internal/config"
	"dispensadoras/software/internal/dsptoken"
	"dispensadoras/software/internal/qr"
	"dispensadoras/software/internal/store"
)

// DefaultPayWindow es la ventana de validez de pago por defecto (spec §3):
// tiempo que el cliente tiene para transferir tras crear la orden.
const DefaultPayWindow = 15 * time.Minute

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
}

// New construye el servidor. adminUser/adminPass protegen /admin con Basic Auth.
// priv es la llave privada Ed25519 con la que se firman los tokens; si es nil, la
// emisión del QR (al conciliar el pago) responde con aviso. allowSim habilita el
// atajo de pruebas POST /m/{id}/simular-pago (déjalo en false en producción, spec
// §8). payWindow ≤ 0 usa DefaultPayWindow.
func New(st *store.Store, adminUser, adminPass string, priv ed25519.PrivateKey, allowSim bool, payWindow time.Duration) (*Server, error) {
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
	s := &Server{st: st, tmpl: base, adminUser: adminUser, adminPass: adminPass,
		priv: priv, allowSim: allowSim, payWindow: payWindow}
	return s, nil
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

	// Admin (protegido con Basic Auth).
	mux.Handle("GET /admin", s.auth(http.HandlerFunc(s.handleAdminDashboard)))
	mux.Handle("POST /admin/machines", s.auth(http.HandlerFunc(s.handleCreateMachine)))
	mux.Handle("POST /admin/products", s.auth(http.HandlerFunc(s.handleCreateProduct)))
	mux.Handle("GET /admin/m/{id}", s.auth(http.HandlerFunc(s.handleAdminMachine)))
	mux.Handle("POST /admin/m/{id}/slot", s.auth(http.HandlerFunc(s.handleSetSlot)))
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

// --- Basic Auth para /admin ---

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(user), []byte(s.adminUser)) != 1 ||
			subtle.ConstantTimeCompare([]byte(pass), []byte(s.adminPass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="panel dispensadoras"`)
			http.Error(w, "no autorizado", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
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
			Machine *store.Machine
			Catalog []store.CatalogRow
		}{m, cat},
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

	now := time.Now().Unix()
	jti := randomJTI()
	order := store.Order{
		Jti:                jti,
		MachineID:          id,
		TotalCOP:           total,
		UniqueAmount:       total + int64(d),
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
				Desambiguador int64
				SecondsLeft   int64
				MinutesLeft   int64
			}{
				Machine:       m,
				Order:         o,
				BreBKey:       config.BreBKey(id),
				PointOfSale:   "GRABI " + id, // punto de venta (ADR-014)
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
	products, err := s.st.ListProducts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "admin_dashboard.html", page{
		Title: "Panel · Dispensadoras",
		Admin: true,
		Data: struct {
			Machines []store.Machine
			Products []store.Product
		}{machines, products},
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

func (s *Server) handleCreateProduct(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "el nombre es obligatorio", http.StatusBadRequest)
		return
	}
	if _, err := s.st.CreateProduct(r.Context(), name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

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
	products, err := s.st.ListProducts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "admin_machine.html", page{
		Title: "Máquina " + m.ID,
		Admin: true,
		Data: struct {
			Machine  *store.Machine
			Catalog  []store.CatalogRow
			Products []store.Product
		}{m, cat, products},
	})
}

func (s *Server) handleSetSlot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	slot, err1 := strconv.Atoi(r.FormValue("slot"))
	productID, err2 := strconv.ParseInt(r.FormValue("product_id"), 10, 64)
	price, err3 := strconv.ParseInt(r.FormValue("price"), 10, 64)
	stock, err4 := strconv.Atoi(r.FormValue("stock"))
	if err := errors.Join(err1, err2, err3, err4); err != nil {
		http.Error(w, "datos de slot inválidos: "+err.Error(), http.StatusBadRequest)
		return
	}
	if slot < 1 || price < 0 || stock < 0 {
		http.Error(w, "slot ≥1, precio y stock ≥0", http.StatusBadRequest)
		return
	}
	if err := s.st.SetSlot(r.Context(), id, slot, productID, price, stock); err != nil {
		http.Error(w, "no se pudo guardar el slot: "+err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/admin/m/"+id, http.StatusSeeOther)
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
