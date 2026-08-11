package web

// Escáner de QR en el celular del cliente (GET /scan, ADR-025).
//
// Es una CAPA NUEVA: no toca el contrato del token, la conciliación ni el flujo
// /m/{id}. Solo resuelve el paso previo — llevar al cliente de la calcomanía de
// la máquina a su tienda — sin depender de la app de cámara del sistema.
//
// La decodificación ocurre en el navegador (jsQR autohospedado en
// /static/vendor/): el vídeo NUNCA sale del dispositivo, el servidor no recibe
// ni un frame.
//
// Seguridad (CLAUDE.md §4, "nunca confiar en lo que trae el cliente"): el
// contenido de un QR es texto que cualquiera puede imprimir y pegar sobre la
// máquina. La página NUNCA navega al URL que trae el QR: extrae el id, lo valida
// contra machineIDPattern y arma ella misma una ruta RELATIVA /m/{id}. Un QR con
// destino externo (phishing) no puede sacar al cliente del sitio.

import (
	"net/http"
	"regexp"
	"strings"
)

// machineIDPattern es la forma de un machine_id válido ("M001", "M0012"…).
// Es la ÚNICA definición: el handler la inyecta en la plantilla y el JS la usa
// tal cual (`new RegExp`), así el escáner del navegador y las pruebas en Go no
// pueden desincronizarse. La sintaxis es la común a Go y JavaScript.
const machineIDPattern = `^M\d{3,}$`

// machineIDRe compila machineIDPattern para validar en el servidor.
var machineIDRe = regexp.MustCompile(machineIDPattern)

// maxIDEco es cuánto del id escrito se le devuelve al usuario en el formulario
// cuando no valida. Sin tope, un query string enorme se repintaría entero.
const maxIDEco = 32

// scanData es lo que recibe scan.html.
type scanData struct {
	MachineIDPattern string // regex compartida con el JS
	Err              string // error del formulario manual ("" ⇒ sin error)
	Typed            string // lo que el usuario escribió, para no perderlo
}

// handleScan sirve la página del escáner y, de paso, atiende el fallback manual
// (`?m=M001`) para que funcione SIN JavaScript: el mismo formulario que el JS
// intercepta se envía al servidor, que valida igual y redirige.
//
// No consulta la base de datos: si el id no existe, /m/{id} ya muestra "máquina
// no encontrada" (esa página es la que sabe de máquinas), así que el escáner
// sigue en pie aunque la DB esté caída.
func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	d := scanData{MachineIDPattern: machineIDPattern}

	if raw := r.URL.Query().Get("m"); strings.TrimSpace(raw) != "" {
		id := normalizeMachineID(raw)
		if machineIDRe.MatchString(id) {
			// El destino lo arma el servidor a partir de un id ya validado
			// (sin barras ni esquema): no hay forma de colar un redirect
			// abierto por este parámetro.
			http.Redirect(w, r, "/m/"+id, http.StatusSeeOther)
			return
		}
		d.Err = "Ese ID no es válido. Tiene la forma M001: una M y al menos 3 números."
		d.Typed = recortar(strings.TrimSpace(raw), maxIDEco)
	}

	s.render(w, "scan.html", page{
		Title: "Escanear la máquina · GRABI",
		Desc:  "Apunta la cámara al QR de la máquina GRABI y entra a su tienda para pagar con Bre-B.",
		Path:  "/scan",
		Data:  d,
	})
}

// normalizeMachineID pone el id en la forma canónica del catálogo ("m 001" →
// "M001"): mayúsculas y sin espacios. Los machine_id de la DB son mayúsculas y
// Postgres compara distinguiendo caja, así que sin esto "m001" no encontraría
// nada.
func normalizeMachineID(raw string) string {
	return strings.ToUpper(strings.Join(strings.Fields(raw), ""))
}

// recortar limita una cadena a n runas (no bytes: cortar a la mitad de un
// carácter multibyte produciría basura en el HTML).
func recortar(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n])
}
