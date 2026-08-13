package web

// Kit físico por máquina: el QR que se pega en el frente y las calcomanías, para
// que el admin no tenga que fabricarlos a mano (pendiente que dejó ADR-025).
//
// Es capa nueva de admin: no toca el contrato del token, la conciliación ni el
// flujo público /m/{id}. Todo se genera al vuelo; nada se guarda en disco.

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"

	"dispensadoras/software/internal/kit"
)

// Tamaños del QR servido. El mínimo evita QR ilegibles y el máximo evita que
// alguien con sesión pida un PNG gigantesco por accidente.
const (
	qrSizeDefault = 512
	qrSizeMin     = 128
	qrSizeMax     = 2048
)

// authAsset protege los archivos del kit. A diferencia de auth (que redirige al
// login por tratarse de páginas), aquí responde 401: quien pide un .svg/.png/.zip
// espera el archivo, y un 303 al login le devolvería HTML disfrazado de imagen.
func (s *Server) authAsset(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil || !s.validSession(c.Value) {
			http.Error(w, "no autorizado", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// qrSize lee el parámetro opcional ?size= y lo acota.
func qrSize(r *http.Request) int {
	n, err := strconv.Atoi(r.URL.Query().Get("size"))
	if err != nil {
		return qrSizeDefault
	}
	return min(max(n, qrSizeMin), qrSizeMax)
}

// machineKit resuelve la máquina de la ruta y la traduce a los datos del kit.
// Devuelve false (y ya respondió) si la máquina no existe.
func (s *Server) machineKit(w http.ResponseWriter, r *http.Request) (kit.Machine, bool) {
	id := r.PathValue("id")
	m, err := s.st.GetMachine(r.Context(), id)
	if err != nil {
		http.Error(w, "máquina no encontrada: "+id, http.StatusNotFound)
		return kit.Machine{}, false
	}
	// El modelo no tiene columna de ciudad: hoy el "nombre" de la máquina es su
	// punto/ubicación, y es lo que se imprime en la placa.
	return kit.Machine{ID: m.ID, Place: m.Name, Site: s.site}, true
}

// handleMachineQRSVG sirve el QR de la máquina en vectorial (para imprenta).
func (s *Server) handleMachineQRSVG(w http.ResponseWriter, r *http.Request) {
	m, ok := s.machineKit(w, r)
	if !ok {
		return
	}
	svg, err := kit.SVG(m.URL(), qrSize(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=60")
	_, _ = w.Write(svg)
}

// handleMachineQRPNG sirve el QR de la máquina rasterizado (para digital).
func (s *Server) handleMachineQRPNG(w http.ResponseWriter, r *http.Request) {
	m, ok := s.machineKit(w, r)
	if !ok {
		return
	}
	pngBytes, err := kit.PNG(m.URL(), qrSize(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=60")
	_, _ = w.Write(pngBytes)
}

// handleMachineKitZip arma el kit completo de la máquina en memoria y lo
// descarga. Se construye entero antes de escribir para poder devolver un 500
// limpio si algo falla (con streaming ya habríamos enviado un ZIP a medias).
func (s *Server) handleMachineKitZip(w http.ResponseWriter, r *http.Request) {
	m, ok := s.machineKit(w, r)
	if !ok {
		return
	}
	var buf bytes.Buffer
	if err := m.WriteZip(&buf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.sirveDescarga(w, buf.Bytes(), "application/zip", fmt.Sprintf("grabi-%s-kit.zip", m.ID))
}

// handleMachineKitImposicion sirve la hoja de imposición: las seis piezas de la
// máquina en un pliego 1:1 con las guías de kiss-cut, lista para la imprenta de
// vinilo (identidad-visual-v1 §8).
func (s *Server) handleMachineKitImposicion(w http.ResponseWriter, r *http.Request) {
	m, ok := s.machineKit(w, r)
	if !ok {
		return
	}
	pliego, err := m.Imposicion()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.sirveDescarga(w, pliego, "application/pdf", fmt.Sprintf("grabi-%s-imposicion.pdf", m.ID))
}

// sirveDescarga manda un archivo ya armado como descarga. Sin
// Content-Disposition el navegador abriría el PDF en una pestaña en vez de
// bajarlo, y lo que hay que hacer con este archivo es reenviarlo a la imprenta.
func (s *Server) sirveDescarga(w http.ResponseWriter, body []byte, tipo, nombre string) {
	w.Header().Set("Content-Type", tipo)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, nombre))
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(body)
}
