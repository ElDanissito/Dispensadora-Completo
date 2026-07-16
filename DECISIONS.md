# DECISIONS.md — Bitácora de decisiones

> Registro de decisiones importantes. Cada agente/humano que tome una decisión relevante
> (tecnología, proveedor, diseño, negocio) la anota aquí con **fecha, decisión, razón** y
> alternativas descartadas. Sirve para no re-discutir y para dar contexto a cualquier IA.
> Formato: la más reciente arriba.

---

## 🏁 HITO (2026-07-16) · Ciclo completo funcionando de punta a punta
- **Compra real end-to-end validada:** web genera orden con monto único → cliente paga por **Bre-B** → el servicio de **conciliación por correo** (grabibot) detecta el pago solo → emite el **QR firmado (v2)** → la máquina (ESP32 + GM65) lo **verifica offline**, comprueba anti-reuso **persistente en NVS** y **dispensa**, confirmando la caída con el **sensor E18-D80NK**.
- **Firmware de producción:** `firmware/paso5b-jti-nvs/` (anti-reuso sobrevive apagones, validado en placa). Pendientes menores atados al RTC (poda de `jti`, `NOW` real).
- **Estado del proyecto:** Fase 1–2 (prototipo técnico) **COMPLETA**. Sigue Fase 2→3: **máquina física + unit economics + punto piloto**.

## ADR-016 · Dominio de la web (opciones — pendiente de elegir)
- **Fecha:** 2026-07-16 · **Estado:** PENDIENTE (decidir mañana).
- **Contexto:** la marca es **GRABI** (ADR-013). El dominio se ve en la máquina y al pagar (`dominio/M001`), así que corto y confiable ayuda.
- **Opciones evaluadas por Daniel:**
  - `grabi.com.co` — ~$20/año. Máxima confianza local en Colombia (.com.co es el estándar de negocio).
  - `grabi.net` — precio medio.
  - `grabi.lat` — ~$2/año. Corto, barato, temática Latam (misma familia que `napi.lat`).
  - `grabi.napi.lat` — **$0** (subdominio de `napi.lat`, que Daniel ya posee). Funciona ya, pero más largo y acopla la marca a NAPI.
- **Recomendación:** para el **piloto**, usar **`grabi.napi.lat` (gratis)** y no gastar aún; **o** `grabi.lat` ($2) si se quiere algo más corto/limpio desde ya. Reservar **`grabi.com.co`** para cuando el piloto valide y se escale en Colombia (proteger marca + confianza). Decisión final mañana.

