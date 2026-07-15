# Plan de port a ESP32 + GM65 — Firmware (Dept. 03)

> **Estado:** PREPARADO, **bloqueado por hardware**. La PoC de verificación cripto en PC ya
> está validada contra el contrato **v2** (ver `poc-verificacion/`). Este documento deja listo
> el port al ESP32 para arrancar **el día que lleguen los componentes** de la Fase A
> (`hardware/lista-compra-piloto.md`). No requiere gastar nada: es diseño y esquema de conexiones.

---

## 0. Qué ya está hecho (no se re-hace)

- **Lógica de verificación** (`poc-verificacion/verificar.c`): parseo del token, Base64URL,
  verificación **Ed25519** con Monocypher (`crypto_ed25519_check`, SHA-512, ver ADR-008),
  validaciones §5 v2 (firma → `mid` → `exp` → `jti`) y códigos §7. **Se porta tal cual**;
  Monocypher es C portable a microcontrolador sin dependencias.
- **Vectores de prueba v2** oficiales para la prueba de humo del ESP32.

## 1. Qué cambia al pasar de PoC (PC) a firmware (ESP32)

| Pieza | En la PoC (PC) | En el ESP32 |
|-------|----------------|-------------|
| **Entrada del token** | archivo `.txt` | **UART** desde el **GM65** (el lector envía el string del QR) |
| **Reloj (`now`)** | parámetro `--now` | **RTC DS3231** por **I²C** (`now = rtc_epoch()`) |
| **Registro de `jti`** | arreglo en RAM | **NVS** (flash), persistente; marcar usado **antes** de dispensar (§5 p.9) |
| **Verificación cripto** | Monocypher en PC | **el mismo Monocypher**, sin cambios |
| **Llave pública `k1`** | archivo b64 | grabada en **NVS** al aprovisionar (§9 contrato) |
| **`MACHINE_ID`** | `#define M001` | leído de **NVS** (una imagen de firmware sirve para todas las máquinas) |
| **Salida (dispensar)** | imprime `items` | **driver de motor** + **sensor** de confirmación con timeout |
| **Feedback** | stdout | LED / buzzer / (opcional) OLED, mapeado a los códigos §7 |

## 2. Pinout propuesto (ESP32 DevKit WROOM-32) — *a confirmar con Dept. 01*

| Periférico | Señal | Pin ESP32 sugerido | Notas |
|-----------|-------|--------------------|-------|
| **GM65** | UART TX→RX | GPIO16 (RX2) | UART2. GM65 en modo UART, 9600/115200 8N1 (confirmar config del módulo) |
| **GM65** | UART RX←TX | GPIO17 (TX2) | Para enviar comandos de configuración al lector si hace falta |
| **GM65** | VCC / GND | 5V / GND | El GM65 típico es 5V; **usar nivel lógico** si su RX no tolera 3V3 |
| **DS3231** | I²C SDA | GPIO21 | I²C por defecto |
| **DS3231** | I²C SCL | GPIO22 | Pull-ups del módulo suelen bastar |
| **DS3231** | VCC / GND | 3V3 / GND | Pila CR2032 mantiene la hora |
| **Driver motor** | PWM/EN | GPIO25 | MOSFET IRLZ44N (lógica 3V3 OK) o L298N |
| **Sensor salida** | entrada dig. | GPIO34 (in-only) | Microswitch/IR; con debounce; define "producto cayó" |
| **LED estado** | salida | GPIO2 | Verde OK / rojo error |
| **Buzzer** | salida | GPIO27 | Beep OK / patrón de error |

> **Alimentación:** fuente 12V→motor; buck a 5V→ESP32/GM65. **Masa común** obligatoria.
> Añadir fusible y diodo de retorno (flyback) en el motor. Cerrar valores con Dept. 01 (motor/sensor definitivos).

## 3. Máquina de estados (contrato §5, orden exacto)

```
IDLE ──(UART: string recibido)──> VERIFICANDO
VERIFICANDO:
    r = verificar_token(buf, pubkey_k1, rtc_now())   // MISMA función que la PoC
    r != OK  -> ERROR(r)               // feedback §7 (LED/buzzer/OLED), volver a IDLE
    r == OK  -> nvs_marcar_jti(jti)    // PERSISTENTE, ANTES de mover motores (§5 p.9)
             -> DISPENSANDO
DISPENSANDO (por cada item {s,q}, q veces):
    activar_motor(slot s); esperar_sensor(timeout)
    sensor OK    -> siguiente
    timeout      -> DISPENSE_FAIL(s): log + marcar para reembolso/soporte (§7)
    -> CONFIRMAR_OK -> IDLE
```

- **Regla de oro:** `jti` a NVS **antes** de accionar motores → un reinicio a mitad no permite
  doble dispensado (§5, riesgo "corte de energía").
- **Poda de `jti`:** guardar `jti` junto a su `exp`; al arrancar/periódicamente borrar los `exp`
  vencidos para no llenar la NVS (§5 del departamento).

## 4. Estructura de proyecto sugerida (cuando llegue el HW)

```
firmware/esp32-dispensadora/     (ESP-IDF o Arduino-ESP32)
  main/
    main.c                 máquina de estados + wiring de periféricos
    verificar.c/.h         <- copiado de poc-verificacion/ (lógica cripto, sin tocar)
    gm65_uart.c/.h         leer string del QR por UART
    rtc_ds3231.c/.h        now() por I²C
    jti_store.c/.h         anti-reuso en NVS + poda por exp
    motor.c/.h             driver + sensor con timeout
    provisioning.c/.h      cargar MACHINE_ID, kid->pubkey, hora RTC
  monocypher/              <- mismo vendor 4.0.2 de la PoC
```

## 5. Prueba de humo (criterio de éxito del port)

1. Aprovisionar: grabar `MACHINE_ID=M001`, `k1`→llave pública de prueba, ajustar RTC.
2. Mostrar en un celular el QR de `especificaciones/vectores-prueba/token-valido.png` → el GM65
   lo lee → el ESP32 responde **OK** y acciona el motor (con `now` real vs `exp`; ojo: el `exp`
   de los vectores es fijo y **ya venció** en tiempo real → para esta prueba, usar un token
   recién emitido por el backend, o un `token-valido` regenerado con `exp` futuro).
3. Reintentar el mismo QR → **ALREADY_USED** (anti-reuso NVS funcionando).
4. QR de otra máquina → **WRONG_MACHINE**; QR corrupto → **BAD_SIGNATURE**.
5. **Checklist §11 del contrato:** confirmar que el GM65 lee el QR de 2–3 items desde varios
   celulares (brillo/ángulo). Éste es el punto que sólo el hardware puede cerrar.

## 6. Dependencias para desbloquear

- **Fase A comprada y recibida** (`hardware/lista-compra-piloto.md`): ESP32, GM65, DS3231,
  motor+driver, sensor, fuente.
- **Dept. 01:** motor/sensor/voltajes definitivos del mecanismo de dispensado elegido
  (espiral vs gravedad) → cierra el pinout §2 y el driver §4.
- **Backend (02):** endpoint/CLI que emita un `token-valido` con `exp` futuro para las pruebas
  en vivo (los vectores tienen `exp` fijo ya vencido).
