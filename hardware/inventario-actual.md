# Inventario de hardware disponible (de proyecto anterior)

> Lo que Daniel YA tiene en mano (2026-07-14). Base real para las pruebas del piloto.
> El agente de Firmware (03) debe trabajar con esto antes de asumir componentes nuevos.

## Lo que hay

| Componente | Detalle | Estado / notas |
|-----------|---------|----------------|
| **ESP32** | **30 pines** (15/lado), **USB-C**, loader **CP2102**. Con **base de borneras** | Cableado sin soldar. Ver pinout abajo. |
| **Lector QR GM65** | Módulo 2D UART/TTL | ¡Ya lo tiene! Era el componente crítico. Confirmar baudios y modo de disparo. |
| **Sensor E18-D80NK** | IR de proximidad ajustable (~3–80 cm), 3 hilos (VCC/GND/OUT), salida **NPN NO** | Sirve como sensor de "producto cayó". **OJO nivel lógico:** si se alimenta a 5V, su OUT puede dar 5V → el GPIO del ESP32 NO tolera 5V. Medir y usar divisor o pull-up a 3.3V. |
| **Circuito casero de motores** | Soporta hasta **4 motores** | Hecho en casa; **falta documentar** cómo está armado (driver, nivel de control, diodos de protección). Verificar antes de conectar al ESP32. |
| **Fuente 12V** | Alimentación de motores | OK. |
| **Motores** | **5 unidades**, pero solo **2 usables** (faltan conectores para los otros 2) | Suficiente para probar 2 canales (snack + bebida). |

## Pinout real (confirmado por Daniel)

| Señal | GPIO | Lógica / notas |
|-------|------|----------------|
| Motor 1 | **D27** | **Activo en BAJO (lógica inversa):** HIGH = quieto, LOW = mueve. **Conectado y usable.** |
| Motor 2 | **D14** | Igual (activo en bajo). **Conectado y usable.** |
| Motor 3 | **D12** ⚠️ | Igual (activo en bajo). Sin conector aún (no usable todavía). **OJO:** GPIO12 es un **strapping pin** que debe estar en **BAJO al arrancar**; si se mantiene en HIGH puede **bloquear el flasheo/boot**. No urgente mientras no se use; al conectar el Motor 3, considerar **reasignar a GPIO25 / GPIO32 / GPIO33.** |
| Motor 4 | **D13** | Igual (activo en bajo). Sin conector aún (no usable todavía). |
| Sensor de caída (E18-D80NK) | **D26** | Entrada. Distancia **configurable** en el propio sensor. |
| Alimentación | **VIN / GND (5V)** | Buck interno **12V→5V** alimenta ESP32 y sensor. |
| **Lector GM65 (UART)** | **RX2=GPIO16, TX2=GPIO17** | UART2. GM65 TX→GPIO16, GM65 RX→GPIO17. Alimentar GM65 a **3.3V** (para que su TX sea seguro al ESP32). Baudios de fábrica ~9600. |

- **Estado inicial de los motores:** poner los 4 pines en **HIGH** al arrancar (`OUTPUT` + `HIGH`)
  para que ningún motor se mueva solo en el boot. Mover = pulso a **LOW** durante el dispensado.
- **Usables ahora:** Motor 1 (D27) y Motor 2 (D14) — conectados físicamente. Suficiente para snack + bebida.

## Toolchain

- **Arduino IDE** para compilar y cargar (placa: "ESP32 Dev Module").
- **Si al cargar sale `Connecting...` + `Write timeout`:** (1) mantener **BOOT** al subir hasta que
  empiece a escribir; (2) **desconectar el circuito de motores** — GPIO12/D12 en HIGH bloquea el
  flasheo; (3) bajar **Upload Speed a 115200**; (4) cerrar el Monitor Serie y usar cable de datos.
- Ed25519 en Arduino: opción A, compilar **Monocypher** dentro del sketch (ya validado en la PoC,
  ADR-008); opción B, librería **Crypto (rweather)** que trae Ed25519 estándar (RFC 8032 / SHA-512,
  también compatible con el backend Go). RTC: librería **RTClib** (Adafruit) para el DS3231 cuando llegue.

## Lo único que falta

| Componente | Acción |
|-----------|--------|
| **RTC DS3231** ("el del tiempo") | **Pedir 2 unidades** (barato). Valida la expiración `exp` offline. **No bloquea** el inicio de pruebas: el firmware puede usar una hora fija temporal hasta que llegue. |

## Se puede empezar a probar YA

Con lo disponible se cubre el pipeline completo excepto la validación de `exp`:
**GM65 lee QR → ESP32 verifica firma (v2) → mueve motor → E18-D80NK confirma caída.**
El `exp` se añade cuando llegue el RTC.

## Antes de conectar nada — checklist de seguridad (para no quemar el ESP32)