## ADR-015 · Conciliación de pagos: implementación (IMAP, parser, matching, estados)
- **Fecha:** 2026-07-16
- **Autor:** Agente de Software (02).
- **Decisión:** Se implementa el servicio de conciliación de la [spec de Negocio](./negocio/spec-conciliacion-correo.md) con estas elecciones técnicas:
  - **Acceso al correo por IMAP** (no Gmail API) con la librería `github.com/emersion/go-imap/v2`. La spec §4 prefería Gmail API por OAuth/push, pero para el piloto grabibot ya usa **App Password** (ADR-013) y el canal es un **reenvío filtrado** al buzón; IMAP es más simple, sin flujo OAuth y sin dependencia de Google Cloud. El contrato de salida (`orden.pagada`) no cambia, así que migrar a Gmail API/webhook luego (spec §11) no afecta al resto.
  - **Poller** cada ~12 s (configurable, `-concil-interval`) sobre `UNSEEN FROM <remitente oficial>`, con **BODY.PEEK** (no marca leído al traer) y marca `\Seen` **tras** persistir. Reconexión perezosa si Gmail cierra la sesión.
  - **Parser** (`internal/bankmail`) sobre el **`text/plain`** decodificando **quoted-printable** y colapsando espacios; regex por campo de la [muestra](./negocio/muestra-correo-conciliacion.md). **Normalización del monto**: el correo real de Bancolombia usa formato **US** (`$2.00`, `$1,234.00` → coma=miles, punto=2 decimales `.00`); se descartan miles y decimales para casar por **entero de pesos**. Si el banco cambia el formato → `PARSE_FALLIDO` + alerta (el pago no se pierde).
  - **Matching** por **(máquina + monto único + ventana)**, tolerancia 0. `machine_id` se saca del texto `GRABI M00X` del correo (ADR-013/014), así que **no** depende de una-llave-por-máquina y escala. **Desambiguador** `d` ∈ [1,99] elegido para no colisionar con otras órdenes `pending` de la máquina.
  - **Idempotencia** por `Message-ID` en la tabla nueva `bank_movements` (auditoría de todos los abonos: `matched`/`orphan`/`parse_failed`/`discarded`/`conflict`). La transición `pending→paid` es **atómica** y descuenta stock en la misma transacción; solo dispara si la orden seguía `pending` (nunca dos QR por el mismo pago).
  - **Estados de orden** alineados con la spec §3: `pending|paid|dispensed|expired|canceled` + `paid_sim` (pruebas). Columnas nuevas `unique_amount`, `pay_window_expires_at`, `token`, `paid_at`, `bank_message_id` (migración idempotente por `ALTER TABLE` para la base del piloto).
  - **Reemplazo del pago simulado:** el flujo público pasa a `POST /m/{id}/pagar` (orden `pending` + monto único + pantalla de pago con auto-refresh `<meta refresh>`, sin JS, ADR-011bis). `simular-pago` queda **tras el flag `-allow-sim`** (nunca en producción, spec §8).
- **Separación de responsabilidades (spec §0):** la conciliación **solo confirma el pago**; la **firma** del JWT vive en el servidor (llave privada) vía la interfaz `Emitter`. El QR se emite exactamente en el punto donde antes actuaba `simular-pago`.
- **Seguridad:** allowlist estricta del remitente (`alertasynotificaciones@an.notificacionesbancolombia.com`); credenciales (App Password, llaves Bre-B) **solo desde `software/.env`** (git-ignored), jamás en el repo ni en argumentos. El `.eml` real con datos personales **no** se sube; el repo lleva un fixture **anonimizado** para los tests.
- **Verificado:** login IMAP a grabibot OK; parseo del correo real de Bancolombia (todos los campos); tests de matching/idempotencia/huérfano/allowlist y flujo web pago→QR.
- **Pendiente:** valor de la llave Bre-B por máquina en `.env` (`GRABI_BREB_KEY_M001`) para mostrarlo en la pantalla de pago (hoy cae al nombre del punto de venta "GRABI M001"); panel de movimientos/huérfanos; deploy con TLS.

## ADR-014 · Bre-B: una llave por máquina en el piloto; atribución por referencia al escalar
- **Fecha:** 2026-07-14
- **Decisión:** En el piloto, **cada máquina tiene su propia llave Bre-B**. Nombre del punto de venta = **marca + machine_id** → formato **`GRABI M001`**. La llave de M001 ya fue registrada (Bancolombia).
- **Razón:** con 1 llave por máquina, un pago que llega a esa llave es, sin ambigüedad, de esa máquina → conciliación/atribución trivial en el piloto.
- **Límite conocido (no ignorar):** Bre-B **restringe el número de llaves por persona/cuenta**, así que una-llave-por-máquina **NO escala** a muchas máquinas. Confirmar con el banco el máximo de llaves permitidas.
- **Estrategia al escalar:** cuando haya varias máquinas por llave, atribuir por **monto único por orden** y/o **referencia** en la transferencia; a futuro, **QR dinámico Bre-B con referencia** o **webhook de agregador** para atribución exacta. La conciliación (Dept. 04/02) debe casar por `(máquina + monto/referencia)`, no solo por "a qué llave llegó".
- **Seguridad / dónde vive el valor:** los **valores de las llaves Bre-B NO van al repo** (es público). Se guardan en **configuración de la app** (mapa `machine_id → llave` en variables de entorno / secreto). Una llave de cobro no es secreta (solo sirve para *recibir*), pero es un identificador operativo y no se expone sin necesidad.
- **Nota para Dept. 02/04:** el backend necesita un **mapa `machine_id → llave Bre-B`** (config) para saber a qué llave debe llegar el pago de cada máquina y conciliar. M001 ya tiene la suya asignada (valor fuera del repo).

