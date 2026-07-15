# Requisitos sanitarios del piloto — snacks + bebidas empacados

> Autor: Agente de Negocio · Fecha: 2026-07-14 · Producto piloto: **mixto snacks + bebidas empacados** (ADR-009)
> ⚠️ **Investigación, no asesoría profesional.** Validar con un ingeniero de alimentos / la Secretaría de Salud de Cali antes de operar.
> Relacionado: [`07-finanzas-legal.md`](../departamentos/07-finanzas-legal.md) · [`tramites-cali-persona-natural.md`](./tramites-cali-persona-natural.md) · ADR-009 en [`DECISIONS.md`](../DECISIONS.md)

---

## 0. TL;DR

1. **No fabricamos ni reenvasamos**: solo vendemos **producto empacado, sellado y rotulado de fábrica**. El **registro/permiso/notificación sanitaria (INVIMA) es del fabricante/importador**, no del revendedor. Esto es lo que hace al piloto de **baja fricción sanitaria** (razón de ADR-009).
2. **Nuestra obligación** como expendedor: vender productos **con registro sanitario vigente y rótulo conforme** (Res. 5109/2005), **respetar la vida útil / fecha de vencimiento**, y mantener condiciones de **almacenamiento** que indique el rótulo. La máquina y la operación pueden estar sujetas a **inspección/concepto sanitario** de la Secretaría de Salud municipal.
3. **Refrigeración:** **NO es necesaria en el piloto** si elegimos bebidas **estables a temperatura ambiente** (gaseosas PET/lata, agua, jugos UHT/Tetra Pak, energizantes, té). Recomendación → **piloto sin refrigeración** (resuelve la decisión pendiente y simplifica hardware/consumo). Ver §4.

---

## 1. Marco: qué exige Colombia para vender alimentos empacados

