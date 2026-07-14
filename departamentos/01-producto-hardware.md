# 01 · Producto / Hardware mecánico

**Responsable:** Daniel + agente de IA de diseño/CAD (apoyo en Revit/FreeCAD, cálculos, BOM).
**Misión:** Diseñar y construir una máquina expendedora **funcional, segura y lo más barata posible**, empezando por 1 prototipo que dispense de forma fiable.

---

## 1. De qué se encarga

Todo lo físico de la máquina: estructura, mecanismo de dispensado, alimentación eléctrica, gabinete, seguridad física (candado/chapa), integración de la electrónica dentro del mueble, y la lista de materiales (BOM) con costos.

No se encarga de: el código del ESP32 (Dept. 03) ni la web (Dept. 02), pero **coordina la interfaz física** con ellos (dónde va el lector, los motores, la fuente).

## 2. Decisiones clave de diseño

### Mecanismo de dispensado (la decisión más importante de costo)

| Opción | Costo | Fiabilidad | Recomendación |
|--------|-------|-----------|---------------|
| **Espiral / coil** (el clásico de vending) | Medio-alto | Alta, versátil | Estándar de la industria; motor DC + espiral por canal. Buen punto medio. |
| **Gravedad + compuerta** (productos caen por peso) | **Bajo** | Media (atascos) | Ideal para MVP con productos uniformes (latas, botellas pequeñas, snacks rígidos). |
| **Cinta/empujador** | Medio | Alta | Más complejo de fabricar. |

**Recomendación MVP:** empezar con **espiral (coil)** de un solo tipo de producto por canal, motor DC 12V con gearbox + microswitch de fin de giro para confirmar dispensado. Es el patrón más documentado y con repuestos baratos. Si el producto lo permite, evaluar gravedad para bajar costo.

### Confirmación de entrega (anti "pagué y no cayó")

Cada canal debe tener un sensor que confirme que el producto salió:
- **Microswitch de vuelta completa** del espiral (barato), o
- **Sensor óptico/infrarrojo** en la boca de salida (más fiable, ~detecta caída).

Esto es crítico para la confianza del cliente y para la lógica de reembolso/registro.

### Estructura y gabinete

- **MVP barato:** estructura en **lámina metálica calibre 20–22** o **madera/MDF reforzado** para el primer prototipo (más barato y fácil de iterar), migrando a metal para producción.
- Puerta frontal con **chapa/candado** y bisagras. Vidrio o acrílico si se quiere exhibir producto (opcional en MVP).
- Compartimento sellado para electrónica (ESP32, fuente, drivers).

### Alimentación eléctrica

- Fuente conmutada **12V** (para motores) + regulador a **5V/3.3V** (ESP32 y lector).
- Toma de corriente estándar 110V. Protección: fusible + interruptor.

## 3. BOM objetivo (prototipo — orden de magnitud, a validar con cotización)

> Los precios son estimados para ubicar el rango; el agente de IA debe cotizar en proveedores reales (MercadoLibre CO, tiendas de electrónica, ferreterías, importación).

| Componente | Cant. | Notas |
|-----------|-------|-------|
| Motores DC 12V + espirales | 4–6 | 1 por canal en el MVP |
| Drivers de motor (ULN2003 / L298N / MOSFET) | 1–2 | Según nº de canales |
| Microswitches / sensores de salida | 4–6 | Confirmación de entrega |
| ESP32 | 1 | Ver Dept. 03 |
| Lector QR (GM65 o similar) | 1 | Ver Dept. 03 |
| Fuente 12V + regulador 5V | 1 | Dimensionar por consumo de motores |
| Estructura (lámina/MDF, tornillería, chapa) | 1 | El grueso del costo físico |
| Cableado, borneras, fusible, interruptor | 1 | — |

**Meta:** definir un **costo total de máquina** objetivo y compararlo contra una vending comercial usada (para el argumento comercial). Registrar el número real en [Finanzas](./07-finanzas-legal.md).

## 4. Tareas — Fase MVP (1 prototipo)

- [ ] Elegir **producto piloto** (define tamaño de canal y mecanismo). Coordinar con [Comercial](./06-comercial-marketing.md).
- [ ] Bocetar el mecanismo de dispensado y validar con 1 canal de prueba (dispensar 20 veces sin fallo).
- [ ] Diseño CAD del gabinete (Revit/FreeCAD/Fusion) con espacio para electrónica.
- [ ] Cotizar BOM real y cerrar costo del prototipo.
- [ ] Construir prototipo de 1 máquina con 4–6 canales.
- [ ] Definir interfaz física con Dept. 03 (posición de lector, conexión de motores, sensores).
- [ ] Prueba de dispensado end-to-end (con firmware): 100 dispensados, medir tasa de fallo.

## 5. Entregables

- Diseño CAD + planos de fabricación.
- BOM con costos reales y proveedores.
- Prototipo físico funcional.
- Manual de ensamblaje (para replicar máquinas 2..N).

## 6. KPIs

- **Tasa de dispensado exitoso** ≥ 99% (objetivo; medir en prototipo).
- **Costo por máquina** (bajarlo cada iteración).
- **Tiempo de ensamblaje** por máquina (para escalar).

## 7. Riesgos y mitigación

- **Atascos de producto** → elegir producto uniforme; sensor de confirmación; diseño de canal generoso.
- **Costo se dispara** → mantener diseño simple, pocos canales al inicio, materiales locales.
- **Seguridad física (robo/vandalismo)** → chapa robusta, anclaje, ubicación en punto vigilado (coordina con [Operaciones](./05-operaciones-logistica.md)).

## 8. Dependencias

- **Con Dept. 03 (Firmware):** protocolo eléctrico de activación de motores y lectura de sensores.
- **Con Comercial:** qué producto y qué formato se venderá primero.
- **Con Finanzas:** costo de máquina alimenta el unit economics.