## ADR-013 · Marca: GRABI · correo de negocio: grabibot@gmail.com
- **Fecha:** 2026-07-14
- **Decisión:** La marca es **GRABI** (de "grab it" / *agárralo*). Backronym de respaldo: **G**et **R**eady **A**nd **B**uy **I**nstantly. Tagline: **"GRABI — escanea, paga, agárralo."**
- **Correo del negocio:** **grabibot@gmail.com** (creado para esto). **Es el buzón que vigila la conciliación por correo.**
- **Canal de conciliación del piloto (FUNCIONANDO 2026-07-14):** las alertas de Bancolombia llegan al **correo personal** de Daniel (no se cambia, para no desviar todo su banca al negocio). Un **filtro en Gmail personal reenvía a grabibot** SOLO los correos con asunto **"Alertas y Notificaciones"** que **contienen "GRABI"**. El servicio de conciliación lee **solo grabibot** (App Password, no la contraseña real).
- **Atribución por máquina gratis:** como la llave se llama **"GRABI M00X"**, ese texto viaja en la alerta → el parser atribuye el pago a la máquina por el `GRABI M00X` que aparezca en el correo. Escala sin depender de una llave por máquina.
- **Nota para Dept. 02/04:** el parser debe extraer de ese correo: **monto, remitente, hora y el `GRABI M00X`** (máquina). Casar con la orden por **máquina + monto único** dentro de una ventana de tiempo.
- **Llave Bre-B (registro):** punto de venta = "GRABI"; categoría = **Tiendas y mercados**; subcategoría = **Tiendas de barrio y minimercados**.
- **Pendiente:** verificar disponibilidad de dominio (`grabi.co` / `grabi.com.co` / alternativa) antes de fijar la web pública. La marca no cambia; solo el dominio exacto.
- **Nota de seguridad:** las credenciales del correo NO van al repo; el servicio de conciliación las lee vía variable de entorno / secreto (App Password de Gmail, no la contraseña principal).

## ADR-012 · Modelo de inventario: en el servidor, con recuento en el refill como verdad
- **Fecha:** 2026-07-14
- **Decisión:** El **inventario vive en la web/servidor** (stock por máquina). Se **descuenta con cada compra exitosa** (pago confirmado + QR emitido). En cada **reabastecimiento**, el admin **ingresa el conteo real** de cada producto desde el panel → eso resetea el estimado a la realidad. La máquina **NO** necesita conexión permanente ni cable a un PC para gestionar inventario.
- **Razón:** Encaja con el diseño **offline-first** (la máquina vende sin internet), es simple y barato, y el recuento en refill es el mecanismo correcto para corregir la deriva.
- **Matiz clave (asumido y aceptado):** la web sabe qué **cobró**, no qué **cayó físicamente**. El conteo del servidor es un **estimado que deriva** por: (a) QR pagados pero no escaneados/expirados, y (b) `DISPENSE_FAIL`. El **recuento en el refill es el ancla de verdad** que lo corrige periódicamente. Entre refills el estimado sirve para marcar agotados y evitar sobreventa en la web.
- **Alternativa descartada:** máquina conectada a un PC "siempre sincronizada" → rompe la ventaja offline y añade fricción operativa.
- **Mejora futura (opcional, NO bloquea):** **sync oportunista** — la máquina guarda un **log local** de dispensados y de cada `DISPENSE_FAIL`, y lo **sube cuando tenga wifi** (refill, hotspot del operario, wifi del punto). Da reconciliación exacta y visibilidad de fallos/reembolsos sin requerir conexión para vender. Resuelve la preocupación de "la máquina no puede reportar fallos".

