# DECISIONS.md — Bitácora de decisiones

> Registro de decisiones importantes. Cada agente/humano que tome una decisión relevante
> (tecnología, proveedor, diseño, negocio) la anota aquí con **fecha, decisión, razón** y
> alternativas descartadas. Sirve para no re-discutir y para dar contexto a cualquier IA.
> Formato: la más reciente arriba.

---

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
- [ ] **¿Refrigeración en la máquina piloto?** (derivado de vender bebidas — afecta costo/consumo).
- [ ] **Nombre/dominio** definitivo de la empresa/web.
- [ ] **Entidad/llave Bre-B del piloto** — recomendación de Negocio: **Bancolombia** (persona natural, producto dedicado) por sus alertas de correo en tiempo real, que habilitan la conciliación del MVP. Ver [`negocio/bre-b-guia-negocio.md`](./negocio/bre-b-guia-negocio.md).
- [ ] **Agregador Bre-B** para la fase 2 (tras comparar comisiones/onboarding). Estructura y cuestionario de cotización listos en [`negocio/agregadores-bre-b-comparativa.md`](./negocio/agregadores-bre-b-comparativa.md).
- [ ] **Mecanismo de dispensado** definitivo (espiral vs. gravedad) según producto piloto.
