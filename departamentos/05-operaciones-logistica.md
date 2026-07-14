# 05 · Operaciones / Logística

**Responsable:** Daniel + agente de IA de operaciones (planeación de rutas, inventario, alertas).
**Misión:** Que cada máquina esté **ubicada en un buen punto, surtida y funcionando**, con el menor costo y tiempo operativo posible.

---

## 1. De qué se encarga

- Selección y negociación de ubicaciones (junto con [Comercial](./06-comercial-marketing.md)).
- Instalación y anclaje de máquinas.
- Reabastecimiento (qué, cuándo, cuánto).
- Control de inventario (físico vs. sistema).
- Mantenimiento preventivo y correctivo.
- Rutas y logística de visita a máquinas.

## 2. Ubicaciones (el factor #1 de éxito de una vending)

La rentabilidad de una vending depende **más de la ubicación que de la máquina**. Criterios de un buen punto:

- **Tráfico cautivo**: gimnasios, universidades, conjuntos residenciales, oficinas, coworkings, talleres, salas de espera, canchas.
- **Poca competencia** de tiendas cercanas abiertas 24/7.
- **Seguridad** (evita vandalismo/robo).
- **Energía disponible** y espacio techado.
- **Acceso fácil** para reabastecer.

Para el **piloto**: conseguir **1 punto excelente** donde puedas medir de verdad (idealmente con dueño de espacio dispuesto a colaborar).

## 3. Reabastecimiento e inventario

- **Regla simple MVP:** cada máquina reporta stock en el panel (se descuenta con cada venta). Alertar cuando un canal baje de un umbral.
- Definir **par mínimo/máximo** por producto y punto según rotación.
- Registrar **mermas** (vencimiento, atascos, fallos) para ajustar.
- Con más máquinas: el agente de IA arma la **ruta óptima** de reabastecimiento por proximidad y urgencia de stock.

## 4. Mantenimiento

- **Preventivo:** revisar motores, sensores, lector QR y limpieza en cada visita.
- **Correctivo:** protocolo ante "pagué y no cayó" (registro por sensor de dispensado → reembolso/reposición).
- **Monitoreo:** aunque la venta es offline, evaluar que la máquina reporte "estoy viva" y su stock **cuando tenga wifi ocasional** (no es requisito para vender, pero ayuda a operar). Opcional en MVP.

## 5. Tareas — Fase MVP

- [ ] Definir criterios y checklist de evaluación de puntos.
- [ ] Conseguir el punto piloto (con [Comercial](./06-comercial-marketing.md)).
- [ ] Instalar y anclar la máquina; verificar energía y prueba end-to-end en sitio.
- [ ] Definir surtido inicial y niveles par por producto.
- [ ] Diseñar el flujo de reabastecimiento (frecuencia, checklist, registro de merma).
- [ ] Protocolo de fallo de dispensado y reembolso.
- [ ] Medir durante 4–6 semanas: ventas/día, productos top, incidencias.

## 6. Entregables

- Checklist de evaluación de ubicaciones.
- SOP (procedimiento estándar) de reabastecimiento y de mantenimiento.
- Tablero de stock e incidencias por máquina.

## 7. KPIs

- **Ventas por máquina por día** (métrica reina).
- **Tasa de disponibilidad** (% tiempo con stock y funcionando).
- **Agotados (stockouts)** por semana (minimizar).
- **Costo operativo por visita** y visitas/semana.
- **Merma** (% de producto perdido).

## 8. Riesgos y mitigación

- **Mala ubicación** → no comprometerse a largo plazo sin datos; empezar con acuerdos flexibles.
- **Robo/vandalismo** → puntos vigilados, anclaje, cámara del sitio.
- **Sobre-stock / vencimientos** → niveles par conservadores al inicio; productos de larga vida.

## 9. Dependencias

- **Con Comercial:** cierre de puntos y condiciones con el dueño del espacio.
- **Con Software:** panel de stock e incidencias.
- **Con Finanzas:** costos operativos alimentan el unit economics.
