// Package bankmail parsea las notificaciones de correo de Bancolombia (marca
// GRABI) para extraer un "movimiento" de pago entrante y poder conciliarlo con
// una orden. Ver negocio/spec-conciliacion-correo.md y negocio/muestra-correo-
// conciliacion.md (estructura real del correo, regex por campo) y DECISIONS.md
// ADR-013 / ADR-014.
//
// Este paquete SOLO extrae y normaliza datos del correo; NO decide pagos, no
// toca la base ni firma tokens. La conciliación (internal/concil) usa su salida.
//
// Seguridad (spec §7): jamás se siguen enlaces ni se ejecuta contenido del
// correo; solo se lee el texto plano. El remitente debe estar en la allowlist.
package bankmail

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Bogota es la zona horaria del piloto (Colombia, UTC-5, sin horario de verano).
// La hora del CUERPO del correo ("a las 02:47") está en esta zona (muestra §Zona
// horaria).
var Bogota = time.FixedZone("COT", -5*3600)

// Allowlist es el conjunto estricto de remitentes que pueden disparar lógica de
// pago (spec §7.1). Cualquier otro correo se descarta. Se compara en minúsculas.
var Allowlist = []string{
	"alertasynotificaciones@an.notificacionesbancolombia.com",
}

// InAllowlist indica si una dirección de correo (solo la parte addr, sin nombre)
// pertenece al remitente oficial verificado de Bancolombia.
func InAllowlist(addr string) bool {
	addr = strings.ToLower(strings.TrimSpace(addr))
	for _, a := range Allowlist {
		if addr == a {
			return true
		}
	}
	return false
}

// Movement es un abono entrante extraído de un correo de alerta.
type Movement struct {
	MachineID  string    // mid normalizado, ej. "M001" (extraído de "GRABI M001")
	MachineRaw string    // texto crudo del punto de venta, ej. "GRABI M001"
	Payer      string    // nombre del pagador (solo auditoría; no se muestra al cliente)
	AmountCOP  int64     // monto normalizado a entero de pesos (ancla del matching)
	Account    string    // cuenta enmascarada, ej. "*0000" (auditoría)
	BreBKey    string    // llave Bre-B destino, ej. "1234567890" (auditoría)
	OccurredAt time.Time // fecha/hora del cuerpo, en zona Bogotá (para la ventana)
	DateRaw    string    // "16/07/2026"
	TimeRaw    string    // "02:47"
	FromAddr   string    // remitente (parte addr), en minúsculas
	MessageID  string    // Message-ID del correo, sin <>, clave de idempotencia
	ReceivedAt time.Time // cabecera Date (fallback de hora si el cuerpo fallara)
}

// Regex sobre el text/plain con saltos de línea normalizados a espacios
// (muestra §Campos a extraer). El banco parte palabras con guiones de
// quoted-printable, por eso se decodifica QP y se colapsan espacios ANTES.
var (
	reMachine = regexp.MustCompile(`movimientos Bancolombia:\s*(GRABI\s+M\d+)`)
	reMid     = regexp.MustCompile(`M\d+`)
	rePayer   = regexp.MustCompile(`recibiste\s+un\s+pago\s+de\s+(.+?)\s+por\s+\$`)
	reAmount  = regexp.MustCompile(`por\s+\$\s*([\d.,]+)`)
	reAccount = regexp.MustCompile(`en\s+tu\s+cuenta\s+(\*\d+)`)
	reKey     = regexp.MustCompile(`conectado\s+a\s+la\s+llave\s+(\d+)`)
	reDate    = regexp.MustCompile(`el\s+(\d{2}/\d{2}/\d{4})`)
	reTime    = regexp.MustCompile(`a\s+las\s+(\d{2}:\d{2})`)
)

// ErrNoMatch indica que el cuerpo no calzó el patrón esperado (posible cambio de
// formato del banco). La conciliación lo trata como PARSE_FALLIDO (spec §6).
var ErrNoMatch = errors.New("bankmail: el cuerpo no coincide con el patrón de alerta de pago")

// Meta son las cabeceras del correo que la conciliación necesita SIEMPRE, aunque
// el cuerpo no se pueda parsear: para la allowlist (FromAddr) y la idempotencia
// (MessageID).
type Meta struct {
	FromAddr   string
	MessageID  string
	ReceivedAt time.Time
}

