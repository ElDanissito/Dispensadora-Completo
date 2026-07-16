// Package concil es el servicio de conciliación de pagos Bre-B por correo
// (spec: negocio/spec-conciliacion-correo.md). Lee el buzón de grabibot, extrae
// cada abono con internal/bankmail, y lo casa con una orden PENDIENTE por
// (máquina + monto único + ventana de tiempo). Al casar, dispara la emisión del
// token/QR (Emitter) y la transición atómica pending→paid con descuento de stock.
//
// Este módulo SOLO confirma el pago y dispara la emisión; la firma del JWT vive en
// el Emitter (que la implementa el servidor con la llave privada). El contrato de
// salida es la transición `orden.pagada` (spec §4).
//
// Idempotencia (spec §7.2): cada correo se registra por su Message-ID en
// bank_movements; un mismo movimiento nunca marca dos veces ni emite dos QR.
package concil

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/emersion/go-imap/v2"

	"dispensadoras/software/internal/bankmail"
	"dispensadoras/software/internal/imapmail"
	"dispensadoras/software/internal/store"
)

// Mailer abstrae el acceso al buzón (para poder testear sin IMAP real).
// *imapmail.Client lo satisface directamente.
type Mailer interface {
	FetchUnseenFrom(from string) ([]imapmail.RawMessage, error)
	MarkSeen(uids ...imap.UID) error
}

// Emitter firma el token de una orden ya conciliada. Devuelve el JWS y su `exp`
// (epoch s). NO toca la base: la conciliación aplica la transición atómica con
// store.MarkOrderPaid (que además descuenta stock). Separa la firma (llave
// privada, en el servidor) de la confirmación del pago (este módulo), spec §0.
type Emitter interface {
	SignOrder(ctx context.Context, o store.Order) (token string, exp int64, err error)
}

// Service concilia pagos. Se construye con New y se corre con Run (bucle) o
// PollOnce (un ciclo, útil en pruebas y CLI).
type Service struct {
	st      *store.Store
	mailer  Mailer
	emitter Emitter
	sender  string
	log     *log.Logger
	now     func() time.Time
}

// New construye el servicio. `sender` es el remitente oficial a buscar (allowlist,
// spec §7.1). Si logger es nil se usa el estándar.
func New(st *store.Store, mailer Mailer, emitter Emitter, logger *log.Logger) *Service {
	if logger == nil {
		logger = log.Default()
	}
	return &Service{
		st:      st,
		mailer:  mailer,
		emitter: emitter,
		sender:  bankmail.Allowlist[0],
		log:     logger,
		now:     time.Now,
	}
}

// Stats resume el resultado de un ciclo de poll.
type Stats struct {
	Fetched     int
	Matched     int
	Orphan      int
	ParseFailed int
	Discarded   int
	Conflict    int
	Skipped     int // ya procesados (idempotencia)
	Expired     int64
}

// Run corre el bucle de conciliación hasta que ctx se cancele, haciendo poll cada
// `interval`. Los errores de un ciclo se registran y NO detienen el bucle (un fallo
// transitorio de red no debe tumbar el servicio).
func (s *Service) Run(ctx context.Context, interval time.Duration) {
	s.log.Printf("conciliación: iniciada (poll cada %s, remitente %s)", interval, s.sender)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if st, err := s.PollOnce(ctx); err != nil {
			s.log.Printf("conciliación: error en ciclo: %v", err)
		} else if st.Fetched > 0 || st.Matched > 0 || st.Expired > 0 {
			s.log.Printf("conciliación: %d correos (casados=%d huérfanos=%d parse_fallido=%d descartados=%d conflicto=%d) expiradas=%d",
				st.Fetched, st.Matched, st.Orphan, st.ParseFailed, st.Discarded, st.Conflict, st.Expired)
		}
		select {
		case <-ctx.Done():
			s.log.Printf("conciliación: detenida")
			return
		case <-t.C:
		}
	}
}

// PollOnce ejecuta un ciclo: barrido de expiración + lectura y conciliación de los
// correos NO leídos del remitente. Sigue el pseudocódigo de la spec §5.
func (s *Service) PollOnce(ctx context.Context) (Stats, error) {
	var st Stats

	// Barrido de expiración (spec §5, bloque APARTE): libera órdenes vencidas.
	if n, err := s.st.ExpireOrders(ctx, s.now().Unix()); err != nil {
		return st, fmt.Errorf("barrido de expiración: %w", err)
	} else {
		st.Expired = n
	}

	msgs, err := s.mailer.FetchUnseenFrom(s.sender)
	if err != nil {
		return st, err
	}
	st.Fetched = len(msgs)

	for _, m := range msgs {
		if err := s.processOne(ctx, m, &st); err != nil {
			// Un correo problemático no debe abortar el ciclo; se registra y sigue.
			s.log.Printf("conciliación: correo UID %d: %v", m.UID, err)
			continue
		}
		// Marcar \Seen SOLO tras procesar y persistir (idempotencia por Message-ID
		// es el respaldo duro; \Seen evita volver a traerlo).
		if err := s.mailer.MarkSeen(m.UID); err != nil {
			s.log.Printf("conciliación: no se pudo marcar \\Seen UID %d: %v", m.UID, err)
		}
	}
	return st, nil
}

