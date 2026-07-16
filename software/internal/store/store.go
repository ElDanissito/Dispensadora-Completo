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
	"strings"
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
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrando esquema: %w", err)
	}
	return &Store{db: db}, nil
}

// migrate añade columnas nuevas a bases ya existentes (schema.sql usa CREATE IF
// NOT EXISTS, que NO altera una tabla ya creada). Cada ALTER es idempotente: si la
// columna ya existe, SQLite devuelve "duplicate column name" y lo ignoramos. Las
// bases nuevas ya traen las columnas por el CREATE TABLE, así que estos ALTER solo
// aplican al piloto en curso (dispensadoras.db). Ver spec §3 (cambio de esquema).
func migrate(db *sql.DB) error {
	alters := []string{
		`ALTER TABLE orders ADD COLUMN unique_amount INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE orders ADD COLUMN pay_window_expires_at INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE orders ADD COLUMN token TEXT`,
		`ALTER TABLE orders ADD COLUMN paid_at INTEGER`,
		`ALTER TABLE orders ADD COLUMN bank_message_id TEXT`,
	}
	for _, a := range alters {
		if _, err := db.Exec(a); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("%q: %w", a, err)
		}
	}
	// Índices que dependen de columnas nuevas: se crean AQUÍ, ya con los ALTER
	// aplicados (idempotentes por IF NOT EXISTS).
	postIndexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_match ON orders(machine_id, status, unique_amount)`,
	}
	for _, ix := range postIndexes {
		if _, err := db.Exec(ix); err != nil {
			return fmt.Errorf("%q: %w", ix, err)
		}
	}
	return nil
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
	Jti                string
	MachineID          string
	TotalCOP           int64
	UniqueAmount       int64  // total_cop + desambiguador (ancla del matching)
	Status             string // pending|paid|dispensed|expired|canceled|paid_sim
	Iat                int64
	Exp                int64
	PayWindowExpiresAt int64  // fin de la ventana de pago (epoch s)
	Token              string // JWS firmado (se rellena al pagar); "" si aún no
	PaidAt             int64  // epoch s de la conciliación; 0 si no pagada
	BankMessageID      string // Message-ID del correo que la pagó
	CreatedAt          int64
	Items              []OrderItem
}

// BankMovement es el registro de auditoría de un abono leído del correo.
type BankMovement struct {
	MessageID   string
	MachineID   string
	AmountCOP   int64
	Payer       string
	Account     string
	BreBKey     string
	OccurredAt  int64
	ProcessedAt int64
	Result      string // matched|orphan|parse_failed|discarded|conflict
	OrderJti    string
	FromAddr    string
}

// Resultados posibles de un movimiento (columna bank_movements.result, spec §5/§6).
const (
	MovMatched     = "matched"
	MovOrphan      = "orphan"       // PAGO_HUERFANO: no casó con ninguna orden
	MovParseFailed = "parse_failed" // el correo no se pudo parsear
	MovDiscarded   = "discarded"    // remitente fuera de allowlist (seguridad)
	MovConflict    = "conflict"     // >1 orden candidata (no debería ocurrir)
)

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
		INSERT INTO orders (jti, machine_id, total_cop, unique_amount, status, iat, exp,
			pay_window_expires_at, token, paid_at, bank_message_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		o.Jti, o.MachineID, o.TotalCOP, o.UniqueAmount, o.Status, o.Iat, o.Exp,
		o.PayWindowExpiresAt, nullStr(o.Token), nullInt(o.PaidAt), nullStr(o.BankMessageID),
		o.CreatedAt); err != nil {
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
	rows, err := s.db.QueryContext(ctx, orderSelectCols+` FROM orders ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *o)
	}
	return out, rows.Err()
}

// orderSelectCols es la lista de columnas de orders en orden fijo, compartida por
// todas las consultas de órdenes (para que scanOrder sea consistente).
const orderSelectCols = `SELECT jti, machine_id, total_cop, unique_amount, status, iat, exp,
	pay_window_expires_at, token, paid_at, bank_message_id, created_at`

