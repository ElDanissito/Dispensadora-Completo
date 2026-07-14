# 03 · Firmware / Electrónica

**Responsable:** Daniel + agente de IA de firmware/embebidos (C/C++, ESP-IDF/Arduino, criptografía embebida).
**Misión:** Que la máquina lea un QR, **verifique la firma sin internet**, evite reusos y active el dispensado de forma fiable y segura.

---

## 1. De qué se encarga

- Firmware del microcontrolador (ESP32).
- Lectura del QR (lector GM65 u otro).
- **Verificación criptográfica del token** con la llave pública local.
- Anti-reuso (registro de `jti` usados en memoria no volátil).
- Control de motores/actuadores y lectura de sensores de dispensado.
- Manejo del reloj (RTC) para validar expiración.
- Aprovisionamiento (cargar `machine_id` + llave pública).

## 2. Selección de hardware

| Componente | Recomendación | Notas |
|-----------|---------------|-------|
| **MCU** | **ESP32** (o ESP32-S3) | Suficiente potencia y RAM; SHA por hardware; barato; enorme comunidad. El S3 tiene más recursos si se necesita. |
| **Lector QR** | **GM65** para empezar | Barato, interfaz UART/USB, lee de pantalla de celular. Alternativas más robustas: **GM805 / GM810**, **Waveshare Barcode Scanner**, o módulos Honeywell si se quiere calidad industrial. |
| **Reloj** | **RTC DS3231** | Para validar `exp` offline. Muy preciso, mantiene hora con pila por años. Barato. |
| **Memoria no volátil** | Flash del ESP32 (NVS) o EEPROM/FRAM | Guarda `jti` usados. FRAM si se quiere muchísima escritura sin desgaste. |
| **Drivers de motor** | MOSFET / L298N / ULN2003 | Según el motor elegido en [Dept. 01](./01-producto-hardware.md). |
| **Sensores** | Microswitch / óptico | Confirmar dispensado. |

## 3. Algoritmo de firma {#algoritmo-de-firma}

> **Tu duda concreta:** "el ESP32 no procesa tan bien SHA256/ECDSA". En la práctica **no es problema**, porque solo verificas **una firma por venta** (unas pocas por minuto en el peor caso), no miles por segundo.

**Recomendación: firma asimétrica Ed25519 (EdDSA).**

- El **servidor firma** con la llave privada; la **máquina solo tiene la pública**. Si alguien abre la máquina y extrae su memoria, **no puede fabricar QR válidos** (no tiene la privada). Esta es la gran ventaja sobre HMAC.
- **Rendimiento en ESP32:** una verificación Ed25519 toma del orden de **unos pocos a ~decenas de milisegundos**. Para una venta cada varios segundos es totalmente sobrado.
- **Librería recomendada:** **Monocypher** (C, un solo archivo, sin dependencias, portable a microcontroladores, implementa Ed25519). Alternativas: la librería **Crypto de rweather** (Arduino) que incluye Ed25519, o **libsodium** portado. mbedTLS (incluido en ESP-IDF) hace SHA por hardware y ECDSA P-256 si prefieres ese camino.

### Comparación de opciones

| Opción | Llave en la máquina | Si abren la máquina | ESP32 | Veredicto |
|--------|---------------------|---------------------|-------|-----------|
| **Ed25519 (asimétrica)** | Solo **pública** | No pueden falsificar QR | Verifica en ms | ✅ **Recomendada** |
| ECDSA P-256 (asimétrica) | Solo pública | No pueden falsificar | Verifica en ms (mbedTLS) | ✅ Alternativa válida |
| HMAC-SHA256 (simétrica) | La **misma llave** que firma | **Pueden falsificar QR ilimitados** | Muy rápido (HW) | ⚠️ Solo si nunca se puede extraer la llave. Más riesgoso. |

**Conclusión:** usa **Ed25519**. Es el mejor equilibrio de seguridad, tamaño de firma pequeño (64 bytes → QR compacto) y facilidad. HMAC solo como último recurso.

