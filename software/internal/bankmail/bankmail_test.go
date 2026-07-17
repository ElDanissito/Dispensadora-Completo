package bankmail

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// textoMuestra es la frase clave del cuerpo (text/plain) de la muestra real,
// tal como queda tras decodificar quoted-printable — con saltos de línea que el
// parser debe colapsar. Ver negocio/muestra-correo-conciliacion.md.
const textoMuestra = `¡Listo! Todo salió bien con tus movimientos Bancolombia: GRABI M001, recibiste
un pago de Nombre Apellido por $2.00 en tu cuenta *0000 conectado a la
llave 1234567890 el 16/07/2026 a las 02:47.`

func TestParseText_Muestra(t *testing.T) {
	mv, err := ParseText(textoMuestra)
	if err != nil {
		t.Fatalf("ParseText devolvió error: %v", err)
	}
	if mv.MachineID != "M001" {
		t.Errorf("MachineID = %q, quería %q", mv.MachineID, "M001")
	}
	if mv.MachineRaw != "GRABI M001" {
		t.Errorf("MachineRaw = %q, quería %q", mv.MachineRaw, "GRABI M001")
	}
	if mv.Payer != "Nombre Apellido" {
		t.Errorf("Payer = %q, quería %q", mv.Payer, "Nombre Apellido")
	}
	if mv.AmountCOP != 2 {
		t.Errorf("AmountCOP = %d, quería 2", mv.AmountCOP)
	}
	if mv.Account != "*0000" {
		t.Errorf("Account = %q, quería %q", mv.Account, "*0000")
	}
	if mv.BreBKey != "1234567890" {
		t.Errorf("BreBKey = %q, quería %q", mv.BreBKey, "1234567890")
	}
	want := time.Date(2026, 7, 16, 2, 47, 0, 0, Bogota)
	if !mv.OccurredAt.Equal(want) {
		t.Errorf("OccurredAt = %v, quería %v", mv.OccurredAt, want)
	}
}

func TestParseEmail_MuestraEML(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "muestra.eml"))
	if err != nil {
		t.Fatalf("no se pudo leer el fixture: %v", err)
	}
	meta, mv, err := ParseEmail(raw)
	if err != nil {
		t.Fatalf("ParseEmail devolvió error: %v", err)
	}
	if mv.MachineID != "M001" {
		t.Errorf("MachineID = %q, quería M001", mv.MachineID)
	}
	if mv.AmountCOP != 2 {
		t.Errorf("AmountCOP = %d, quería 2", mv.AmountCOP)
	}
	if !InAllowlist(meta.FromAddr) {
		t.Errorf("FromAddr %q debería estar en la allowlist", meta.FromAddr)
	}
	if meta.MessageID != "fixture-0001@an.notificacionesbancolombia.com" {
		t.Errorf("MessageID = %q inesperado", meta.MessageID)
	}
	want := time.Date(2026, 7, 16, 2, 47, 0, 0, Bogota)
	if !mv.OccurredAt.Equal(want) {
		t.Errorf("OccurredAt = %v, quería %v (hora del cuerpo, Bogotá)", mv.OccurredAt, want)
	}
}

func TestNormalizeAmount(t *testing.T) {
	casos := []struct {
		in   string
		want int64
	}{
		{"2.00", 2},
		{"1,234.00", 1234},
		{"2347", 2347},
		{"2,347.00", 2347},
		{" 2.347 ", 2347}, // sin decimales: los puntos son miles
		{"$2.00", 2},
	}
	for _, c := range casos {
		got, err := NormalizeAmount(c.in)
		if err != nil {
			t.Errorf("NormalizeAmount(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("NormalizeAmount(%q) = %d, quería %d", c.in, got, c.want)
		}
	}
}

func TestInAllowlist(t *testing.T) {
	if !InAllowlist("alertasynotificaciones@an.notificacionesbancolombia.com") {
		t.Error("el remitente oficial debería estar en la allowlist")
	}
	if InAllowlist("phishing@bre-b-fake.com") {
		t.Error("un remitente desconocido no debe pasar la allowlist")
	}
	if InAllowlist("ALERTASYNOTIFICACIONES@AN.NOTIFICACIONESBANCOLOMBIA.COM") == false {
		t.Error("la comparación de allowlist debe ser insensible a mayúsculas")
	}
}

func TestParseText_SinPatron(t *testing.T) {
	if _, err := ParseText("Un correo cualquiera sin datos de pago."); err != ErrNoMatch {
		t.Errorf("esperaba ErrNoMatch, obtuve %v", err)
	}
}

func TestPayerMatches(t *testing.T) {
	cases := []struct {
		client, payer string
		want          bool
		nota          string
	}{
		{"Nombre Apellido", "Nombre Apellido", true, "match exacto"},
		{"nombre", "Nombre Apellido", true, "subconjunto de un token"},
		{"  JOSÉ   PEÑA ", "Jose Pena Ramirez", true, "tildes/ñ y espacios ignorados"},
		{"Ramirez", "Jose Pena Ramirez", true, "token final contenido"},
		{"Nombre Apellido", "Nombre", false, "cliente pide un token que el pagador no tiene"},
		{"Carlos", "Nombre Apellido", false, "no coincide"},
		{"", "Nombre Apellido", false, "cliente vacío nunca casa"},
		{"Jo", "Jose Pena", false, "solo tokens < 3 no aporta match usable"},
		{"de la", "Jose de la Cruz", false, "solo tokens cortos → sin tokens usables"},
	}
	for _, c := range cases {
		if got := PayerMatches(c.client, c.payer); got != c.want {
			t.Errorf("PayerMatches(%q,%q)=%v, quería %v (%s)", c.client, c.payer, got, c.want, c.nota)
		}
	}
}