- Todo alimento que se expende al consumidor requiere **registro (RSA), permiso (PSA) o notificación sanitaria (NSA)** según su **nivel de riesgo** (**Resolución 719 de 2015** clasifica el riesgo; **Resolución 2674 de 2013** regula condiciones sanitarias). Ese trámite lo obtiene **quien fabrica o importa** el producto, y **viaja impreso en el empaque**. ([INVIMA — pasos registro/permiso/notificación](https://www.invima.gov.co/biblioteca/pasos-obtener-registro-sanitario-alimentos-invima))
- **Rotulado:** los alimentos envasados deben cumplir la **Resolución 5109 de 2005** (rótulo/etiqueta: identificación, ingredientes, lote, fecha de vencimiento, registro sanitario, responsable). Desde 2024 aplica además **rotulado frontal de advertencia** (Res. 2492/2022 / Ley 2120) que trae el fabricante. ([INVIMA — Res. 5109/2005](https://www.invima.gov.co/biblioteca/resolucion-005109-2005-requisitos-rotulado-alimentos))
- **2026 — trámite ágil:** INVIMA opera bajo **trámite automático con verificación posterior por enfoque de riesgo** (autorización inmediata al declarar cumplimiento, con auditorías selectivas). Relevante para **nuestros proveedores**, no para nosotros como revendedores. ([INVIMA 2026 — enfoque de riesgo](https://insega.co/blog/guia-paso-a-paso-requisitos-para-registro-sanitario-invima-en-2026/))

## 2. Qué implica esto para NOSOTROS (expendedor, no fabricante)

| Obligación | ¿Aplica al piloto? | Cómo lo cumplimos |
|---|---|---|
| Registro sanitario del producto | **No** (es del fabricante) | Comprar solo marcas formales con registro/rótulo visible. |
| Rótulo conforme (Res. 5109) | **Sí, indirecto** | Verificar que todo producto surtido tenga rótulo, lote y **fecha de vencimiento** legibles. |
| Vida útil / no vender vencidos | **Sí** | Control de rotación (FIFO), retirar antes del vencimiento. Ver Operaciones (05). |
| Condiciones de almacenamiento del rótulo | **Sí** | Respetar "consérvese en lugar fresco y seco"; **si el rótulo exige refrigeración, no lo vendemos** en máquina sin frío. |
| Concepto sanitario del punto/actividad | **Posible** | La **Secretaría de Salud de Cali** puede inspeccionar máquinas expendedoras de alimentos (limpieza, plagas, trazabilidad, temperaturas si hay frío). Consultar concepto/registro del expendio. |
| Manipulación de alimentos | **Mínimo** | Al ser sellado, no hay manipulación de alimento abierto. El personal de reabastecimiento debe tener **buenas prácticas** (manos limpias, no romper sellos). Curso de manipulación de alimentos recomendable (barato/gratis). |

**Conclusión:** el piloto de snacks + bebidas **empacados de fábrica** es de baja carga regulatoria: nuestra diligencia se concentra en **proveedores formales, control de vencimientos e higiene del expendio**, no en obtener registros INVIMA propios.

> **Acción recomendada:** una consulta corta a la **Secretaría de Salud Pública Municipal de Cali** para confirmar si una máquina expendedora de alimentos empacados requiere **concepto sanitario / visita** y bajo qué condiciones. Es gratis y despeja el único punto gris.

## 3. Selección de surtido de baja fricción (recomendado)

**Snacks (sellados, larga vida, ambiente):**
- Papas/pasabocas, galletas, maní/mezclas, barras de cereal/proteína, chocolatinas, chicles/mentas.
- Evitar al inicio: productos que exijan frío, artesanales sin registro, o de vida útil muy corta.

**Bebidas (ambiente, sin cadena de frío):**
- Gaseosas en **lata o PET**, **agua** embotellada, **jugos UHT / Tetra Pak**, **té**, **energizantes**, hidratantes.
- Todos **estables a temperatura ambiente** hasta abrir; el rótulo típicamente dice "consérvese en lugar fresco y seco", **no** "manténgase refrigerado".

**Evitar en el piloto (exigen frío / mayor riesgo):** lácteos refrigerados, jugos "frescos" no UHT, yogurts, sándwiches, cualquier perecedero que el rótulo obligue a refrigerar.

## 4. ¿Refrigeración en la máquina piloto? — recomendación: **NO**

**Argumento:**
- Las bebidas objetivo (gaseosa, agua, jugo UHT, energizante) son **estables a ambiente**; no requieren cadena de frío por norma ni por rótulo. La cadena de frío solo es obligatoria para alimentos que el rótulo/norma marque como refrigerados. ([Minsalud — guía inocuidad/almacenamiento](https://www.minsalud.gov.co/sites/rid/Lists/BibliotecaDigital/RIDE/VS/PP/SNA/Guia-inocuidad-alimentos-establecimientos-almacenamiento.pdf))
- Ventajas de no refrigerar en el piloto: **menor CAPEX** (sin compresor/aislamiento), **mucho menor consumo eléctrico**, mecánica más simple, menos mantenimiento y menos riesgo de falla → coherente con la tesis de "máquina de bajo costo".
- Contrapartida comercial: una **bebida fría vende más** en clima cálido (Cali). Mitigación en el piloto: elegir puntos con **nevera/tienda cercana** no es opción (competencia); mejor **medir** la venta de bebida ambiente y, **si los datos lo justifican**, evaluar una **versión con frío** en la fase de escalado (o un mini-enfriador separado). No bloquear el piloto por esto.

**Impacto en otros departamentos:**
- **Hardware (01):** diseñar el piloto **sin sistema de frío**; canal reforzado para lata/PET a temperatura ambiente. Deja la puerta abierta a un módulo de frío opcional a futuro.
- **Finanzas (07):** el CAPEX del piloto **no** incluye refrigeración (ver hoja de unit economics).
- **Decisión pendiente en `DECISIONS.md`** ("¿Refrigeración en la máquina piloto?"): **recomendación de Negocio = NO refrigerar en el piloto**. Queda para que Daniel la apruebe formalmente (posible ADR).

## 5. Checklist sanitario del piloto

- [ ] Surtir **solo** productos empacados con **registro sanitario y rótulo** de fábrica visibles.
- [ ] Verificar **fecha de vencimiento** al comprar y al reabastecer (rotación FIFO).
- [ ] Elegir bebidas/snacks **estables a temperatura ambiente** (lista §3).
- [ ] Consultar a la **Secretaría de Salud de Cali** si el expendio requiere **concepto sanitario/visita**.
- [ ] Curso básico de **manipulación de alimentos** para quien reabastece (recomendado).
- [ ] Registrar proveedores (facturas) → **trazabilidad** ante una inspección.
- [ ] Confirmar con Daniel la decisión **"piloto sin refrigeración"** y registrarla.

## Fuentes
- [INVIMA — pasos para registro/permiso/notificación sanitaria](https://www.invima.gov.co/biblioteca/pasos-obtener-registro-sanitario-alimentos-invima) · [Res. 5109/2005 rotulado](https://www.invima.gov.co/biblioteca/resolucion-005109-2005-requisitos-rotulado-alimentos) · [Otros alimentos y bebidas](https://www.invima.gov.co/productos-vigilados/alimentos/otros-alimentos-y-bebidas) · [Entidades territoriales](https://www.invima.gov.co/productos-vigilados/alimentos/entidades-territoriales)
- [Resolución 5109 de 2005 (texto, Minsalud PDF)](https://www.minsalud.gov.co/sites/rid/Lists/BibliotecaDigital/RIDE/DE/DIJ/Resolucion%205109%20de%202005.pdf)
- [Minsalud — guía inocuidad / almacenamiento](https://www.minsalud.gov.co/sites/rid/Lists/BibliotecaDigital/RIDE/VS/PP/SNA/Guia-inocuidad-alimentos-establecimientos-almacenamiento.pdf)
- [INVIMA 2026 — enfoque de riesgo (resumen)](https://insega.co/blog/guia-paso-a-paso-requisitos-para-registro-sanitario-invima-en-2026/)