// ParseEmail parsea un correo crudo (RFC 0000): devuelve las cabeceras (Meta,
// siempre que el correo sea legible) y el movimiento extraído del cuerpo
// text/plain (decodificando quoted-printable). No valida la allowlist (eso es
// política de la conciliación); solo rellena FromAddr para que el llamador decida.
//
// Si el correo es legible pero el CUERPO no calza el patrón, devuelve
// (Meta, nil, ErrNoMatch): el llamador aún tiene el Message-ID para idempotencia y
// puede registrar PARSE_FALLIDO (spec §6).
func ParseEmail(raw []byte) (Meta, *Movement, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return Meta{}, nil, fmt.Errorf("bankmail: leyendo correo: %w", err)
	}

	var meta Meta
	if addr, err := mail.ParseAddress(msg.Header.Get("From")); err == nil {
		meta.FromAddr = strings.ToLower(addr.Address)
	}
	meta.MessageID = strings.Trim(strings.TrimSpace(msg.Header.Get("Message-ID")), "<>")
	if d, err := msg.Header.Date(); err == nil {
		meta.ReceivedAt = d
	}

	text, err := extractPlainText(msg.Header.Get("Content-Type"), msg.Body)
	if err != nil {
		return meta, nil, err
	}

	mv, err := ParseText(text)
	if err != nil {
		return meta, nil, err
	}
	mv.FromAddr = meta.FromAddr
	mv.MessageID = meta.MessageID
	mv.ReceivedAt = meta.ReceivedAt
	// Si el cuerpo no trajera hora usable, se cae a la de recepción (spec §4).
	if mv.OccurredAt.IsZero() && !meta.ReceivedAt.IsZero() {
		mv.OccurredAt = meta.ReceivedAt.In(Bogota)
	}
	return meta, mv, nil
}

// extractPlainText devuelve el text/plain decodificado de un correo. Soporta
// multipart/alternative (elige la parte text/plain, muestra §Formato) y el caso
// degenerado de un cuerpo text/plain directo. Decodifica quoted-printable.
func extractPlainText(contentType string, body io.Reader) (string, error) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		// Sin Content-Type usable: tratar el cuerpo como texto plano tal cual.
		b, _ := io.ReadAll(body)
		return string(b), nil
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		mr := multipart.NewReader(body, params["boundary"])
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", fmt.Errorf("bankmail: leyendo parte MIME: %w", err)
			}
			pmt, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
			if pmt == "text/plain" {
				return decodePart(part.Header.Get("Content-Transfer-Encoding"), part)
			}
		}
		return "", errors.New("bankmail: el correo multipart no trae parte text/plain")
	}

	// Cuerpo simple (no multipart).
	return decodeReader(mediaType, contentType, body)
}

func decodeReader(mediaType, contentType string, body io.Reader) (string, error) {
	// Content-Transfer-Encoding no está en el media type; el llamador (cuerpo
	// simple) rara vez ocurre en este banco, así que asumimos texto tal cual.
	b, err := io.ReadAll(body)
	return string(b), err
}

// decodePart lee una parte MIME aplicando su Content-Transfer-Encoding.
func decodePart(encoding string, r io.Reader) (string, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "quoted-printable":
		b, err := io.ReadAll(quotedprintable.NewReader(r))
		return string(b), err
	default: // 7bit, 8bit, "" → tal cual (utf-8)
		b, err := io.ReadAll(r)
		return string(b), err
	}
}

// ParseText extrae los campos del cuerpo ya decodificado (text/plain). Normaliza
// los saltos de línea a espacios y colapsa espacios (los correos del banco
// parten frases en varias líneas). Devuelve ErrNoMatch si no hay máquina, monto,
// fecha y hora (los campos que anclan la conciliación).
func ParseText(text string) (*Movement, error) {
	flat := collapseSpaces(text)

	mv := &Movement{}
	if m := reMachine.FindStringSubmatch(flat); m != nil {
		mv.MachineRaw = collapseSpaces(m[1])
		mv.MachineID = reMid.FindString(mv.MachineRaw)
	}
	if m := rePayer.FindStringSubmatch(flat); m != nil {
		mv.Payer = strings.TrimSpace(m[1])
	}
	if m := reAccount.FindStringSubmatch(flat); m != nil {
		mv.Account = m[1]
	}
	if m := reKey.FindStringSubmatch(flat); m != nil {
		mv.BreBKey = m[1]
	}
	if m := reDate.FindStringSubmatch(flat); m != nil {
		mv.DateRaw = m[1]
	}
	if m := reTime.FindStringSubmatch(flat); m != nil {
		mv.TimeRaw = m[1]
	}

	amountOK := false
	if m := reAmount.FindStringSubmatch(flat); m != nil {
		if v, err := NormalizeAmount(m[1]); err == nil {
			mv.AmountCOP = v
			amountOK = true
		}
	}

	if mv.DateRaw != "" && mv.TimeRaw != "" {
		if t, err := time.ParseInLocation("02/01/2006 15:04", mv.DateRaw+" "+mv.TimeRaw, Bogota); err == nil {
			mv.OccurredAt = t
		}
	}

	// Anclas mínimas de la conciliación: máquina + monto + fecha + hora.
	if mv.MachineID == "" || !amountOK || mv.DateRaw == "" || mv.TimeRaw == "" {
		return nil, ErrNoMatch
	}
	return mv, nil
}

