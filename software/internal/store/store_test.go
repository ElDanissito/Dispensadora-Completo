package store

import (
	"context"
	"path/filepath"
	"testing"
)

// openTemp abre una base en un archivo temporal del test.
func openTemp(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestMachineAndCatalog(t *testing.T) {
	ctx := context.Background()
	st := openTemp(t)

	if err := st.CreateMachine(ctx, "M001", "Cafetería Cali", "k1"); err != nil {
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
	if err := st.CreateMachine(ctx, "M001", "Demo", "k1"); err != nil {
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