// scanRow abstrae *sql.Row y *sql.Rows (ambos tienen Scan).
type scanRow interface{ Scan(dest ...any) error }

// scanOrder lee una fila de orders (columnas de orderSelectCols) a un Order.
func scanOrder(r scanRow) (*Order, error) {
	var o Order
	var token, bankMsg sql.NullString
	var paidAt sql.NullInt64
	if err := r.Scan(&o.Jti, &o.MachineID, &o.TotalCOP, &o.UniqueAmount, &o.Status, &o.Iat,
		&o.Exp, &o.PayWindowExpiresAt, &token, &paidAt, &bankMsg, &o.CreatedAt); err != nil {
		return nil, err
	}
	o.Token = token.String
	o.PaidAt = paidAt.Int64
	o.BankMessageID = bankMsg.String
	return &o, nil
}

// GetOrder devuelve una orden por jti, con sus líneas.
func (s *Store) GetOrder(ctx context.Context, jti string) (*Order, error) {
	o, err := scanOrder(s.db.QueryRowContext(ctx, orderSelectCols+` FROM orders WHERE jti = ?`, jti))
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT slot, qty, price_cop FROM order_items WHERE order_jti = ? ORDER BY slot`, jti)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var it OrderItem
		if err := rows.Scan(&it.Slot, &it.Qty, &it.PriceCOP); err != nil {
			return nil, err
		}
		o.Items = append(o.Items, it)
	}
	return o, rows.Err()
}

// --- Conciliación (spec-conciliacion-correo.md) ---

// PendingUniqueAmounts devuelve el conjunto de `unique_amount` reservados por las
// órdenes PENDIENTE de una máquina. El generador del desambiguador `d` lo usa para
// que el monto a cobrar sea único entre órdenes vivas (spec §2).
func (s *Store) PendingUniqueAmounts(ctx context.Context, machineID string) (map[int64]bool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT unique_amount FROM orders WHERE machine_id = ? AND status = 'pending'`, machineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	set := map[int64]bool{}
	for rows.Next() {
		var a int64
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		set[a] = true
	}
	return set, rows.Err()
}

// FindMatchingPending busca órdenes de `machineID` cuyo `unique_amount` sea
// exactamente `amount` (tolerancia 0) y cuya ventana de pago contenga el instante
// `at` del PAGO: created_at <= at <= pay_window_expires_at (spec §5).
//
// Casa órdenes en estado 'pending' O 'expired': el criterio es cuándo PAGÓ el
// cliente (hora del correo), no cuándo lo procesamos. Así, si el poller estuvo
// caído y la orden expiró en la base pero el pago entró dentro de la ventana, el
// pago se honra igual. Excluye las ya 'paid'/'dispensed'/'canceled'.
// Devuelve todas las candidatas; la conciliación decide según su cardinalidad.
func (s *Store) FindMatchingPending(ctx context.Context, machineID string, amount, at int64) ([]Order, error) {
	rows, err := s.db.QueryContext(ctx, orderSelectCols+`
		FROM orders
		WHERE machine_id = ? AND status IN ('pending','expired') AND unique_amount = ?
		  AND created_at <= ? AND ? <= pay_window_expires_at
		ORDER BY created_at`, machineID, amount, at, at)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Rellenar líneas (para descontar stock al pagar).
	for i := range out {
		items, err := s.orderItems(ctx, out[i].Jti)
		if err != nil {
			return nil, err
		}
		out[i].Items = items
	}
	return out, nil
}

