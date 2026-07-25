# 02 · Software / Web

**Responsable:** Daniel + agente de IA de desarrollo (backend Go, frontend, DevOps).
**Misión:** Construir la plataforma web que muestra productos por máquina, cobra el pago y **emite el QR firmado** que la máquina valida offline. Es el corazón del sistema.

---

## 1. De qué se encarga

- Página pública de venta por máquina: `dominio.com/<ID>` (o `/m/<ID>`).
- Catálogo y stock por máquina (panel de administración).
- Orquestación del pago (ver [Pagos](./04-pagos.md)).
- **Generación y firma del token de dispensado (JWT) → QR.**
- Registro de órdenes, conciliación y reportes.
- Gestión de llaves criptográficas (rotación, distribución de la pública a las máquinas).

## 2. Stack recomendado

| Capa | Elección | Por qué |
|------|----------|---------|
| **Backend** | **Go** | Lo pediste; ideal: binario único, rápido, excelente para firmar JWT y para correr barato en un VPS pequeño. Librerías: `net/http` o `chi`/`echo`, `golang-jwt` o `lestrrat-go/jwx`. |
| **Base de datos** | **PostgreSQL** (pgx) ✅ | **Migrado de SQLite a Postgres** (ADR-020/021). Esquema en `internal/store/schema.sql`. En el piloto corre como contenedor en la misma EC2; RDS al escalar. |
| **Frontend** | HTML server-rendered (Go templates) + JS mínimo | Front ultraligero, sin SPA (ADR-011bis). Design system en `internal/web/templates/base.html`. |
| **QR** | `skip2/go-qrcode` | Genera el QR desde el token en el servidor. |
| **Hosting** | **AWS EC2 + Docker Compose** ✅ (ADR-021) | Desplegado y **vivo en `grabi.napi.lat`**. **CI/CD con GitHub Actions → EC2 vía SSM + OIDC** (build sostenido por swap de 2 GB). Secretos en un `.env` `600` en la EC2, nunca al repo. Backups `pg_dump`→S3 diferidos para el MVP. Guía: `software/DESPLIEGUE.md`. |
| **Dominio + TLS** | `grabi.napi.lat` + Caddy (TLS automático) ✅ | Caddy en la misma caja da HTTPS. |

## 3. Arquitectura del flujo (crítico)

```
Cliente (celular)                Servidor (Go)                 Máquina (ESP32, offline)
      │                               │                                │
 1. Abre dominio.com/ID  ───────────► │                                │
 2. ◄──── catálogo + stock de ID ──── │                                │
 3. Selecciona productos, paga  ────► │  (flujo Bre-B, ver Dept. 04)   │
 4.                                   │  Verifica pago recibido        │
 5.                                   │  Crea orden + firma JWT        │
 6. ◄──── muestra QR (JWT) ────────── │                                │
 7. Muestra QR al lector ───────────────────────────────────────────► │
 8.                                   │           Verifica firma (llave pública local)
 9.                                   │           ¿jti ya usado? ¿machine_id ok? ¿no expiró?
10.                                   │           Dispensa y guarda jti en memoria
```

La máquina **nunca habla con el servidor** para vender. Solo necesita su llave pública (cargada al aprovisionarla).

## 4. Diseño del token de dispensado (JWT)