### Funcionalidades a implementar (panel admin — futuro, Dept. 02 + 05)
- Gestión de inventario por máquina desde el panel: ver stock por slot/producto.
- **Descuento automático** de stock ante compra exitosa (integrar con el flujo de orden/pago).
- **Recarga de inventario en refill:** el admin escribe cuánto hay de cada producto → fija el conteo.
- (Futuro) Ingesta del **log de dispensados/fallos** de la máquina vía sync oportunista para reconciliar.
- (Futuro) Alertas de bajo stock y reporte de `DISPENSE_FAIL` para reembolsos.

## ADR-011bis · Frontend web: mantener server-rendered (Go templates), no SPA por ahora
- **Fecha:** 2026-07-14
- **Decisión:** La **página del cliente** sigue en **Go html/template** (server-rendered) + CSS/HTMX ligero. **No** se reescribe a Vue/React/Next.
- **Razón:** Es una página transaccional que el cliente abre en el celular frente a la máquina: prioridad = **carga rápida en 4G, despliegue simple (un binario + Caddy), bajo costo y bajo mantenimiento** para un fundador solo. Un SPA añade bundle de JS y complejidad de build/deploy justo donde no aporta.
- **Cuándo sí un framework:** el **panel admin** (app-like, interactivo, no crítico en latencia) — ahí Vue/React es razonable a futuro, como app aparte contra la API en Go. Si se busca "verse más pro", es tema de **estilo (Tailwind/CSS)**, no de framework.

## ADR-001 · Firma del token: Ed25519 (asimétrica)
- **Fecha:** 2026-07-14
- **Decisión:** Firmar el token de dispensado con **Ed25519**. Servidor tiene la privada; la máquina solo la pública.
- **Razón:** Si abren la máquina y extraen su memoria, **no pueden falsificar QR** (no tienen la privada). El ESP32 verifica en milisegundos (solo 1 verificación por venta). Firma de 64 bytes → QR compacto.
- **Alternativas descartadas:** HMAC-SHA256 (simétrica): la misma llave firma y verifica, así que si se extrae de la máquina se pueden crear QR ilimitados. ECDSA P-256: válida, pero Ed25519 es más simple y con librerías más limpias para micro (Monocypher).

## ADR-002 · Stack de backend: Go + SQLite→Postgres
- **Fecha:** 2026-07-14
- **Decisión:** Backend en **Go**, front ligero (templates/HTMX), **SQLite** en el piloto y **Postgres** al escalar.
- **Razón:** Binario único, barato de correr, ideal para firmar tokens y servir páginas rápidas. SQLite = cero costo y cero fricción para 1 máquina.
- **Alternativas descartadas:** SPA pesada (innecesaria y lenta en celular); Postgres desde el día 1 (sobredimensionado para el piloto).

## ADR-003 · Hardware base: ESP32 + GM65 + RTC DS3231
- **Fecha:** 2026-07-14
- **Decisión:** MCU **ESP32**, lector **GM65** (evaluar GM805/GM810 si hace falta más robustez), reloj **RTC DS3231**.
- **Razón:** Baratos, documentados, suficientes. El RTC permite validar la expiración (`exp`) sin internet.
- **Pendiente:** confirmar que el GM65 lee bien QR desde pantalla de celular en varios equipos.

## ADR-004 · Pagos: Bre-B por fases (correo → QR dinámico → webhook)
- **Fecha:** 2026-07-14
- **Decisión:** Empezar cobrando por **Bre-B** con conciliación por **notificación/correo** (monto único por orden para casar pago↔orden). Evolucionar a **QR dinámico** y luego a **API/webhook de un agregador** (Mono/Cobre/MOVii).
- **Razón:** Arrancar sin integración formal ni costo; migrar a algo escalable cuando el volumen lo pida.
- **Regla:** nunca confiar en el comprobante que muestra el cliente, solo en la notificación real de la cuenta del negocio.

