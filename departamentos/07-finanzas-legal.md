# 07 · Finanzas y Legal / Regulatorio

**Responsable:** Daniel + agente de IA de finanzas (modelado, unit economics) y de investigación legal.
**Misión:** Saber **si cada máquina gana dinero y en cuánto tiempo**, controlar el gasto del piloto, y mantener el negocio **formal y en regla** desde el inicio sin sobre-invertir.

> ⚠️ Este documento es orientativo, no es asesoría legal ni contable profesional. Antes de constituir la empresa y para temas tributarios/sanitarios, validar con un contador y/o abogado en Colombia.

---

## PARTE A — FINANZAS

## 1. De qué se encarga

Costeo, precios, unit economics por máquina, presupuesto del piloto, punto de equilibrio, flujo de caja y decisión de cuándo escalar.

## 2. Unit economics por máquina (plantilla)

**Inversión inicial por máquina (CAPEX):**
```
Costo de la máquina (BOM + ensamblaje) ............ $ ____   (de Dept. 01)
Electrónica (ESP32, GM65, RTC, fuente) ............ $ ____   (de Dept. 03)
Instalación / anclaje ............................. $ ____
TOTAL CAPEX por máquina ........................... $ ____
```

**Por unidad vendida:**
```
Precio de venta .................................... $ ____
(-) Costo del producto (mayorista) ................. $ ____
(-) Comisión de pago (Bre-B directo ≈ 0; agregador/pasarela = %) $ ____
(-) Prorrateo de espacio (si hay comisión/renta) ... $ ____
= Margen de contribución por unidad ................ $ ____
```

**Mensual por máquina:**
```
Ventas/día × 30 = unidades/mes ..................... ____
Margen de contribución mensual ..................... $ ____
(-) Costo de reabastecimiento/operación ............ $ ____
(-) Renta del espacio (si aplica) .................. $ ____
(-) Prorrateo infra software (VPS, dominio) ........ $ ____
= Utilidad mensual por máquina ..................... $ ____

Payback (meses) = CAPEX por máquina ÷ utilidad mensual
```

> El agente de IA convierte esto en una **hoja de cálculo** viva. Los números reales vienen de: BOM (Dept. 01), electrónica (Dept. 03), comisiones (Dept. 04), ventas/día (Dept. 05), precios/mayorista (Dept. 06).

## 3. Presupuesto del piloto

Con presupuesto para **1 prototipo**, controlar que el gasto se concentre en: (1) construir 1 máquina fiable, (2) software mínimo desplegado barato, (3) surtido inicial. Todo lo demás (marca, prospección, conciliación) es tiempo + IA, casi sin costo monetario.

## 4. Punto de equilibrio y decisión de escalar

- **Break-even del piloto:** cuándo la utilidad acumulada cubre el CAPEX de la 1ª máquina.
- **Señal para escalar:** el piloto muestra ventas/día y payback aceptables (definir umbral, p. ej. payback < X meses) **antes** de fabricar máquinas 2..N.
- Modelar escenarios: 1, 5, 20 máquinas — costos que bajan por volumen vs. costos operativos que crecen.

## 5. Tareas — Fase MVP

- [ ] Construir la hoja de unit economics (con celdas para llenar desde los otros departamentos).
- [ ] Fijar precios de venta con Comercial.
- [ ] Registrar CAPEX real del prototipo.
- [ ] Medir ventas/día reales en el piloto y calcular payback real.
- [ ] Modelar escenarios de escalado (5 y 20 máquinas).
- [ ] Definir umbral de decisión "go/no-go" para escalar.

## 6. KPIs

- **Margen de contribución por unidad** y por máquina/mes.
- **Payback por máquina** (meses).
- **Burn** del piloto vs. presupuesto.
- **Utilidad por máquina/mes** una vez estabilizada.

---

## PARTE B — LEGAL / REGULATORIO (Colombia)

> Investigar y confirmar con profesional. El agente de IA prepara la investigación y los borradores; la validación final es humana/profesional.

## 7. Temas a resolver

- **Figura jurídica**: empezar como **persona natural** (más simple/barato) vs. constituir **SAS** (protege patrimonio, da formalidad, útil para crecer y para acuerdos B2B). Decidir según ritmo de crecimiento.
- **Registro mercantil** (Cámara de Comercio) y **RUT/DIAN** con las actividades económicas correctas (comercio al por menor / máquinas expendedoras).
- **Facturación electrónica DIAN**: obligación de facturar; evaluar régimen. Para vending esto tiene particularidades (ticket, facturación por demanda) — confirmar.
- **Cuenta y llave Bre-B de negocio**: recibir pagos en una cuenta **del negocio**, no personal. Idealmente **código de comercio** como llave. Coordinar con [Pagos](./04-pagos.md).
- **Sanidad / INVIMA**: si se venden **alimentos/bebidas**, requisitos de manipulación, rotulado, cadena de conservación, y permisos según el producto. Es el punto legal más sensible para vending de comida.
- **Uso del espacio**: contrato/acuerdo con el dueño del punto (comodato, arriendo o comisión) por escrito.
- **Protección de datos (Habeas Data, Ley 1581)**: la web maneja datos mínimos; publicar política de privacidad y tratar datos conforme a la ley.
- **Impuestos locales**: ICA municipal, y otros según ciudad.

## 8. Tareas — Fase MVP

- [ ] Decidir figura jurídica inicial (persona natural vs. SAS) con base en costo y plan de crecimiento.
- [ ] Abrir RUT con actividades correctas; registro mercantil si aplica.
- [ ] Resolver facturación electrónica (asesoría contable).
- [ ] Verificar requisitos sanitarios del/los producto(s) piloto (clave si es alimento).
- [ ] Redactar el acuerdo de uso de espacio (plantilla).
- [ ] Publicar política de privacidad/tratamiento de datos en la web.
- [ ] Abrir cuenta y llave Bre-B del negocio.

## 9. Entregables

- Hoja de unit economics y modelo de escenarios.
- Checklist legal con estado (pendiente/hecho) y responsable.
- Plantillas: acuerdo de espacio, política de datos.

## 10. Riesgos y mitigación

- **Vender alimentos sin cumplir sanidad** → resolver requisitos ANTES de vender comida; empezar con producto de menor exigencia si es posible.
- **Informalidad al recibir pagos** → cuenta/llave de negocio y facturación desde el piloto.
- **Números que no cierran** → no escalar hasta que el piloto valide el payback.

## 11. Dependencias

- Recibe datos de **todos** los departamentos (es el que mide la salud del negocio).
- **Con Pagos:** cuenta de negocio, comisiones, facturación.
- **Con Comercial/Operaciones:** precios, ventas, costos operativos.