**Algoritmo de firma: `EdDSA` (Ed25519).** Ver justificación técnica completa en [Firmware](./03-firmware-electronica.md#algoritmo-de-firma). Resumen: la máquina solo guarda la **llave pública**; el ESP32 verifica Ed25519 en pocos milisegundos; nadie puede falsificar tokens sin la privada del servidor.

> **La fuente de verdad es [`especificaciones/contrato-token.md`](../especificaciones/contrato-token.md) v2.**
> Este bloque es solo un resumen; ante cualquier duda, manda el contrato.

**Payload v2** (ADR-006 quitó `iss` e `iat` del token; se guardan solo en el servidor):

```json
{
  "mid": "M001",                     // machine_id: el token solo sirve en ESA máquina
  "jti": "ord_7f3a9c2e",             // id único de orden → anti-reuso
  "exp": 1752461100,                 // expira (~5 min) → limita ventana de abuso
  "items": [ { "s": 3, "q": 1 }, { "s": 5, "q": 2 } ]   // s=slot, q=cantidad
}
```

**Reglas de validación en la máquina (orden exacto en el contrato §5):**
1. Firma Ed25519 válida con la llave pública local (`kid` del header).
2. `mid` == id de esta máquina.
3. `exp` no vencido (requiere reloj/RTC en la máquina; ver nota abajo).
4. `jti` no está en la lista de usados (memoria no volátil — resuelto con NVS, paso 5b).

> **Nota sobre el reloj offline:** el ESP32 no tiene hora real sin internet/RTC. Opciones: (a) módulo **RTC DS3231** barato (recomendado, ~confiable por años); (b) si no hay RTC, usar solo `jti` + una ventana basada en contador. El RTC es la opción robusta y cuesta poco — coordinar con Dept. 03.

**Tamaño del QR:** un JWT EdDSA es compacto, pero vigilar que quepa cómodo en un QR legible por el GM65. Mantener el payload mínimo (slots numéricos, no nombres largos). Si crece, considerar un formato binario propio firmado (CBOR/COSE) en vez de JWT clásico — decisión conjunta con Dept. 03.

## 5. Gestión de llaves

- **Par de llaves por flota** (una privada en el servidor, pública en todas las máquinas) para el MVP. Simple.
- **Evolución:** llave por lote/máquina para poder revocar sin afectar a todas. Registrar qué máquina tiene qué `kid` (key id) en el header del JWT.
- La **privada nunca sale del servidor** (idealmente en un secreto/variable de entorno cifrada, no en el repo).
- Procedimiento de **aprovisionamiento**: al fabricar una máquina, cargarle `machine_id` + llave pública + (opcional) su propio `kid`.

## 6. Tareas — Fase MVP (✅ hechas)

- [x] Esquema de datos (`machines`, `products`, `machine_products`, `orders`, `order_items`, `used_jti`, `bank_movements`) — en Postgres.
- [x] Endpoint `GET /m/{id}`: catálogo + stock por máquina.
- [x] Flujo de pago: **conciliación por correo Bre-B** (Dept. 04), match por monto + nombre (ADR-018).
- [x] Servicio de firma Ed25519 + vectores de prueba + llave pública para aprovisionar.
- [x] Tras confirmar pago → crea orden y emite el **QR**.
- [x] Panel admin (máquinas, productos/precios, stock/refill, órdenes, movimientos).
- [x] Desplegado con dominio + TLS (**EC2 + Caddy, vivo en `grabi.napi.lat`**) + CI/CD.
- [x] **Simulador de verificación** (`dsp verify`) que da los mismos códigos que el ESP32.
- [x] **Rediseño de UI** (ADR-022) — cliente + admin, "kiosko oscuro".

### Backlog actual (tareas pequeñas — una por sesión de agente)
- [ ] **Landing pública + captura de interesados** (semilla CRM) → `especificaciones/landing-v1.md`.
- [ ] **Panel de Configuración por máquina** (ADR-022): llave Bre-B desde el panel (hoy en `.env`),
      nº de canales, activar/desactivar, nombre, `kid` — para dar de alta máquinas sin redeploy.

## 7. Entregables

- Repositorio Go desplegable (backend + templates).
- Documentación de la API y del formato del token (fuente de verdad compartida con Dept. 03).
- Herramienta CLI de aprovisionamiento de máquinas.
- Simulador de verificación (para pruebas y para Dept. 03).

## 8. KPIs

- Tiempo de carga de `/m/ID` en 4G < 2 s.
- Tiempo desde "pago confirmado" hasta "QR en pantalla" < 5 s.
- 0 tokens válidos emitidos sin pago confirmado (integridad).
- Costo de infraestructura mensual (mantener < USD ~10 en piloto).

## 9. Seguridad (checklist)

- Llave privada fuera del repositorio y del cliente.
- `exp` corto + `jti` de un solo uso → un QR filtrado no sirve dos veces ni por siempre.
- `machine_id` en el token → un QR de la máquina A no funciona en la B.
- HTTPS obligatorio en la web.
- Rate limiting en endpoints de pago para evitar abuso.
- Registrar cada orden y cada verificación para auditoría/conciliación.

## 10. Dependencias

- **Con Dept. 03:** formato exacto del token, algoritmo, manejo del reloj/RTC, tamaño de QR. **Deben acordar un contrato único.**
- **Con Dept. 04 (Pagos):** cómo el backend se entera de que un pago llegó (correo, webhook, API).
- **Con Operaciones:** el panel de stock debe reflejar reabastecimientos.

---

## 11. Notas para otros departamentos

### Para Firmware (Dept. 03) — actualizar la PoC a v2
- La implementación de referencia de firma/verificación está en `software/internal/dsptoken`
  y ya sigue el contrato **v2** (payload `{mid, jti, exp, items}`, sin `iss`/`iat`; se elimina
  el código `BAD_ISSUER`; orden de validación §5, códigos §7).
- **Vectores REGENERADOS para v2** en `especificaciones/vectores-prueba/`:
  `llave-publica-k1.txt` (sin cambios — mismo par de llaves), `token-valido.txt`,
  `token-expirado.txt`, `token-firma-mala.txt`, `token-valido.png` (QR nuevo) y
  `resultados-esperados.md`. **Los vectores v1 quedan obsoletos.**
- Evaluar con `NOW = 1752460900` y `MACHINE_ID = "M001"` (los tokens llevan `exp` fijo).
  El firmware DEBE dar los mismos códigos que el simulador `dsp verify`:
  `token-valido → OK`, `token-expirado → EXPIRED`, `token-firma-mala → BAD_SIGNATURE`.
- **Buena noticia de tamaño:** con v2, el token de **2 items baja a 258 chars** (antes 318).
  Queda holgado bajo el objetivo de ~300 del §6.
- **Pendiente conjunto:** confirmar con el GM65 real que el QR se lee bien desde varias
  pantallas de celular (checklist §11 del contrato).

### Para Daniel / Gerencia
- **ADR-006 cerrada por parte de Software (02):** migrado `dsptoken` a v2 y regenerados los
  vectores. El token de 2 items pasó de 318 a **258 chars**. Queda pendiente solo la
  validación con el GM65 real (frente de Firmware/Hardware).
- **Rediseño de UI COMPLETO (2026-07-25, ADR-022):** todo el front pasó al lenguaje "kiosko oscuro 1a"
  (cliente + admin con sidebar), fiel a los mockups, en móvil y escritorio. Incluye modo
  **reabastecer**, panel de **movimientos**, campo `channels` (canales libres), y **estados de error**
  (máquina no encontrada + 404). Se **cerró ADR-017**: un producto **sin motor** ahora se **oculta de
  la venta** (no aparece al cliente ni se puede comprar). Terminología: la UI dice "canal", el código
  sigue en `slot`.
- **Pendiente (futuro, no bloquea el piloto) — panel de CONFIGURACIÓN por máquina:** editar la
  **cantidad de canales**, y sobre todo **configurar la llave Bre-B de cada máquina desde el panel**
  (hoy depende del `.env` `GRABI_BREB_KEY_M00X`) para no requerir redeploy al dar de alta una máquina,
  más activar/desactivar, nombre y `kid`. Detalle en ADR-022.
