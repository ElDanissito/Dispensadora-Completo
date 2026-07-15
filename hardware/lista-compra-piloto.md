# Lista de compra — Hardware del piloto

> Ref. ADR-011. Producto piloto: **mixto snacks + bebidas** (ADR-009).
> Precios en COP **aproximados** (jul-2026) para dimensionar; **confirmar en MercadoLibre
> Colombia / tiendas locales** (Sigma, Vistronica, Electronilab, etc.) antes de comprar.
> Objetivo de esta compra: **validar la lectura del QR y el dispensado de 1 canal**, y tener con
> qué portar el firmware al ESP32. No es la máquina completa todavía.

---

## Fase A — Kit mínimo de validación (comprar YA)

Con esto pruebas: el GM65 leyendo el QR real desde el celular, el ESP32 verificando el token
(portando la PoC) y accionando **un** motor con confirmación de sensor.

| # | Componente | Para qué | Cant. | Precio aprox. (COP) |
|---|-----------|----------|-------|---------------------|
| 1 | **ESP32 DevKit** (WROOM-32) | Cerebro de la máquina | 1 | 25.000 – 45.000 |
| 2 | **Lector QR GM65** (módulo UART) | Leer el QR del cliente | 1 | 90.000 – 160.000 |
| 3 | **RTC DS3231** (módulo) | Validar `exp` offline | 1 | 8.000 – 20.000 |
| 4 | **Motor DC 12V con reductor** + **espiral** (o motor de vending usado) | Dispensar 1 canal (snack) | 1 | 30.000 – 90.000 |
| 5 | **Driver de motor** (MOSFET IRLZ44N o módulo, o L298N) | Que el ESP32 mueva el motor de 12V | 1 | 5.000 – 25.000 |
| 6 | **Microswitch** o **sensor IR** de salida | Confirmar que el producto cayó | 1–2 | 3.000 – 15.000 |
| 7 | **Fuente 12V** (≥3A) + **regulador a 5V** (buck) | Alimentar motor + ESP32/lector | 1 | 25.000 – 50.000 |
| 8 | Protoboard, cables dupont, borneras, fusible, interruptor | Montaje de prueba | 1 lote | 20.000 – 40.000 |

**Subtotal Fase A estimado:** ~**240.000 – 450.000 COP** (rango amplio según GM65 y motor).

### Cantidades y repuestos (importante en prototipado)

En prototipado los repuestos ahorran tiempo: lo barato y fácil de quemar se compra por varios;
lo caro/lento de importar, mínimo 2 para no volver a esperar el envío.

| Componente | Comprar | Motivo |
|-----------|---------|--------|
| ESP32 | **2–3** | Barato y fácil de freír (voltaje invertido, cable mal puesto). |
| Lector 2D (GM66/GM65) | **2** | Es lo caro y lento de importar; repuesto cubre unidad DOA o dañada. |
| Driver de motor (MOSFET/L298N) | **2–3** | El que **más se quema** (corriente y picos del motor). |
| RTC DS3231 | **2** | Cuesta poco; tener respaldo. |
| Motor + espiral | **1–2** | Con 2 se prueban canal de snack y de bebida a la vez. |
| Sensores (microswitch/IR) | **3–4** | Muy baratos, se dañan con el manoseo. |
| Fuente 12V | **1** | Robusta; no vale duplicar en el piloto. |

**Consumibles de protección a sumar:** fusibles, **level shifter** (3.3V ESP32 ↔ 5V lector),
cables dupont de sobra, protoboard extra y — si no lo tienes — un **multímetro** (herramienta #1
para medir antes de conectar y no quemar componentes).

> El **GM65 es el componente clave y el más caro** del kit. Si la lectura desde pantalla de
> celular fallara (brillo/ángulo), alternativas más robustas: **GM805/GM810** o un módulo
> Waveshare. Por eso esta fase valida ese punto ANTES de construir la máquina.

## Fase B — Máquina piloto completa (comprar tras validar Fase A)

Depende de una decisión pendiente: **¿la máquina lleva refrigeración para las bebidas?**
(afecta costo y consumo — ver DECISIONS.md). Componentes adicionales típicos:

- Más motores + espirales/canales: **snacks** (espiral fino) y **bebidas** (canal reforzado o
  dispensador por gravedad para latas). Mixto ⇒ **canales de dos tamaños**.
- Estructura/gabinete (lámina o MDF reforzado para el prototipo), puerta con chapa/candado.
- Refrigeración (si se decide): unidad tipo mini-nevera o sistema de compresor pequeño → sube
  costo y consumo; evaluar si el piloto arranca solo con bebidas no refrigeradas o con nevera aparte.
- Vidrio/acrílico frontal (opcional), iluminación LED, cableado definitivo.
- Cartel/QR estático con `dominio.com/ID` y las instrucciones de 3 pasos (Comercial 06).

## Notas para otros departamentos

- **Firmware (03):** con la Fase A puede portar la PoC al ESP32, integrar GM65 por UART, RTC,
  anti-reuso en NVS y driver de motor con sensor. Es su siguiente bloque de trabajo.
- **Hardware/Producto (01):** definir mecánica de los dos tamaños de canal y la decisión de
  refrigeración; cerrar el BOM de la Fase B con costos reales.
- **Finanzas (07):** el costo real de Fase A + Fase B alimenta el CAPEX por máquina del unit economics.

> Todos los precios son referenciales. **Confirmar cotizaciones reales** antes de comprar y
> registrar el costo efectivo en el unit economics.