// collapseSpaces reemplaza cualquier secuencia de espacios en blanco (incluidos
// saltos de línea) por un solo espacio y recorta los extremos.
func collapseSpaces(s string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

// --- Matching por nombre del pagador (ADR-018) ---

// minNameToken es el largo mínimo de un token del nombre para considerarlo (los
// tokens más cortos —"de", "la", iniciales— se ignoran, ADR-018).
const minNameToken = 3

// NormalizeName pasa un nombre a minúsculas, le quita tildes/diéresis/ñ y colapsa
// espacios, para comparar de forma tolerante (ADR-018). Ej. "José Peña" → "jose pena".
func NormalizeName(s string) string {
	s = strings.ToLower(collapseSpaces(s))
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case 'á', 'à', 'ä', 'â', 'ã':
			b.WriteRune('a')
		case 'é', 'è', 'ë', 'ê':
			b.WriteRune('e')
		case 'í', 'ì', 'ï', 'î':
			b.WriteRune('i')
		case 'ó', 'ò', 'ö', 'ô', 'õ':
			b.WriteRune('o')
		case 'ú', 'ù', 'ü', 'û':
			b.WriteRune('u')
		case 'ñ':
			b.WriteRune('n')
		case 'ç':
			b.WriteRune('c')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// PayerMatches indica si el nombre que escribió el cliente (clientInput) casa con
// el nombre del pagador del correo (emailPayer), según la regla de ADR-018:
// normalizar ambos (minúsculas, sin tildes); exigir que **todos** los tokens del
// cliente de ≥ minNameToken caracteres estén **contenidos** en el nombre del
// pagador. Los tokens cortos del cliente se ignoran. Si el cliente no aportó
// ningún token usable (nombre vacío o solo tokens cortos), NO casa (seguridad: un
// nombre en blanco jamás debe casar contra cualquiera).
func PayerMatches(clientInput, emailPayer string) bool {
	client := NormalizeName(clientInput)
	payer := NormalizeName(emailPayer)
	if payer == "" {
		return false
	}
	usable := 0
	for _, tok := range strings.Fields(client) {
		if len([]rune(tok)) < minNameToken {
			continue
		}
		usable++
		if !strings.Contains(payer, tok) {
			return false
		}
	}
	return usable > 0
}

// NormalizeAmount convierte el monto del correo a un entero de pesos colombianos.
//
// Formato REAL del correo GRABI (muestra §Normalización del monto): estilo
// "US" — la coma es separador de miles y el punto precede a 2 decimales que en
// COP siempre son "00" (ej. "$2.00" = 2 pesos, "$1,234.00" = 1234 pesos). Se
// quitan los separadores de miles y se descartan los 2 decimales.
//
// Si el formato del banco cambiara (p. ej. estilo europeo "$2.347,00"), el
// parseo fallaría y la conciliación lo marcaría PARSE_FALLIDO (spec §6): el
// pago no se pierde, se corrige el parser.
func NormalizeAmount(raw string) (int64, error) {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, "$", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", "") // coma = separador de miles
	// Descartar los 2 decimales finales (".00") si están presentes.
	if i := strings.LastIndex(s, "."); i >= 0 && len(s)-i-1 == 2 {
		s = s[:i]
	}
	s = strings.ReplaceAll(s, ".", "") // cualquier punto restante = miles
	if s == "" {
		return 0, fmt.Errorf("bankmail: monto vacío tras normalizar %q", raw)
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bankmail: monto no numérico %q: %w", raw, err)
	}
	return v, nil
}
