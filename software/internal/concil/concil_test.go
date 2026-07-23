package concil

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"

	"dispensadoras/software/internal/bankmail"
	"dispensadoras/software/internal/imapmail"
	"dispensadoras/software/internal/store"
)

// fakeMailer devuelve siempre los mismos mensajes (aunque se marquen \Seen), para
// poder ejercitar la idempotencia por Message-ID en la base.
type fakeMailer struct {
	msgs []imapmail.RawMessage
	seen []imap.UID
}

func (f *fakeMailer) FetchUnseenFrom(string) ([]imapmail.RawMessage, error) { return f.msgs, nil }
func (f *fakeMailer) MarkSeen(uids ...imap.UID) error {
	f.seen = append(f.seen, uids...)
	return nil
}

// fakeEmitter firma un token de mentira determinista.
type fakeEmitter struct{ calls int }

func (e *fakeEmitter) SignOrder(_ context.Context, o store.Order) (string, int64, error) {
	e.calls++
	return "TOKEN-" + o.Jti, o.PayWindowExpiresAt, nil
}

// rawEmail arma un correo crudo tipo Bancolombia GRABI (QP) con el monto y máquina
// dados, y el Message-ID dado.
func rawEmail(msgID, machine string, amount string) []byte {
	body := fmt.Sprintf("=C2=A1Listo! Todo sali=C3=B3 bien con tus movimientos Bancolombia: %s, recibiste\r\n"+
		"un pago de Nombre Apellido por $%s en tu cuenta *0000 conectado a la\r\n"+
		"llave 1234567890 el 16/07/2026 a las 02:47.\r\n", machine, amount)
	return []byte("From: Alertas y Notificaciones <alertasynotificaciones@an.notificacionesbancolombia.com>\r\n" +
		"To: grabibot@gmail.com\r\n" +
		"Subject: Alertas y Notificaciones\r\n" +
		"Message-ID: <" + msgID + ">\r\n" +
		"Date: Thu, 16 Jul 2026 07:47:25 +0000\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n" +
		"\r\n" + body)
}

func discardLogger() *log.Logger { return log.New(io.Discard, "", 0) }

func setupStore(t *testing.T) (*store.Store, int64) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL no definido; se omite (requiere Postgres de pruebas)")
	}
	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	if err := st.ResetForTest(ctx); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if err := st.CreateMachine(ctx, "M001", "Prueba", "k1"); err != nil {
		t.Fatal(err)
	}
	pid, err := st.CreateProduct(ctx, "Papas")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetSlot(ctx, "M001", 3, pid, 2, 8); err != nil {
		t.Fatal(err)
	}
	// Hora del cuerpo del correo (Bogotá) → epoch.
	at := time.Date(2026, 7, 16, 2, 47, 0, 0, bankmail.Bogota).Unix()
	return st, at
}

