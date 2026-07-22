# Investigación de costos de producto — GRABI M001 (2026-07-22)

> Insumo para `unit-economics-grabi-m001.xlsx` (hoja **Fuentes**). Precios reales de fuentes
> públicas 2026. **Regla clave:** D1 es hard-discount y **NO vende al por mayor** → su precio de
> góndola **ES tu costo** (ya es de los más bajos, pero no hay descuento por volumen). Para volumen
> real: Makro o mayorista de barrio.

## Hallazgo transversal (importante)

**D1 casi no maneja porción individual pequeña** (bolsas de 22–50 g), que es el formato ideal para
una expendedora de espiral. Su surtido son bolsas familiares (80–454 g) y botellas de 1 L+. Por eso
los costos realistas de porción individual salen de **retail/mayorista de otras cadenas** y **deben
confirmarse en tienda física**. Las celdas de costo del Excel quedan editables para eso.

## Precios por categoría

### Agua ~600 ml
| Producto | Tamaño | Precio | Fuente |
|---|---|---|---|
| Agua Omi con gas (D1) | 600 ml | **$890** | [losprecios.co D1](https://losprecios.co/d1_t1/bebidas/aguas_s18) |
| Agua Omi botella (D1) | 1.000 ml | $950 | [losprecios.co D1](https://losprecios.co/d1_t1/bebidas/aguas_s18) |
| Agua Cristal (paca x24) | 600 ml c/u | **~$630/u** | [Éxito](https://www.exito.com/agua-ahorra-pack-pet-845228/p) |
| Agua Cristal | 600 ml | $1.650 | [Megatiendas](https://www.megatiendas.co/agua-cristal-sin-gas-x-600-ml-7702090022711/p) |
| Agua Cristal | 600 ml | $2.200 | [OXXO](https://colombia.oxxodomicilios.com/product-details/agua-purificada-cristal-600ml/01H1Y71ZTVND8EWVXHS05EHMWQ) — techo de precio de venta |
| ⚠️ Agua Omi **sin gas** 600 ml | 600 ml | **NO listada** | Confirmar en tienda D1 |

**Costo usado en el modelo:** $800 (editable). Rango real esperado: $630 (paca) – $890 (D1 unidad).

### Snack salado económico (platanitos / Yupi)
| Producto | Tamaño | Precio | Fuente |
|---|---|---|---|
| Papas Kythos (D1) | 115 g | $3.400 | [losprecios.co D1](https://losprecios.co/d1_t1/dulces-y-pasabocas/paquetes_s45) |
| Platanitos Plataitos (D1) | 80 g | $4.250 | [losprecios.co D1](https://losprecios.co/d1_t1/dulces-y-pasabocas/paquetes_s45) |
| Yupi Clásica (pack x12) | 22 g c/u | **~$975/u** | [Múcura mayorista](https://mucura.co/bogota/marca/yupi) |
| Margarita Papas Natural | individual | $2.010 | [Rappi](https://www.rappi.com.co/p/margarita-papas-fritas-natural-101993) |
| ⚠️ Platanitos indiv. pequeño (~25–30 g) | ~25–30 g | **NO confirmado** | Confirmar en tienda |

**Costo usado en el modelo:** $1.000 (editable). Yupi 22 g mayorista ~$975 es la mejor referencia.

### Maní individual
| Producto | Tamaño | Precio | Fuente |
|---|---|---|---|
| Maní Salado Nuthos (D1) | 200 g | $3.950 | [losprecios.co D1](https://losprecios.co/d1_t1/dulces-y-pasabocas/frutos-secos_s48) |
| La Especial Maní con Sal | 50 g | **$1.700** | [Rappi](https://www.rappi.com.co/p/la-especial-mani-con-sal-2735880) |
| Maní Moto Natural | 80 g | $3.190 (~$1.600/40g) | [Rappi/Éxito](https://www.rappi.com.co/p/mani-moto-snack-natural-80-g-176158) |
| ⚠️ Maní Nuthos indiv. (~40–50 g) | ~40–50 g | **NO listado** | Confirmar en tienda D1 |

**Costo usado en el modelo:** $1.400 (editable). Bajará si se consigue mayorista o D1 en bolsita.

### Energizante (Speed Max / Vive 100)
| Producto | Tamaño | Precio | Fuente |
|---|---|---|---|
| ⚠️ Energizante marca propia D1 | n/d | **categoría VACÍA en losprecios.co** | Confirmar en tienda D1 |
| Speed Max | 250 ml | desde $1.820 | [Rappi](https://www.rappi.com.co/p/speed-max-bebida-energizante-250-ml-14689) |
| Speed Max | 400 ml | $2.030–2.450 | [Rappi (Éxito/Makro)](https://www.rappi.com.co/p/speed-max-bebida-energizante-400-ml-101763) |
| Vive 100 Original | 380 ml | $2.100 (Makro) – $2.850 (Locatel) | [Locatel](https://www.locatelcolombia.com/7702354934361-bebida-vive-100-original-energizante-x-380ml/p) |

**Costo usado en el modelo:** $1.900 (Speed Max 400 ml, editable). Vive 100 subiría a ~$2.100+.

### Dulce (referencia, no en la canasta base)
| Producto | Tamaño | Precio | Fuente |
|---|---|---|---|
| Chocolatina Jet (D1) | 22 g | $2.000 | [losprecios.co D1](https://losprecios.co/d1_t1/dulces-y-pasabocas/chocolatinas_s47) |

## Pendientes para Daniel (confirmar en tienda física)

1. **Energizante D1** (marca propia): nombre y precio — categoría vacía en losprecios.co.
2. **Agua Omi sin gas 600 ml**: precio real (probable ~$800–1.000).
3. **Porciones individuales** (22–50 g) de maní y snack salado, y bebidas personales — el formato de vending.
4. **Mercamío (Cali):** no consultable por Rappi (SPA con login). Usar app logueada con dirección de Cali o visita presencial.
5. **Estructura mecánica (socio):** cotizar el costo real — es el rubro que más mueve el payback en el Excel.

---
*Esto es investigación de precios públicos, no una cotización en firme. Cerrar costos con compra real antes de fijar precios definitivos.*
