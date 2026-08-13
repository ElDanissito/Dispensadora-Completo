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
| 2026-08-XX | Madera | Tablero **MDF 4 mm · 183 × 244 cm** (con cortes hechos) | *(por confirmar)* | **103.000** | pendiente | Entrega prevista mañana. **Rinde para 1,5 máquinas**: ~2/3 (≈ 68.700 COP) se cargan a M001 y ~1/3 (≈ 34.300 COP) queda como material sobrante disponible para media M002 — al pedir madera para M002 solo se completa la otra mitad. Ajustar el reparto real cuando se corte. |

> **Instrucciones para llenar:** cuando llegue una factura, agregar la fila con **fecha real**,
> **proveedor**, **monto exacto** y **link/ruta de la factura**. No inventar montos.
> Los ~$254 USD de la PCB de 46 canales van en un ledger aparte (es **CAPEX de la máquina #2**, no de M001).

## Totales (a la fecha)

- **Salida de caja para M001 (efectivo real):** **163.000 COP** (60.000 herrajes + 103.000 madera).
- **CAPEX imputable a M001 (contable):** **~128.700 COP** — herrajes 60.000 + **2/3 del tablero** (≈ 68.700).
- **Material sobrante disponible para M002 (media máquina):** **~34.300 COP** — 1/3 del tablero MDF.
- **CAPEX M001 pendiente de confirmar:** hardware que ya estaba en stock (ESP32, GM65, sensor E18,
  circuito de 4 motores, fuente 12V — heredado de proyecto anterior, sin costo nuevo pero con
  **valor de reposición** por estimar).

## Impacto en el unit economics

Cuando el CAPEX total de M001 esté cerrado, actualizar la **celda de CAPEX** en
`unit-economics-grabi-m001.xlsx` (hoja Supuestos) para recalcular el **payback real** vs. el
estimado inicial (~1,5 meses). Si el payback se estira, es señal para ajustar precios/mezcla de
productos, no para bajar los brazos.

## Historial (resumen)

- **2026-08-XX:** inicio del ledger. Primera compra de herrajes: 60.000 COP.
- **2026-08-XX:** compra de madera MDF 4 mm (183 × 244 cm) con cortes hechos: 103.000 COP.
  Rinde para 1,5 máquinas → 2/3 a M001, 1/3 sobra para M002.
