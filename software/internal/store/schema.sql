-- Esquema del backend de dispensadoras (contrato-token.md v2).
-- Piloto: SQLite (un archivo, cero costo). Migrable a Postgres al escalar (ADR-002).
-- Todas las tablas usan CREATE IF NOT EXISTS para que Open() sea idempotente.

PRAGMA foreign_keys = ON;

-- machines: cada máquina física. `id` es el machine_id (mid) del token.
CREATE TABLE IF NOT EXISTS machines (
  id         TEXT PRIMARY KEY,             -- ej. "M001" (= mid del token)
  name       TEXT NOT NULL,                -- nombre/ubicación legible
  kid        TEXT NOT NULL DEFAULT 'k1',   -- llave con la que se firman sus tokens
  active     INTEGER NOT NULL DEFAULT 1,   -- 1 = operativa
  created_at INTEGER NOT NULL              -- epoch s
);

-- products: catálogo global de productos (nombre reutilizable entre máquinas).
CREATE TABLE IF NOT EXISTS products (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  name       TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

-- machine_products: qué producto ocupa cada slot de cada máquina, con precio y stock.
-- `slot` es el `s` que viaja en el token; la máquina dispensa ese slot.
CREATE TABLE IF NOT EXISTS machine_products (
  machine_id TEXT    NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
  slot       INTEGER NOT NULL,             -- slot físico (el `s` del token)
  product_id INTEGER NOT NULL REFERENCES products(id),
  price_cop  INTEGER NOT NULL,             -- precio en pesos colombianos (entero)
  stock      INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (machine_id, slot)
);

-- orders: una orden = un token que se emitirá tras confirmar el pago.
-- v2: `iat` y el emisor se guardan AQUÍ (no viajan en el token) — ver ADR-006.
CREATE TABLE IF NOT EXISTS orders (
  jti         TEXT PRIMARY KEY,            -- id único de la orden (= jti del token)
  machine_id  TEXT NOT NULL REFERENCES machines(id),
  total_cop   INTEGER NOT NULL,
  status      TEXT NOT NULL,               -- pending | paid | dispensed | expired
  iat         INTEGER NOT NULL,            -- emitido en (auditoría, solo servidor)
  exp         INTEGER NOT NULL,            -- expiración del token
  created_at  INTEGER NOT NULL
);

-- order_items: líneas de cada orden (congela precio y cantidad al momento de la compra).
CREATE TABLE IF NOT EXISTS order_items (
  order_jti  TEXT    NOT NULL REFERENCES orders(jti) ON DELETE CASCADE,
  slot       INTEGER NOT NULL,
  qty        INTEGER NOT NULL,
  price_cop  INTEGER NOT NULL
);

-- used_jti: auditoría de los jti que las máquinas reportan como consumidos.
-- La máquina es la fuente de verdad del anti-reuso (offline); esto es el registro central.
CREATE TABLE IF NOT EXISTS used_jti (
  jti        TEXT PRIMARY KEY,
  machine_id TEXT NOT NULL,
  used_at    INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_orders_machine ON orders(machine_id);
CREATE INDEX IF NOT EXISTS idx_orders_created ON orders(created_at);
