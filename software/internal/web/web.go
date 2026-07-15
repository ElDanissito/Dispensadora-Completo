// Package web sirve la página pública por máquina (GET /m/{id}) y un panel de
// administración mínimo (crear máquinas, cargar productos/precios/stock, ver
// órdenes). Front ligero con html/template server-rendered (ADR-002); sin JS
// pesado para que /m/{id} cargue rápido en el celular del cliente.
package web

import (
	"crypto/subtle"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"dispensadoras/software/internal/store"
)

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
}

// New construye el servidor. adminUser/adminPass protegen /admin con Basic Auth.
func New(st *store.Store, adminUser, adminPass string) (*Server, error) {
	// Cada página se compone de base.html + su propia plantilla "content".
	// Parseamos todo junto: base define "base"; cada archivo redefine "content",
	// así que renderizamos clonando y parseando la página concreta bajo demanda.
	base, err := template.New("base").Funcs(funcs).ParseFS(tmplFS, "templates/base.html")
	if err != nil {
		return nil, err
	}
	s := &Server{st: st, tmpl: base, adminUser: adminUser, adminPass: adminPass}
	return s, nil
}

// Routes registra todas las rutas y devuelve el handler raíz.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Pública.
	mux.HandleFunc("GET /m/{id}", s.handleMachinePublic)

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