## ADR-008 · Firmware: usar el módulo Ed25519 (SHA-512) de Monocypher, no el núcleo EdDSA (BLAKE2b)
- **Fecha:** 2026-07-14
- **Autor:** Agente de Firmware (03).
- **Decisión:** En la máquina, verificar la firma con `crypto_ed25519_check()` del módulo
  **opcional** `monocypher-ed25519` (EdDSA sobre curve25519 + **SHA-512**, RFC 8032). **No** usar
  `crypto_eddsa_check()` del núcleo de Monocypher, que por defecto usa **BLAKE2b**.
- **Razón:** El servidor firma con `crypto/ed25519` de Go = Ed25519 estándar (SHA-512). El núcleo
  de Monocypher con BLAKE2b **no es compatible** y daría `BAD_SIGNATURE` en tokens legítimos. Es
  un fallo sutil (mismo tamaño de firma, misma curva) que solo se ve al probar contra vectores reales.
- **Evidencia:** PoC en C (`firmware/poc-verificacion/`) contra los vectores oficiales da resultados
  **idénticos** al simulador del backend (02): `token-valido`→`OK`, `token-expirado`→`EXPIRED`,
  `token-firma-mala`→`BAD_SIGNATURE`, y anti-reuso `OK`→`ALREADY_USED`. Con esto la **arquitectura
  cripto queda validada en PC antes de comprar hardware** (objetivo del brief de firmware).
- **Nota para Dept. 02:** cualquier cambio del algoritmo de hash de la firma en el servidor debe
  reflejarse aquí; el par correcto es Go `ed25519` ⇔ Monocypher `crypto_ed25519_*`.

## ADR-009 · Producto piloto: mixto snacks + bebidas
- **Fecha:** 2026-07-14
- **Decisión:** El piloto vende **snacks empacados sellados + bebidas** (latas/botellas).
- **Razón:** Definición de Daniel. Más variedad y ticket que un solo tipo. Snacks empacados = baja fricción sanitaria (vienen sellados con registro del fabricante).
- **Impacto:**
  - **Hardware (01):** se requieren **canales de dos tamaños** (espiral para snack + canal/espiral reforzado para bebida). Mecánica algo más compleja; validar dispensado por tipo.
  - **Legal (07):** productos empacados de fábrica; el operador vende sellado. Confirmar requisitos de manipulación/rotulado para venta en expendedora; evitar productos que exijan cadena de frío estricta si no hay refrigeración en el piloto (definir si la máquina tendrá refrigeración para bebidas).
  - **Finanzas (07):** dos categorías de margen/rotación a modelar.
- **Pendiente derivado:** ¿la máquina piloto lleva **refrigeración** para bebidas? (afecta costo y consumo).

## ADR-010 · Figura jurídica inicial: persona natural
- **Fecha:** 2026-07-14
- **Decisión:** Arrancar el piloto como **persona natural** (RUT + registro mercantil), no SAS.
- **Razón:** Rápido y barato para validar 1 máquina. Se reevalúa pasar a SAS al escalar (protección patrimonial e imagen B2B).
- **Impacto:** habilita abrir cuenta/llave Bre-B de negocio y facturar. Negocio (07) prepara el checklist de trámites en Cali (RUT/DIAN, ICA municipal, facturación electrónica).

## ADR-011 · Comprar hardware mínimo del piloto
- **Fecha:** 2026-07-14
- **Decisión:** Comprar el **BOM mínimo** (ESP32 + GM65 + RTC DS3231 + 1 motor/espiral + sensor + fuente) para desbloquear el frente de Firmware y validar la lectura del QR real.
- **Razón:** La cripto ya está validada en PC (ADR-008); el siguiente cuello de botella es hardware. Lista de compra en `hardware/lista-compra-piloto.md`.