func (s *Store) orderItems(ctx context.Context, jti string) ([]OrderItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT slot, qty, price_cop FROM order_items WHERE order_jti = ? ORDER BY slot`, jti)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []OrderItem
	for rows.Next() {
		var it OrderItem
		if err := rows.Scan(&it.Slot, &it.Qty, &it.PriceCOP); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// MarkOrderPaid ejecuta la transición pending→paid de forma ATÓMICA e IDEMPOTENTE:
// solo actualiza si la orden sigue en 'pending' (WHERE status='pending'). Guarda el
// token firmado, `exp`, `paid_at` y el Message-ID que la pagó, y DESCUENTA stock de
// cada línea (ADR-012). Devuelve dispensed=true si ESTA llamada hizo la transición;
// false si la orden ya no estaba pendiente (p. ej. un segundo correo del mismo pago).
func (s *Store) MarkOrderPaid(ctx context.Context, jti, token string, exp, paidAt int64, messageID string) (bool, error) {
	return s.markPaid(ctx, jti, "paid", token, exp, paidAt, messageID)
}

// MarkOrderPaidSim es como MarkOrderPaid pero deja la orden en 'paid_sim' (atajo
// de pruebas, distinguible de un pago real). Ver spec §8 y CLAUDE.md §4.
func (s *Store) MarkOrderPaidSim(ctx context.Context, jti, token string, exp, paidAt int64, messageID string) (bool, error) {
	return s.markPaid(ctx, jti, "paid_sim", token, exp, paidAt, messageID)
}

// markPaid es la transición común pending→{status}. status debe ser 'paid' o
// 'paid_sim'. Atómica: firma+stock viajan juntos y solo si la orden estaba pending.
func (s *Store) markPaid(ctx context.Context, jti, status, token string, exp, paidAt int64, messageID string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	// Guard status IN ('pending','expired'): una orden expirada aún puede pagarse si
	// el pago entró en ventana (ver FindMatchingPending). Nunca re-transiciona una ya
	// 'paid'/'dispensed'/'canceled' → idempotencia (no emite dos QR ni descuenta 2x).
	res, err := tx.ExecContext(ctx, `
		UPDATE orders SET status = ?, token = ?, exp = ?, paid_at = ?, bank_message_id = ?
		WHERE jti = ? AND status IN ('pending','expired')`,
		status, token, exp, paidAt, messageID, jti)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil // no estaba pendiente: idempotencia (no descuenta stock)
	}

	// Descontar stock de cada slot (clamp a 0 por seguridad).
	rows, err := tx.QueryContext(ctx,
		`SELECT slot, qty FROM order_items WHERE order_jti = ?`, jti)
	if err != nil {
		return false, err
	}
	type li struct{ slot, qty int }
	var lines []li
	for rows.Next() {
		var l li
		if err := rows.Scan(&l.slot, &l.qty); err != nil {
			rows.Close()
			return false, err
		}
		lines = append(lines, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, err
	}
	for _, l := range lines {
		if _, err := tx.ExecContext(ctx, `
			UPDATE machine_products SET stock = MAX(0, stock - ?)
			WHERE slot = ? AND machine_id = (SELECT machine_id FROM orders WHERE jti = ?)`,
			l.qty, l.slot, jti); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// ExpireOrders marca EXPIRADA toda orden PENDIENTE cuya ventana de pago ya venció
// (now > pay_window_expires_at). Libera su desambiguador. Devuelve cuántas expiró.
func (s *Store) ExpireOrders(ctx context.Context, now int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE orders SET status = 'expired'
		WHERE status = 'pending' AND pay_window_expires_at > 0 AND ? > pay_window_expires_at`, now)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// MovementProcessed indica si ya se procesó un correo con ese Message-ID
// (idempotencia, spec §7.2).
func (s *Store) MovementProcessed(ctx context.Context, messageID string) (bool, error) {
	if messageID == "" {
		return false, nil
	}
	var one int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM bank_movements WHERE message_id = ?`, messageID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// RecordMovement persiste el registro de auditoría de un abono. Usa INSERT OR
// IGNORE sobre la PK message_id: si el correo ya estaba registrado, no lo duplica.
func (s *Store) RecordMovement(ctx context.Context, m BankMovement) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO bank_movements
			(message_id, machine_id, amount_cop, payer, account, breb_key, occurred_at,
			 processed_at, result, order_jti, from_addr)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.MessageID, m.MachineID, m.AmountCOP, m.Payer, m.Account, m.BreBKey, m.OccurredAt,
		m.ProcessedAt, m.Result, nullStr(m.OrderJti), m.FromAddr)
	return err
}

// --- helpers de NULL ---

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullInt(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}