func newPendingOrder(t *testing.T, st *store.Store, jti string, uniqueAmount, at int64, payerName string) {
	t.Helper()
	err := st.CreateOrder(context.Background(), store.Order{
		Jti: jti, MachineID: "M001", TotalCOP: uniqueAmount, UniqueAmount: uniqueAmount,
		PayerName: payerName,
		Status:    "pending", Iat: at - 60, Exp: 0, PayWindowExpiresAt: at + 600,
		CreatedAt: at - 60, Items: []store.OrderItem{{Slot: 3, Qty: 1, PriceCOP: uniqueAmount}},
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
}

func TestPoll_MatchAndIdempotency(t *testing.T) {
	st, at := setupStore(t)
	ctx := context.Background()
	newPendingOrder(t, st, "ord_match", 2, at, "Nombre Apellido")

	mailer := &fakeMailer{msgs: []imapmail.RawMessage{{UID: 1, Raw: rawEmail("mov-1@bank", "GRABI M001", "2.00")}}}
	em := &fakeEmitter{}
	svc := New(st, mailer, em, discardLogger(), false)
	svc.now = func() time.Time { return time.Unix(at+30, 0) } // dentro de la ventana

	s1, err := svc.PollOnce(ctx)
	if err != nil {
		t.Fatalf("PollOnce: %v", err)
	}
	if s1.Matched != 1 {
		t.Fatalf("esperaba 1 casado, obtuve %+v", s1)
	}
	if em.calls != 1 {
		t.Errorf("esperaba 1 firma, obtuve %d", em.calls)
	}
	o, err := st.GetOrder(ctx, "ord_match")
	if err != nil {
		t.Fatal(err)
	}
	if o.Status != "paid" {
		t.Errorf("estado = %q, quería paid", o.Status)
	}
	if o.Token != "TOKEN-ord_match" {
		t.Errorf("token = %q inesperado", o.Token)
	}
	if o.BankMessageID != "mov-1@bank" {
		t.Errorf("bank_message_id = %q inesperado", o.BankMessageID)
	}
	// Stock descontado 8 → 7.
	cat, _ := st.Catalog(ctx, "M001")
	if cat[0].Stock != 7 {
		t.Errorf("stock = %d, quería 7 (descuento por venta)", cat[0].Stock)
	}

	// Segundo poll con EL MISMO correo: idempotencia, no re-firma ni re-descuenta.
	s2, err := svc.PollOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s2.Skipped != 1 || s2.Matched != 0 {
		t.Errorf("segundo poll debía saltar por idempotencia, got %+v", s2)
	}
	if em.calls != 1 {
		t.Errorf("no debía re-firmar; firmas=%d", em.calls)
	}
	cat, _ = st.Catalog(ctx, "M001")
	if cat[0].Stock != 7 {
		t.Errorf("stock no debía cambiar en el segundo poll: %d", cat[0].Stock)
	}
}

// Escenario real: el poller estuvo caído, la orden expiró en la base, pero el
// pago entró DENTRO de la ventana. Debe honrarse igual (casa órdenes 'expired').
func TestPoll_MatchExpiredButPaidInWindow(t *testing.T) {
	st, at := setupStore(t)
	ctx := context.Background()
	// Orden creada y con ventana pasada respecto a "ahora", pero el correo (pago)
	// cae dentro de la ventana.
	err := st.CreateOrder(ctx, store.Order{
		Jti: "ord_exp", MachineID: "M001", TotalCOP: 2, UniqueAmount: 2, PayerName: "Nombre Apellido",
		Status: "expired", Iat: at - 60, Exp: 0, PayWindowExpiresAt: at + 600,
		CreatedAt: at - 60, Items: []store.OrderItem{{Slot: 3, Qty: 1, PriceCOP: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	mailer := &fakeMailer{msgs: []imapmail.RawMessage{{UID: 1, Raw: rawEmail("mov-exp@bank", "GRABI M001", "2.00")}}}
	svc := New(st, mailer, &fakeEmitter{}, discardLogger(), false)
	svc.now = func() time.Time { return time.Unix(at+9999, 0) } // procesamos MUY tarde
	s, err := svc.PollOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.Matched != 1 {
		t.Fatalf("una orden expirada pagada en ventana debía casar, got %+v", s)
	}
	o, _ := st.GetOrder(ctx, "ord_exp")
	if o.Status != "paid" || o.Token == "" {
		t.Errorf("orden = %q token=%q; se esperaba paid con token", o.Status, o.Token)
	}
}

func TestPoll_Orphan_MontoNoCasa(t *testing.T) {
	st, at := setupStore(t)
	ctx := context.Background()
	newPendingOrder(t, st, "ord_x", 2, at, "Nombre Apellido") // orden espera 2

	mailer := &fakeMailer{msgs: []imapmail.RawMessage{{UID: 1, Raw: rawEmail("mov-2@bank", "GRABI M001", "3.00")}}} // pagó 3
	svc := New(st, mailer, &fakeEmitter{}, discardLogger(), false)
	svc.now = func() time.Time { return time.Unix(at+30, 0) } // dentro de la ventana
	s, err := svc.PollOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.Orphan != 1 {
		t.Errorf("esperaba 1 huérfano por monto que no casa, got %+v", s)
	}
	o, _ := st.GetOrder(ctx, "ord_x")
	if o.Status != "pending" {
		t.Errorf("la orden no debía pagarse: estado=%q", o.Status)
	}
}

func TestPoll_Discarded_RemitenteNoAutorizado(t *testing.T) {
	st, _ := setupStore(t)
	ctx := context.Background()
	raw := []byte("From: phishing <atacante@bre-b-fake.com>\r\n" +
		"Message-ID: <evil-1@fake>\r\nSubject: Alertas\r\n" +
		"Content-Type: text/plain\r\n\r\nGRABI M001 recibiste un pago de X por $2.00 ...\r\n")
	mailer := &fakeMailer{msgs: []imapmail.RawMessage{{UID: 1, Raw: raw}}}
	svc := New(st, mailer, &fakeEmitter{}, discardLogger(), false)
	s, err := svc.PollOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.Discarded != 1 {
		t.Errorf("esperaba 1 descartado por allowlist, got %+v", s)
	}
}

// ADR-018: el nombre del cliente NO casa con el pagador del correo → huérfano
// (aunque monto y ventana coincidan). La orden sigue pendiente.
func TestPoll_NameMismatch_Orphan(t *testing.T) {
	st, at := setupStore(t)
	ctx := context.Background()
	newPendingOrder(t, st, "ord_nm", 2, at, "Carlos Ramirez") // el correo paga "Nombre Apellido"

	mailer := &fakeMailer{msgs: []imapmail.RawMessage{{UID: 1, Raw: rawEmail("mov-nm@bank", "GRABI M001", "2.00")}}}
	svc := New(st, mailer, &fakeEmitter{}, discardLogger(), false)
	svc.now = func() time.Time { return time.Unix(at+30, 0) }
	s, err := svc.PollOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.Orphan != 1 || s.Matched != 0 {
		t.Fatalf("nombre que no casa debía ser huérfano, got %+v", s)
	}
	o, _ := st.GetOrder(ctx, "ord_nm")
	if o.Status != "pending" {
		t.Errorf("la orden debía seguir pendiente, estado=%q", o.Status)
	}
}

// ADR-018 (regla de seguridad crítica): 2 órdenes con mismo monto+ventana cuyo
// nombre casa el mismo pagador → AMBIGUO. NO se dispensa; ambas quedan 'ambiguous'.
func TestPoll_Ambiguous_NoDispensa(t *testing.T) {
	st, at := setupStore(t)
	ctx := context.Background()
	newPendingOrder(t, st, "ord_a", 2, at, "Nombre")   // ambas casan "Nombre Apellido"
	newPendingOrder(t, st, "ord_b", 2, at, "Apellido") // (tokens contenidos en el pagador)

	em := &fakeEmitter{}
	mailer := &fakeMailer{msgs: []imapmail.RawMessage{{UID: 1, Raw: rawEmail("mov-amb@bank", "GRABI M001", "2.00")}}}
	svc := New(st, mailer, em, discardLogger(), false)
	svc.now = func() time.Time { return time.Unix(at+30, 0) }
	s, err := svc.PollOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.Ambiguous != 1 || s.Matched != 0 {
		t.Fatalf("2 candidatas debían dar AMBIGUO, got %+v", s)
	}
	if em.calls != 0 {
		t.Errorf("no debía firmarse ningún token en caso ambiguo, firmas=%d", em.calls)
	}
	for _, jti := range []string{"ord_a", "ord_b"} {
		o, _ := st.GetOrder(ctx, jti)
		if o.Status != "ambiguous" {
			t.Errorf("orden %s debía quedar 'ambiguous', estado=%q", jti, o.Status)
		}
		if o.Token != "" {
			t.Errorf("orden %s no debía tener token", jti)
		}
	}
	// Stock intacto (8): no se dispensó.
	cat, _ := st.Catalog(ctx, "M001")
	if cat[0].Stock != 8 {
		t.Errorf("stock no debía descontarse en caso ambiguo: %d", cat[0].Stock)
	}
}

// Fallback por monto único (GRABI_MATCH_MODE=unique_amount): NO se filtra por
// nombre; casa por monto exacto aunque la orden no tenga payer_name.
func TestPoll_UniqueAmountFallback(t *testing.T) {
	st, at := setupStore(t)
	ctx := context.Background()
	newPendingOrder(t, st, "ord_u", 2, at, "") // sin nombre

	mailer := &fakeMailer{msgs: []imapmail.RawMessage{{UID: 1, Raw: rawEmail("mov-u@bank", "GRABI M001", "2.00")}}}
	svc := New(st, mailer, &fakeEmitter{}, discardLogger(), true) // fallback
	svc.now = func() time.Time { return time.Unix(at+30, 0) }
	s, err := svc.PollOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.Matched != 1 {
		t.Fatalf("en fallback por monto único debía casar sin nombre, got %+v", s)
	}
}