## ADR-006 · Tamaño del token → APROBADO adelgazar a v2 (quitar iss/iat)
- **Fecha:** 2026-07-14 · **Estado:** APROBADA por Daniel (era propuesta)
- **Decisión:** Se adopta el **contrato v2**: se eliminan `iss` e `iat` del payload. Con eso 2–3 items caben bajo el objetivo de ~300 chars del QR. El servidor sigue registrando `iat`/emisor en su BD, solo que no viajan en el token.
- **Acción requerida:** Software (02) actualiza `dsptoken` y **regenera los vectores**; Firmware (03) actualiza la PoC y re-verifica. Ver `especificaciones/contrato-token.md` v2.
- **Nota:** los vectores de prueba v1 quedan **obsoletos**.
- **Estado Software (02) — 2026-07-14:** HECHO. `dsptoken` y el CLI `dsp` migrados a v2 (payload `{mid,jti,exp,items}`, sin `iss`/`iat`, código `BAD_ISSUER` eliminado); tests verdes; vectores regenerados con el **mismo par de llaves** (la pública no cambia). Tamaño medido (jti de 11 chars): **1 item 239 · 2 items 258 chars**, holgado bajo el objetivo de ~300 del §6 (antes v1: 2 items = 318). Falta solo la validación con el GM65 real (Firmware/Hardware).
- **Estado Firmware (03) — 2026-07-14:** HECHO. PoC (`firmware/poc-verificacion/`) migrada a v2: eliminado el paso `iss` y el código `R_BAD_ISSUER`; orden de verificación firma→`mid`→`exp`→`jti` (§5 v2). Re-compilada (MSVC en el PC de Daniel; añadido `run-poc.ps1` para Windows además de `run-poc.sh`) y re-verificada contra los vectores v2 regenerados por 02 → resultados **idénticos** al backend: `token-valido`→`OK`, `token-expirado`→`EXPIRED`, `token-firma-mala`→`BAD_SIGNATURE`, reuso→`ALREADY_USED`. La llave pública `k1` no cambió, así que sigue siendo compatible. Pendiente único: validación óptica con GM65 real (bloqueada por hardware).
- **Contexto original del hallazgo (agente 02):**
- **Autor:** Agente de Software (02).
- **Hallazgo:** Con el contrato v1 (JWT/JSON + Ed25519), medido con `dsp` sobre un `jti`
  realista de 14 chars:
  - 1 item → **299** chars (justo dentro del objetivo de ~300 del §6).
  - 2 items → **318** chars (el propio ejemplo del contrato ya usa 2 items).
  - 3 items → 337 · 5 items → 374.
  - Desglose fijo: header 51 + firma 86 + 2 puntos = 139. Cada item añade ~19 chars.
    El `payload` base (1 item) son 160 chars; pesan `iss:"dispensadoras.co"` (24 chars) e `iat`.
- **Implicación:** un pedido normal (2+ productos) genera un QR más denso (~versión 16–17,
  ECC M, en modo byte). Probablemente legible en pantalla de celular por el GM65, pero hay
  que **confirmarlo con hardware** (checklist del contrato §11).
- **Opciones (para decidir con Dept. 03):**
  1. **Aceptar v1 como está** y limitar nº de items por orden (p. ej. ≤2–3) — cero código extra.
  2. **v2 "JSON adelgazado"** (sigue siendo JWT, cambio menor): quitar `iss` (es constante, la
     máquina ya lo asume) y `iat` (solo auditoría; la máquina solo necesita `exp`). Ahorra
     ~40 chars → 2 items caben < 300 con holgura. Requiere versionar el contrato.
  3. **v2 binario COSE/CBOR** (lo que anticipa el §6): el más compacto, pero más trabajo de
     implementación en el ESP32.
