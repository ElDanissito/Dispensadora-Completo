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
-- description/image_path son de PRESENTACIÓN (ui-web-v1 §3.1): la foto se sirve
-- estáticamente desde /uploads y aquí solo se guarda su ruta pública.
CREATE TABLE IF NOT EXISTS products (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  image_path  TEXT NOT NULL DEFAULT '',    -- ruta pública de la foto (ej. /uploads/xxx.jpg); '' = sin imagen
  created_at  INTEGER NOT NULL
);

-- machine_products: qué producto ocupa cada slot de cada máquina, con precio y stock.
-- `slot` es el `s` que viaja en el token; la máquina dispensa ese slot.
-- `wired` = el canal/motor de ese slot está físicamente conectado y SÍ dispensa
-- (ui-web-v1 §4: avisar si se publica un producto en un slot sin motor).
CREATE TABLE IF NOT EXISTS machine_products (
  machine_id TEXT    NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
  slot       INTEGER NOT NULL,             -- slot físico (el `s` del token)
  product_id INTEGER NOT NULL REFERENCES products(id),
  price_cop  INTEGER NOT NULL,             -- precio en pesos colombianos (entero)
  stock      INTEGER NOT NULL DEFAULT 0,
  wired      INTEGER NOT NULL DEFAULT 0,   -- 1 = motor conectado (dispensa); 0 = aún sin cablear
  PRIMARY KEY (machine_id, slot)
);

-- orders: una orden = un token que se emitirá tras confirmar el pago.
-- v2: `iat` y el emisor se guardan AQUÍ (no viajan en el token) — ver ADR-006.
-- Conciliación Bre-B (spec §3): la orden nace `pending`; pasa a `paid` cuando el
-- pago real casa. Match por (máquina + monto exacto + ventana + nombre del
-- pagador) en modo `payer` (ADR-018), o por (máquina + monto único + ventana) en
-- el fallback `unique_amount`. El reloj de 5 min del token (`exp`) ARRANCA al
-- pagar, no al crear la orden.
--   status: pending | paid | dispensed | expired | canceled | ambiguous | paid_sim(pruebas)
--   `ambiguous`: ≥2 órdenes casaron un mismo pago (ADR-018) → NO se dispensa; revisión/soporte.
--   `unique_amount`: en modo payer = total_cop (monto redondo exacto); en fallback = total_cop + d.
--   `payer_name`: nombre que el cliente declaró como quien hará la transferencia (ADR-018; PII mínima).
CREATE TABLE IF NOT EXISTS orders (
  jti                   TEXT PRIMARY KEY,      -- id único de la orden (= jti del token)
  machine_id            TEXT NOT NULL REFERENCES machines(id),
  total_cop             INTEGER NOT NULL,      -- precio base (suma de ítems)
  unique_amount         INTEGER NOT NULL DEFAULT 0, -- monto exacto a cobrar (ancla del matching)
  payer_name            TEXT NOT NULL DEFAULT '',   -- nombre declarado de quien paga (ADR-018)
  status                TEXT NOT NULL,
  iat                   INTEGER NOT NULL,      -- emitido en (auditoría, solo servidor)
  exp                   INTEGER NOT NULL,      -- expiración del token (se fija al pagar)
  pay_window_expires_at INTEGER NOT NULL DEFAULT 0, -- fin de la ventana de pago (epoch s)
  token                 TEXT,                  -- JWS firmado (se rellena al pagar)
  paid_at               INTEGER,               -- epoch s en que se concilió el pago
  bank_message_id       TEXT,                  -- Message-ID del correo que la pagó (auditoría)
  created_at            INTEGER NOT NULL
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

-- bank_movements: cada abono leído del correo (casado, huérfano, fallido o
-- descartado). La PK por `message_id` garantiza IDEMPOTENCIA (spec §7.2): un
-- mismo correo nunca se procesa dos veces. Es también el registro de auditoría
-- contable (spec §7.5, Dept. 07).
CREATE TABLE IF NOT EXISTS bank_movements (
  message_id  TEXT PRIMARY KEY,            -- Message-ID del correo (idempotencia)
  machine_id  TEXT,                        -- mid extraído ("GRABI M001" → "M001")
  amount_cop  INTEGER,                     -- monto normalizado a entero COP
  payer       TEXT,                        -- pagador (auditoría; no decide nada)
  account     TEXT,                        -- cuenta enmascarada (auditoría)
  breb_key    TEXT,                        -- llave Bre-B destino (auditoría)
  occurred_at INTEGER,                     -- fecha/hora del cuerpo (epoch s)
  processed_at INTEGER NOT NULL,           -- cuándo lo procesó la conciliación
  result      TEXT NOT NULL,               -- matched|orphan|parse_failed|discarded|conflict
  order_jti   TEXT,                        -- orden casada (si result=matched)
  from_addr   TEXT                         -- remitente (allowlist / seguridad)
);

CREATE INDEX IF NOT EXISTS idx_orders_machine ON orders(machine_id);
CREATE INDEX IF NOT EXISTS idx_orders_created ON orders(created_at);
-- Los índices que dependen de columnas NUEVAS (unique_amount) se crean en migrate()
-- DESPUÉS del ALTER TABLE, porque en una base preexistente esa columna aún no
-- existe cuando se ejecuta este script.
