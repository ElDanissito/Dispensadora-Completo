# Proyecto Dispensadoras — Vending económica con pago digital y máquina offline

> Máquina expendedora de bajo costo, **sin billetero, sin monedero y sin datáfono físico**.
> El pago ocurre en una página web por máquina (`dominio.com/ID`), y la compra se entrega
> mostrando un **QR firmado (JWT)** al lector de la máquina, que valida **offline** y dispensa.

Este repositorio es el "cerebro" de la empresa. Cada archivo es un **departamento** con su
misión, responsabilidades, tareas y KPIs. El [Plan Maestro](./plan-maestro.md) une todo y
explica cómo gestionar la operación desde una sola gerencia (tú + agentes de IA).

---

## Cómo está organizado

| # | Documento | De qué se encarga |
|---|-----------|-------------------|
| 🧭 | [**plan-maestro.md**](./plan-maestro.md) | Visión, roadmap por fases, cómo coordinar todo, riesgos e hitos. **Empieza aquí.** |
| 01 | [Producto / Hardware mecánico](./departamentos/01-producto-hardware.md) | Diseño de la máquina: mecanismo de dispensado, estructura, BOM, prototipo. |
| 02 | [Software / Web](./departamentos/02-software-web.md) | Página de venta por máquina, backend Go, generación del QR con JWT firmado. |
| 03 | [Firmware / Electrónica](./departamentos/03-firmware-electronica.md) | ESP32, lector QR, verificación de firma offline, control de motores. |
| 04 | [Pagos (Bre-B) e integración](./departamentos/04-pagos.md) | Cobro por Bre-B, conciliación automática, ruta a integración oficial / pasarela. |
| 05 | [Operaciones / Logística](./departamentos/05-operaciones-logistica.md) | Reabastecimiento, ubicaciones, inventario, mantenimiento, rutas. |
| 06 | [Comercial / Ventas y Marketing](./departamentos/06-comercial-marketing.md) | Captación de puntos, modelo de ingresos, marketing en punto y digital. |
| 07 | [Finanzas y Legal / Regulatorio](./departamentos/07-finanzas-legal.md) | Unit economics, precios, punto de equilibrio, constitución, DIAN, sanidad. |

---

## La idea en 6 pasos

1. El cliente frente a la máquina abre `dominio.com/<ID>` (o escanea un QR estático pegado en la máquina).
2. La web muestra los productos **de esa máquina** y su stock.
3. El cliente paga por **Bre-B** (transferencia inmediata a la llave del negocio).
4. Al confirmar el pago, la web genera un **QR con un JWT firmado** que contiene: productos comprados + `machine_id` + timestamp + id único de orden.
5. El cliente muestra ese QR al **lector de la máquina** (GM65). La máquina **verifica la firma con su llave pública** (no necesita internet).
6. Si es válido y no fue usado, la máquina lo guarda en memoria (anti-reuso) y **dispensa**.

## Principios de diseño

- **Costo mínimo**: sin componentes de manejo de efectivo (los más caros y frágiles de una vending).
- **Offline-first**: la máquina nunca depende de conexión para vender; solo valida firmas.
- **Seguridad por criptografía**: la máquina solo tiene la **llave pública**; nadie puede fabricar QR válidos sin la privada del servidor.
- **Anti-fraude**: cada QR es de un solo uso (timestamp + jti registrado en memoria) y atado a un `machine_id`.
- **Empresa lean**: cada departamento lo operas tú apoyado por agentes de IA; los documentos están escritos para eso.

## Estado del proyecto

- **Fase actual**: 0 → 1 (definición + piloto de 1 máquina). Presupuesto: 1 prototipo.
- **Pago inicial**: Bre-B con conciliación semi-automática (revisión de correo/notificación). Meta: QR dinámico e integración oficial.
- **Última actualización**: 14 de julio de 2026.