- **Recomendación del agente 02:** validar primero con el GM65 real (opción 1 sin código). Si
  se necesita margen, la opción 2 es barata y suficiente para el piloto; reservar COSE/CBOR
  para cuando los pedidos crezcan. **No se cambia el contrato hasta que Daniel decida.**
- **Postura del agente 03 (Firmware):** de acuerdo con 02. El tamaño del QR es un problema de
  **lectura óptica** (depende del GM65 + brillo de pantalla), no de cómputo: verificar el token
  cuesta lo mismo con 1 o 5 items. Prioridad: **medir con el GM65 real** (checklist §11) antes de
  tocar el contrato. Si hace falta margen, la **opción 2 ("JSON adelgazado")** es trivial en el
  firmware (la máquina ya asume `iss` constante y solo necesita `exp`, no `iat`) y no añade
  dependencias; **COSE/CBOR (opción 3) sí es más código en el ESP32** (parser CBOR + firma sobre
  bytes) — reservarla para cuando el volumen/tamaño lo exija. Mientras tanto, el firmware acepta v1
  sin cambios: la PoC ya verifica el ejemplo de 2 items del contrato correctamente.

## ADR-007 · Ciudad del piloto: Cali
- **Fecha:** 2026-07-14
- **Decisión:** El **piloto** (primera máquina y primer punto) se hará en **Cali**.
- **Razón:** Definición del fundador (Daniel). Fija el contexto local para trámites municipales (**ICA**), prospección de puntos (Dept. 06) y logística (Dept. 05).
- **Impacto:** las llaves/cuentas Bre-B se abren igual a nivel nacional (sin diferencia por ciudad); lo local es tributación municipal y captación de puntos.

## ADR-005 · Contrato del token como fuente de verdad versionada
- **Fecha:** 2026-07-14
- **Decisión:** La interfaz Software↔Firmware vive en `especificaciones/contrato-token.md`, versionada. Cambios ⇒ nueva versión + entrada aquí.
- **Razón:** Evita que los dos frentes se desincronicen trabajando en paralelo.

---

## Decisiones pendientes (por tomar)
- [x] **Producto piloto** → mixto snacks + bebidas (ADR-009).
- [x] **Figura jurídica** → persona natural (ADR-010).
- [x] **Ventana de expiración** del token → 5 min (300 s), configurable en servidor (contrato v2).
- [ ] **¿Refrigeración en la máquina piloto?** (derivado de vender bebidas — afecta costo/consumo). **Recomendación de Negocio: NO refrigerar en el piloto** y surtir bebidas estables a temperatura ambiente (gaseosa PET/lata, agua, jugo UHT, energizantes) → menor CAPEX y consumo, mecánica más simple. Análisis y fuentes en [`negocio/requisitos-sanitarios-piloto.md`](./negocio/requisitos-sanitarios-piloto.md) §4.
- [x] **Nombre** de la marca → **GRABI** (ADR-013).
- [ ] **Dominio** de la web → opciones en ADR-016; recomendación piloto: `grabi.napi.lat` (gratis) o `grabi.lat` ($2). Decidir mañana. 3 propuestas listas (**Piqa**, **Antoja**, **Untoque**) con dominios y método de verificación en [`negocio/propuestas-nombre-dominio.md`](./negocio/propuestas-nombre-dominio.md) (disponibilidad por verificar).
- [x] **Entidad/llave Bre-B del piloto** → **Bancolombia**; llave de **M001** registrada como "GRABI M001" (una llave por máquina, ADR-014). Valor guardado en config, fuera del repo.
- [ ] **Agregador Bre-B** para la fase 2 (tras comparar comisiones/onboarding). Estructura y cuestionario de cotización listos en [`negocio/agregadores-bre-b-comparativa.md`](./negocio/agregadores-bre-b-comparativa.md).
- [ ] **Mecanismo de dispensado** definitivo (espiral vs. gravedad) según producto piloto.
