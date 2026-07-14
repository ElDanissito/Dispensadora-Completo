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
| **Base de datos** | **PostgreSQL** (o SQLite para arrancar) | SQLite basta para el piloto (1 archivo, cero costo). Migrar a Postgres al escalar. |
| **Frontend** | HTML server-rendered (Go templates) + un poco de JS, o **HTMX** | Mantén el front ultraligero: la página de venta debe cargar rápido en el celular del cliente. Evita SPA pesada al inicio. |
| **QR** | Librería Go de QR (`skip2/go-qrcode`) | Genera el QR desde el JWT en el servidor. |
| **Hosting** | VPS barato (Hetzner/DigitalOcean/Contabo) o Fly.io | Un binario Go + SQLite corre en la instancia más pequeña. |
| **Dominio + TLS** | Dominio `.co` + Caddy (TLS automático) | Caddy simplifica HTTPS. |

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

**Payload de ejemplo:**

```json
{
  "iss": "dispensadoras.co",
  "mid": "M001",                     // machine_id: el token solo sirve en ESA máquina
  "jti": "ord_7f3a9c2e",             // id único de orden → anti-reuso
  "iat": 1752460800,                 // emitido en
  "exp": 1752461100,                 // expira (p.ej. 5 min) → limita ventana de abuso
  "items": [                          // qué dispensar
    { "slot": 3, "qty": 1 },
    { "slot": 5, "qty": 2 }
  ]
}
```

**Reglas de validación en la máquina (todas deben cumplirse):**
1. Firma Ed25519 válida con la llave pública local.
2. `mid` == id de esta máquina.
3. `exp` no vencido (requiere reloj en la máquina; ver nota abajo).
4. `jti` no está en la lista de usados (memoria no volátil).

> **Nota sobre el reloj offline:** el ESP32 no tiene hora real sin internet/RTC. Opciones: (a) módulo **RTC DS3231** barato (recomendado, ~confiable por años); (b) si no hay RTC, usar solo `jti` + una ventana basada en contador. El RTC es la opción robusta y cuesta poco — coordinar con Dept. 03.

**Tamaño del QR:** un JWT EdDSA es compacto, pero vigilar que quepa cómodo en un QR legible por el GM65. Mantener el payload mínimo (slots numéricos, no nombres largos). Si crece, considerar un formato binario propio firmado (CBOR/COSE) en vez de JWT clásico — decisión conjunta con Dept. 03.

## 5. Gestión de llaves

- **Par de llaves por flota** (una privada en el servidor, pública en todas las máquinas) para el MVP. Simple.
- **Evolución:** llave por lote/máquina para poder revocar sin afectar a todas. Registrar qué máquina tiene qué `kid` (key id) en el header del JWT.
- La **privada nunca sale del servidor** (idealmente en un secreto/variable de entorno cifrada, no en el repo).
- Procedimiento de **aprovisionamiento**: al fabricar una máquina, cargarle `machine_id` + llave pública + (opcional) su propio `kid`.

## 6. Tareas — Fase MVP

- [ ] Definir esquema de datos: `machines`, `products`, `machine_products` (stock por máquina), `orders`, `order_items`, `used_jti` (auditoría).
- [ ] Endpoint `GET /m/{id}`: render de catálogo + stock de esa máquina.
- [ ] Flujo de pago (integración con Dept. 04, empezar con verificación semi-manual/correo).
- [ ] Servicio de firma: generar par Ed25519, firmar JWT, exponer llave pública para aprovisionar máquinas.
- [ ] Endpoint que, tras confirmar pago, cree la orden y devuelva el **QR** (imagen) al cliente.
- [ ] Panel admin mínimo: crear máquinas, cargar productos/precios, ajustar stock, ver órdenes.
- [ ] Desplegar en VPS con dominio + TLS.
- [ ] **Simulador de máquina** (script) que verifique el JWT igual que el ESP32, para probar sin hardware.

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
