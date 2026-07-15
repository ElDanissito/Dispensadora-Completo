// Package store es la capa de datos (SQLite) del backend de dispensadoras.
//
// Modela el esquema de schema.sql: máquinas, catálogo por máquina (slot →
// producto/precio/stock), órdenes y registro de jti usados. Usa el driver
// puro-Go modernc.org/sqlite para no depender de cgo (compila igual en Windows,
// Linux y en el VPS del piloto). Ver ADR-002 (SQLite en piloto → Postgres al escalar).
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Store envuelve la conexión a la base de datos.
type Store struct {
	db *sql.DB
}

// Open abre (o crea) la base SQLite en path y aplica el esquema (idempotente).
func Open(path string) (*Store, error) {
	// _pragma activa foreign_keys por conexión (el PRAGMA del script solo aplica
	// a la conexión que lo ejecuta; con un pool hay que fijarlo por DSN).
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("aplicando esquema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close cierra la base.
func (s *Store) Close() error { return s.db.Close() }

// --- Tipos de dominio ---

// Machine es una máquina física.
type Machine struct {
	ID        string
	Name      string
	Kid       string
	Active    bool
	CreatedAt int64
}

// Product es un producto del catálogo global.
type Product struct {
	ID   int64
	Name string
}

// CatalogRow es una fila del catálogo de una máquina: slot + producto + precio + stock.
type CatalogRow struct {
	Slot        int
	ProductID   int64
	ProductName string
	PriceCOP    int64
	Stock       int
}

// OrderItem es una línea de una orden.
type OrderItem struct {
	Slot     int
	Qty      int
	PriceCOP int64
}

// Order es una orden emitida.
type Order struct {
	Jti       string
	MachineID string
	TotalCOP  int64
	Status    string
	Iat       int64
	Exp       int64
	CreatedAt int64
	Items     []OrderItem
}

// --- Máquinas ---

// CreateMachine inserta una máquina nueva.
func (s *Store) CreateMachine(ctx context.Context, id, name, kid string) error {
	if kid == "" {
		kid = "k1"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO machines (id, name, kid, active, created_at) VALUES (?, ?, ?, 1, ?)`,
		id, name, kid, time.Now().Unix())
	return err
}

// GetMachine devuelve una máquina por id.
func (s *Store) GetMachine(ctx context.Context, id string) (*Machine, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, kid, active, created_at FROM machines WHERE id = ?`, id)
	var m Machine
	var active int
	if err := row.Scan(&m.ID, &m.Name, &m.Kid, &active, &m.CreatedAt); err != nil {
		return nil, err
	}
	m.Active = active == 1
	return &m, nil
}

// ListMachines devuelve todas las máquinas ordenadas por id.
func (s *Store) ListMachines(ctx context.Context) ([]Machine, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, kid, active, created_at FROM machines ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Machine
	for rows.Next() {
		var m Machine
		var active int
		if err := rows.Scan(&m.ID, &m.Name, &m.Kid, &active, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.Active = active == 1
		out = append(out, m)
	}
	return out, rows.Err()
}

// --- Productos ---

// CreateProduct inserta un producto y devuelve su id.
func (s *Store) CreateProduct(ctx context.Context, name string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO products (name, created_at) VALUES (?, ?)`, name, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListProducts devuelve el catálogo global de productos.
func (s *Store) ListProducts(ctx context.Context) ([]Product, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name FROM products ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// --- Catálogo por máquina ---

// SetSlot asigna (o actualiza) un slot de una máquina con producto, precio y stock.
func (s *Store) SetSlot(ctx context.Context, machineID string, slot int, productID int64, priceCOP int64, stock int) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO machine_products (machine_id, slot, product_id, price_cop, stock)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(machine_id, slot) DO UPDATE SET
			product_id = excluded.product_id,
			price_cop  = excluded.price_cop,
			stock      = excluded.stock`,
		machineID, slot, productID, priceCOP, stock)
	return err
}

// Catalog devuelve las filas del catálogo de una máquina (solo con stock ≥ 0),
// ordenadas por slot. Incluye el nombre del producto.
func (s *Store) Catalog(ctx context.Context, machineID string) ([]CatalogRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT mp.slot, mp.product_id, p.name, mp.price_cop, mp.stock
		FROM machine_products mp
		JOIN products p ON p.id = mp.product_id
		WHERE mp.machine_id = ?
		ORDER BY mp.slot`, machineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CatalogRow
	for rows.Next() {
		var r CatalogRow
		if err := rows.Scan(&r.Slot, &r.ProductID, &r.ProductName, &r.PriceCOP, &r.Stock); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- Órdenes ---

// CreateOrder inserta una orden con sus líneas en una transacción.
func (s *Store) CreateOrder(ctx context.Context, o Order) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO orders (jti, machine_id, total_cop, status, iat, exp, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		o.Jti, o.MachineID, o.TotalCOP, o.Status, o.Iat, o.Exp, o.CreatedAt); err != nil {
		return err
	}
	for _, it := range o.Items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO order_items (order_jti, slot, qty, price_cop) VALUES (?, ?, ?, ?)`,
			o.Jti, it.Slot, it.Qty, it.PriceCOP); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListOrders devuelve las últimas `limit` órdenes (más recientes primero).
func (s *Store) ListOrders(ctx context.Context, limit int) ([]Order, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT jti, machine_id, total_cop, status, iat, exp, created_at
		FROM orders ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.Jti, &o.MachineID, &o.TotalCOP, &o.Status, &o.Iat, &o.Exp, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
