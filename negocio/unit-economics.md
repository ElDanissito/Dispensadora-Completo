# Hoja de Unit Economics — piloto (snacks + bebidas)

> Autor: Agente de Negocio · Fecha: 2026-07-14 · Acompaña a **[`unit-economics.csv`](./unit-economics.csv)** (ábrelo en Excel/Google Sheets)
> Base: piloto en Cali, persona natural, **sin refrigeración** (ADR-007/009/010 + recomendación en [`requisitos-sanitarios-piloto.md`](./requisitos-sanitarios-piloto.md))
> ⚠️ Los números son **ejemplos ilustrativos**. Reemplázalos con datos reales de cada departamento.

---

## Cómo usar la hoja

1. Abre `unit-economics.csv`. Está dividido en 6 bloques.
2. **Solo edita las celdas "Valor ejemplo"**; las filas con `=fórmula` explican cómo se calcula cada resultado.
3. Los datos reales vienen de: **precios/costos y mix** → Comercial (06); **ventas/día y merma** → Operaciones (05); **CAPEX** → Hardware (01) y Firmware (03); **comisión de pago** → Pagos (04).

## Modelo de dos categorías

La novedad de ADR-009 es modelar **dos márgenes distintos** y mezclarlos por el **mix de ventas**:

| | Snacks | Bebidas |
|---|---|---|
| Precio venta (ej.) | $3.000 | $4.000 |
| Costo mayorista (ej.) | $1.800 | $2.500 |
| **Margen/unidad** | **$1.200 (40%)** | **$1.500 (37,5%)** |
| Mix (ej.) | 60% | 40% |

**Promedio ponderado (blended):** precio $3.400 · costo $2.080 · **margen $1.320/unidad (≈38,8%)**.

> Las bebidas dejan **más pesos por unidad** pero **menos %**; los snacks al revés. El mix real moverá el margen — por eso se modela por separado. Si en el piloto se venden más bebidas, sube el ticket promedio y el margen absoluto.

## Resultado del ejemplo (1 máquina)

- Unidades/mes: **450** (15/día × 30).
- Margen de contribución mensual: **$594.000**.
- Menos operación ($100k) + infra software ($20k) + merma ($28k): **utilidad ≈ $446.000/mes**.
- CAPEX ejemplo: **$1.600.000** → **payback ≈ 3,6 meses**.

## Escenarios 1 / 5 / 20 máquinas

El escalado mejora la utilidad por máquina porque se **diluyen costos fijos** (VPS/software) y la **operación por ruta** es más eficiente; el CAPEX unitario baja por **compra por volumen**. Con los supuestos de ejemplo:

| | 1 máq | 5 máq | 20 máq |
|---|---|---|---|
| Utilidad mensual/máq | $446k | $492k | $515k |
| Utilidad mensual TOTAL | $446k | $2,46M | $10,3M |
| Payback promedio | 3,6 m | 3,3 m | 3,1 m |

## Palancas y sensibilidades a vigilar

- **Ventas/día** es la variable reina: si caen a 8/día, la utilidad se reduce a la mitad. Medir en el piloto antes de fabricar más (regla go/no-go de Finanzas).
- **Mix hacia bebidas** sube el ticket; **hacia snacks** sube el % de margen. Optimizar surtido por punto.
- **Comisión de pago:** Fase 0 (Bre-B directo) ≈ 0. Al pasar a **agregador** (Fase 2) restar ~1–2% de las ventas.
- **Riesgo regulatorio:** decreto propuesto de **retención ~1,5%** a pagos digitales (Bre-B incluido). Si prospera, es un costo directo sobre ventas — modelarlo como fila negativa. Ver [`bre-b-guia-negocio.md`](./bre-b-guia-negocio.md) §5.
- **Merma:** productos de larga vida y niveles par conservadores la mantienen baja (Operaciones 05).

## Pendiente para cerrar el modelo con datos reales

- [ ] CAPEX real del prototipo (Hardware 01 + Firmware 03 → ver `hardware/lista-compra-piloto.md`).
- [ ] Precios de venta y costos mayoristas reales (Comercial 06).
- [ ] Ventas/día medidas en 4–6 semanas de piloto (Operaciones 05).
- [ ] Confirmar tarifa **ICA** y si se entra al **RST/SIMPLE** (afecta impuestos sobre la utilidad) — ver [`tramites-cali-persona-natural.md`](./tramites-cali-persona-natural.md).
- [ ] Definir **umbral go/no-go** de payback para escalar (p. ej. < 6 meses).
