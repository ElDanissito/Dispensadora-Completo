package store

import (
	"context"
	"os"
	"testing"
	"time"
)

// openTemp abre la base Postgres de pruebas (TEST_DATABASE_URL) y la limpia. Si
// la variable no está definida, se omite el test (no hay Postgres disponible).
func openTemp(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL no definido; se omite (requiere Postgres de pruebas)")
	}
	st, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := st.ResetForTest(context.Background()); err != nil {
		t.Fatalf("reset: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestMachineAndCatalog(t *testing.T) {
	ctx := context.Background()
	st := openTemp(t)

	if err := st.CreateMachine(ctx, "M001", "Cafetería Cali", "k1", 4); err != nil {
		t.Fatalf("CreateMachine: %v", err)
	}
	m, err := st.GetMachine(ctx, "M001")
	if err != nil {
		t.Fatalf("GetMachine: %v", err)
	}
	if m.Name != "Cafetería Cali" || m.Kid != "k1" || !m.Active {
		t.Fatalf("máquina inesperada: %+v", m)
	}

	pid, err := st.CreateProduct(ctx, "Papas")
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	if err := st.SetSlot(ctx, "M001", 3, pid, 3000, 5); err != nil {
		t.Fatalf("SetSlot: %v", err)
	}
	// Upsert: mismo slot cambia precio/stock, no duplica.
	if err := st.SetSlot(ctx, "M001", 3, pid, 3200, 8); err != nil {
		t.Fatalf("SetSlot upsert: %v", err)
	}
	cat, err := st.Catalog(ctx, "M001")
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	if len(cat) != 1 {
		t.Fatalf("esperaba 1 fila de catálogo, obtuve %d", len(cat))
	}
	got := cat[0]
	if got.Slot != 3 || got.ProductName != "Papas" || got.PriceCOP != 3200 || got.Stock != 8 {
		t.Fatalf("fila de catálogo inesperada: %+v", got)
	}
}

func TestCreateAndListOrders(t *testing.T) {
	ctx := context.Background()
	st := openTemp(t)
	if err := st.CreateMachine(ctx, "M001", "Demo", "k1", 4); err != nil {
		t.Fatal(err)
	}

	o := Order{
		Jti: "ord_abc123", MachineID: "M001", TotalCOP: 6500, Status: "paid",
		Iat: 1000, Exp: 1300, CreatedAt: 1000,
		Items: []OrderItem{{Slot: 3, Qty: 1, PriceCOP: 3000}, {Slot: 5, Qty: 1, PriceCOP: 3500}},
	}
	if err := st.CreateOrder(ctx, o); err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	orders, err := st.ListOrders(ctx, 10)
	if err != nil {
		t.Fatalf("ListOrders: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("esperaba 1 orden, obtuve %d", len(orders))
	}
	if orders[0].Jti != "ord_abc123" || orders[0].TotalCOP != 6500 || orders[0].Status != "paid" {
		t.Fatalf("orden inesperada: %+v", orders[0])
	}

	// jti duplicado (PRIMARY KEY) debe fallar → protege el anti-reuso a nivel de orden.
	if err := st.CreateOrder(ctx, o); err == nil {
		t.Fatal("esperaba error por jti duplicado, no hubo")
	}
}

// TestRefreshOrderToken cubre la re-emisión del QR vencido: renueva token/exp de
// una orden PAGADA conservando el jti, y no toca las que no están pagadas.
func TestRefreshOrderToken(t *testing.T) {
	ctx := context.Background()
	st := openTemp(t)
	if err := st.CreateMachine(ctx, "M001", "Demo", "k1", 4); err != nil {
		t.Fatal(err)
	}
	base := Order{
		MachineID: "M001", TotalCOP: 3000, Iat: 1000, Exp: 1300, CreatedAt: 1000,
		Items: []OrderItem{{Slot: 1, Qty: 1, PriceCOP: 3000}},
	}
	paid := base
	paid.Jti, paid.Status, paid.Token = "ord_paid01", "paid", "jws.viejo"
	if err := st.CreateOrder(ctx, paid); err != nil {
		t.Fatal(err)
	}
	ok, err := st.RefreshOrderToken(ctx, paid.Jti, "jws.nuevo", 9999)
	if err != nil || !ok {
		t.Fatalf("RefreshOrderToken en orden pagada: ok=%v err=%v", ok, err)
	}
	got, err := st.GetOrder(ctx, paid.Jti)
	if err != nil {
		t.Fatal(err)
	}
	if got.Token != "jws.nuevo" || got.Exp != 9999 || got.Jti != paid.Jti || got.Status != "paid" {
		t.Fatalf("orden re-emitida inesperada: %+v", got)
	}

	// Pendiente (nadie pagó) y dispensada (el producto ya salió): no se re-emiten.
	for _, status := range []string{"pending", "dispensed"} {
		o := base
		o.Jti, o.Status = "ord_"+status, status
		if err := st.CreateOrder(ctx, o); err != nil {
			t.Fatal(err)
		}
		if ok, err := st.RefreshOrderToken(ctx, o.Jti, "jws.no", 8888); err != nil || ok {
			t.Fatalf("estado %s: esperaba ok=false, obtuve ok=%v err=%v", status, ok, err)
		}
	}
}

// Los interesados de la landing se guardan y se listan con los más recientes
// primero (landing-v1 §4/§5). No se relacionan con órdenes ni máquinas.
func TestCreateAndListLeads(t *testing.T) {
	ctx := context.Background()
	st := openTemp(t)

	id, err := st.CreateLead(ctx, Lead{
		Name: "Ana Ruiz", SpaceType: "conjunto", City: "Cali", WhatsApp: "3001234567",
	})
	if err != nil {
		t.Fatalf("CreateLead: %v", err)
	}
	if id == 0 {
		t.Fatal("CreateLead devolvió id 0")
	}
	// Un segundo lead, más reciente y con origen explícito.
	if _, err := st.CreateLead(ctx, Lead{
		Name: "Beto Gómez", SpaceType: "oficina", City: "Palmira", WhatsApp: "3109876543",
		Source: "referido", CreatedAt: time.Now().Unix() + 60,
	}); err != nil {
		t.Fatalf("CreateLead 2: %v", err)
	}

	leads, err := st.ListLeads(ctx, 0)
	if err != nil {
		t.Fatalf("ListLeads: %v", err)
	}
	if len(leads) != 2 {
		t.Fatalf("esperaba 2 leads, obtuve %d", len(leads))
	}
	if leads[0].Name != "Beto Gómez" || leads[0].Source != "referido" {
		t.Errorf("el más reciente debe ir primero: %+v", leads[0])
	}
	primero := leads[1]
	if primero.ID != id || primero.SpaceType != "conjunto" || primero.City != "Cali" ||
		primero.WhatsApp != "3001234567" {
		t.Errorf("lead inesperado: %+v", primero)
	}
	// Source vacío ⇒ "landing"; CreatedAt en 0 ⇒ se sella al insertar.
	if primero.Source != "landing" || primero.CreatedAt == 0 {
		t.Errorf("valores por defecto no aplicados: %+v", primero)
	}
}