- [x] **E18-D80NK → D26 — MEDIDO Y RESUELTO (2026-07-14).** Alimentado a **5V**. Medido con multímetro:
      **reposo ≈ 2.2V, detección (mano) = 0V.** El 2.2V es la salida **flotando** y queda por debajo del
      umbral de HIGH fiable del ESP32 (~2.5V) → **usar `pinMode(D26, INPUT_PULLUP)`**: el pull-up interno
      lo sube a **3.3V limpio** en reposo y el sensor lo tira a **0V** al detectar. Como es **NPN colector
      abierto** (solo tira a GND, nunca empuja 5V al pin), **no hay riesgo de 5V en el D26** aunque el
      sensor se alimente a 5V. **Lógica en código: HIGH = libre, LOW = producto detectado.**
      Verificación de seguridad: con el pull-up activo, el pin en reposo debe medir ~3.3V (no más).
- [ ] **GM65 (UART) → ESP32:** confirmar a qué voltaje va la línea TX del GM65. Si es 5V, alimentar
      el GM65 a 3.3V o usar level shifter; el RX del ESP32 no tolera 5V.
- [ ] **Doble fuente de 5V:** al **programar por USB** hay 5V del USB; si el **buck 12→5V** también
      alimenta el riel de 5V/VIN, evitas back-feeding **cargando con USB y el buck apagado** (motores
      sin energía). Para operar normal: solo el buck, sin USB. No alimentar los motores desde el USB.
- [ ] **GND común** unido entre fuente 12V, buck, circuito de motores, sensor y ESP32.
- [ ] Motores en **HIGH** desde el `setup()` para que no se muevan en el arranque (lógica inversa).

## Tareas para el agente de Firmware (03)

1. Con fotos del montaje, **documentar el pinout real** (ESP32 ↔ GM65, ↔ circuito de motores, ↔ E18-D80NK).
2. Producir un **diagrama de conexiones** para ESTOS componentes.
3. Escribir **código de prueba incremental** (un paso a la vez, no todo de golpe):
   1. [x] **Blink + motor — HECHO (2026-07-14).** Sketch `paso1-motores.ino` funciona: mueve un motor con lógica inversa. Daniel corrigió el pinout (estaba invertido respecto al circuito físico): Motor 1 = D27, Motor 2 = D14.
   2. [x] **Paso 2 — HECHO (2026-07-14).** GM65 por UART2 (RX2=GPIO16, TX2=GPIO17) con `firmware/paso2-gm65/paso2-gm65.ino`. Leyó `token-valido.png`: 258 chars del token v2 completos e intactos (`eyJhbGciOiJFZERTQS...`).
   3. [x] **Paso 3 — HECHO (2026-07-14). ¡Núcleo de seguridad en hardware!** `firmware/paso3-verificacion/paso3-verificacion.ino` con **Monocypher** (`crypto_ed25519_check`, RFC 8032/SHA-512, compatible con Go). En el ESP32, escaneando por GM65: `token-valido`→**`OK`** + items `[{s:3,q:1},{s:5,q:2}]`; segundo escaneo del mismo → **`ALREADY_USED`** (anti-reuso demostrado). `NOW = 1752460900` fijo (sin RTC). Anti-reuso de `jti` en **RAM** → **pendiente pasar a NVS** para que sobreviva reinicios.
   4. [x] **Paso 4 — HECHO (2026-07-14). ¡FLUJO COMPLETO EN HARDWARE!** `firmware/paso4-dispensado/paso4-dispensado.ino` (paso 3 + motores). Escanear `token-valido` → verifica `OK` → **dispensa** moviendo el/los motor(es) del slot (lógica inversa + timeout). Segundo escaneo del mismo QR → **niega** (`ALREADY_USED`, no dispensa). Mapa slot→GPIO: slot 3→D27, slot 5→D14. El `jti` se marca antes de mover motores (§5). **Prueba de concepto del producto completa: cobra por QR firmado, offline, dispensa una sola vez.**
   5. [x] **Paso 5 — HECHO (2026-07-14). ¡Ciclo físico completo y a prueba de fallos!** `firmware/paso5-dispensado-sensor/` integra el **E18-D80NK** (D26, `INPUT_PULLUP`; HIGH=libre, LOW=detectado). Al dispensar, el motor gira hasta que el sensor confirma la caída dentro de `MOTOR_TIMEOUT_MS` (4 s): detecta→OK y corta; timeout (boca bloqueada)→**`DISPENSE_FAIL`**. Sketch de calibración `firmware/paso5-sensor-minimo/` para ajustar la distancia. Ciclo: **leer → verificar → dispensar → confirmar caída → si no cae, marcar fallo.** La máquina física está completa.

   **Pendientes de firmware (refinamientos, no bloquean):** (a) anti-reuso `jti` en **NVS** (hoy RAM); (b) **RTC DS3231** para `exp` cuando llegue el módulo (hoy `NOW` fijo).
   6. [ ] Unir todo en la máquina de estados (con `exp` fijo hasta que llegue el RTC).