func (s *Service) processOne(ctx context.Context, m imapmail.RawMessage, st *Stats) error {
	meta, mv, perr := bankmail.ParseEmail(m.Raw)

	// Message-ID: clave de idempotencia. Si el correo no trae uno, se sintetiza uno
	// estable a partir del contenido (respaldo; \Seen ya evita el re-fetch).
	msgID := meta.MessageID
	if msgID == "" {
		sum := sha256.Sum256(m.Raw)
		msgID = "nomsgid:" + hex.EncodeToString(sum[:8])
	}

	// Idempotencia (spec §7.2): ¿ya lo procesamos?
	done, err := s.st.MovementProcessed(ctx, msgID)
	if err != nil {
		return fmt.Errorf("chequeo idempotencia: %w", err)
	}
	if done {
		st.Skipped++
		return nil
	}

	rec := store.BankMovement{
		MessageID:   msgID,
		FromAddr:    meta.FromAddr,
		ProcessedAt: s.now().Unix(),
	}

	// Allowlist estricta (spec §7.1): remitente fuera de la lista → descartar.
	if !bankmail.InAllowlist(meta.FromAddr) {
		rec.Result = store.MovDiscarded
		st.Discarded++
		s.log.Printf("conciliación: SEGURIDAD correo de remitente NO autorizado %q descartado (msgid=%s)", meta.FromAddr, msgID)
		return s.st.RecordMovement(ctx, rec)
	}

	// Parseo del cuerpo.
	if perr != nil || mv == nil {
		rec.Result = store.MovParseFailed
		st.ParseFailed++
		s.log.Printf("conciliación: PARSE_FALLIDO (posible cambio de formato del banco) msgid=%s: %v", msgID, perr)
		return s.st.RecordMovement(ctx, rec)
	}

	// Datos del abono para auditoría.
	rec.MachineID = mv.MachineID
	rec.AmountCOP = mv.AmountCOP
	rec.Payer = mv.Payer
	rec.Account = mv.Account
	rec.BreBKey = mv.BreBKey
	rec.OccurredAt = mv.OccurredAt.Unix()

	// Matching por (máquina + monto único + ventana), spec §5.
	// Ancla temporal: la hora de RECEPCIÓN del correo (cabecera Date, RFC, fiable)
	// en vez de la del cuerpo ("a las 02:47"), que es frágil de parsear (formato
	// 12h/24h del banco). Si no hubiera Date, se cae a la del cuerpo.
	at := mv.ReceivedAt.Unix()
	if mv.ReceivedAt.IsZero() {
		at = mv.OccurredAt.Unix()
	}
	candidatas, err := s.st.FindMatchingPending(ctx, mv.MachineID, mv.AmountCOP, at)
	if err != nil {
		return fmt.Errorf("buscando orden candidata: %w", err)
	}

	switch len(candidatas) {
	case 1:
		o := candidatas[0]
		token, exp, err := s.emitter.SignOrder(ctx, o)
		if err != nil {
			return fmt.Errorf("firmando token de orden %s: %w", o.Jti, err)
		}
		dispensed, err := s.st.MarkOrderPaid(ctx, o.Jti, token, exp, s.now().Unix(), msgID)
		if err != nil {
			return fmt.Errorf("marcando pagada la orden %s: %w", o.Jti, err)
		}
		if !dispensed {
			// La orden dejó de estar pendiente entre la búsqueda y ahora
			// (carrera improbable con un solo poller): trátalo como huérfano.
			rec.Result = store.MovOrphan
			st.Orphan++
			s.log.Printf("conciliación: orden %s ya no estaba pendiente; abono %d → huérfano", o.Jti, mv.AmountCOP)
			return s.st.RecordMovement(ctx, rec)
		}
		rec.Result = store.MovMatched
		rec.OrderJti = o.Jti
		st.Matched++
		s.log.Printf("conciliación: PAGADA orden %s (máquina %s, $%d) por correo %s", o.Jti, mv.MachineID, mv.AmountCOP, msgID)
		return s.st.RecordMovement(ctx, rec)

	case 0:
		rec.Result = store.MovOrphan
		st.Orphan++
		s.log.Printf("conciliación: PAGO_HUERFANO máquina=%s monto=%d hora=%s (revisión/soporte)", mv.MachineID, mv.AmountCOP, mv.OccurredAt.Format(time.RFC3339))
		return s.st.RecordMovement(ctx, rec)

	default:
		rec.Result = store.MovConflict
		st.Conflict++
		s.log.Printf("conciliación: CONFLICTO %d órdenes casan máquina=%s monto=%d (revisión manual)", len(candidatas), mv.MachineID, mv.AmountCOP)
		return s.st.RecordMovement(ctx, rec)
	}
}
