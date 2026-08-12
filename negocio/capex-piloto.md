# CAPEX del piloto — GRABI M001

> Registro **vivo** de gastos únicos (una sola vez) que construyen la máquina M001 y su
> infraestructura de piloto. Complementa a [`costos-fijos-mensuales.md`](./costos-fijos-mensuales.md)
> (suscripciones recurrentes) y a [`unit-economics-grabi-m001.xlsx`](./unit-economics-grabi-m001.xlsx)
> (margen unitario). **Todo en COP.**
>
> **Regla contable:** aquí solo entra el **CAPEX del piloto M001** (herrajes, madera, electrónica ya
> usada del stock previo, PCB futura, sensores, etc.). NO entran:
> - Suscripciones mensuales (Claude, Flux, AWS) → van en `costos-fijos-mensuales.md`.
> - OPEX por máquina (reposición, arriendo del punto, datos) → van en el unit economics.
> - CAPEX de máquinas futuras (M002 refrigerada, PCB de 46 canales) → registrar aparte cuando se decida.

## Ledger

| Fecha | Categoría | Concepto | Proveedor | Monto (COP) | Factura | Notas |
|-------|-----------|----------|-----------|-------------|---------|-------|
| 2026-08-XX | Herrajes | Rieles, bisagras, tornillos, cerraduras con llaves | *(por confirmar)* | **60.000** | pendiente | Compra en tienda física. Insumos para el ensamble del gabinete de M001. |
| *pendiente* | Madera | Tablero(s) para el gabinete | *(por confirmar)* | *(por confirmar)* | pendiente | Compra prevista para mañana. |

> **Instrucciones para llenar:** cuando llegue una factura, agregar la fila con **fecha real**,
> **proveedor**, **monto exacto** y **link/ruta de la factura**. No inventar montos.
> Los ~$254 USD de la PCB de 46 canales van en un ledger aparte (es **CAPEX de la máquina #2**, no de M001).

## Totales (a la fecha)

- **CAPEX M001 confirmado:** **60.000 COP** (herrajes).
- **CAPEX M001 pendiente de confirmar:** madera (mañana) + hardware que ya estaba en stock (ESP32,
  GM65, sensor E18, circuito de 4 motores, fuente 12V — heredado de proyecto anterior, sin costo
  nuevo pero con **valor de reposición** por estimar).

## Impacto en el unit economics

Cuando el CAPEX total de M001 esté cerrado, actualizar la **celda de CAPEX** en
`unit-economics-grabi-m001.xlsx` (hoja Supuestos) para recalcular el **payback real** vs. el
estimado inicial (~1,5 meses). Si el payback se estira, es señal para ajustar precios/mezcla de
productos, no para bajar los brazos.

## Historial (resumen)

- **2026-08-XX:** inicio del ledger. Primera compra de herrajes: 60.000 COP.