### Sobre el tamaño del QR
JWT en Base64URL con firma Ed25519 (64 bytes) es manejable. Si el payload crece, evaluar con Dept. 02 un formato binario firmado (p. ej. **COSE/CBOR**) para reducir bytes y que el QR sea más fácil de leer por el GM65.

## 4. Lógica de verificación (pseudocódigo)

```c
on_qr_scanned(raw):
    token = parse(raw)
    if not ed25519_verify(token.header+payload, token.sig, PUBLIC_KEY):
        beep_error("firma inválida"); return
    if token.mid != MACHINE_ID:
        beep_error("máquina equivocada"); return
    if rtc_now() > token.exp:
        beep_error("expirado"); return
    if nvs_contains(token.jti):
        beep_error("ya usado"); return
    nvs_store(token.jti)              // marcar como usado ANTES de dispensar
    for item in token.items:
        dispense(item.slot, item.qty) // activa motor, espera sensor
    confirm_ok()
```

**Orden importante:** marcar el `jti` como usado **antes** de dispensar evita doble dispensado si hay un reinicio a mitad. Manejar el caso de fallo de dispensado (sensor no confirma) → registrar para reembolso/soporte.

## 5. Anti-reuso y memoria

- Guardar `jti` usados en NVS/EEPROM. Como cada `jti` es único y los tokens expiran, se puede **podar** periódicamente los `jti` cuyo `exp` ya pasó (si el `jti` codifica o se guarda junto al `exp`), evitando llenar la memoria.
- Contador de dispensados y log de eventos para diagnóstico.

## 6. Aprovisionamiento de una máquina

1. Grabar firmware.
2. Escribir en NVS: `MACHINE_ID`, `PUBLIC_KEY` (y `kid` si se usa).
3. Ajustar el RTC a la hora correcta.
4. Prueba de humo: escanear un QR de prueba firmado por el servidor y confirmar dispensado.

## 7. Tareas — Fase MVP

- [ ] Acordar con Dept. 02 el **contrato del token** (campos, algoritmo, encoding, tamaño máx. de QR).
- [ ] Prototipo de lectura GM65 → ESP32 por UART (imprimir el contenido del QR).
- [ ] Integrar Monocypher y verificar un JWT/token firmado de prueba.
- [ ] Implementar validaciones (mid, exp con RTC, jti anti-reuso en NVS).
- [ ] Driver de dispensado: activar motor + esperar sensor de confirmación con timeout.
- [ ] Máquina de estados completa (idle → escaneo → validación → dispensado → confirmación).
- [ ] Manejo de errores y realimentación al usuario (LED/buzzer/pantalla OLED opcional).
- [ ] Rutina de aprovisionamiento.

## 8. Entregables

- Firmware flasheable + instrucciones de compilación.
- Esquema eléctrico (coordinado con Dept. 01).
- Procedimiento de aprovisionamiento reproducible.

## 9. KPIs

- Tasa de lectura del QR al primer intento ≥ 95%.
- Tiempo de escaneo→dispensado < 3 s.
- 0 dispensados sin token válido; 0 doble-dispensados por reuso.

## 10. Riesgos y mitigación

- **Lector no lee la pantalla del celular** (brillo/ángulo) → probar el GM65 con varios celulares; considerar módulo más robusto; instruir al cliente (brillo alto).
- **Reloj desfasado** → RTC DS3231; procedimiento de ajuste; ventana de `exp` no demasiado corta.
- **Extracción de llave** → Ed25519 (solo pública en máquina) neutraliza el riesgo de falsificación.
- **Corte de energía a mitad de dispensado** → orden "marcar usado antes de dispensar" + log + política de soporte.

## 11. Dependencias

- **Con Dept. 02:** contrato del token es la interfaz más crítica del proyecto. Un solo documento de verdad, versionado.
- **Con Dept. 01:** voltajes, motores, sensores, espacio físico.
