# 🧭 Plan Maestro — Proyecto Dispensadoras

> Documento de gerencia. Une los departamentos, define el rumbo, el orden de ejecución y **cómo tú (Daniel) diriges la empresa apoyado en agentes de IA**. Léelo como el mapa; cada [departamento](./README.md) es el detalle de cada territorio.

**Fecha:** 14 de julio de 2026 · **Fase:** 0→1 (definición + piloto de 1 máquina).

---

## 1. Visión

Construir una red de máquinas expendedoras **radicalmente baratas** eliminando lo más costoso y frágil de una vending tradicional (billetero, monedero, datáfono), aprovechando los **pagos digitales inmediatos de Colombia (Bre-B)** y una arquitectura **offline y segura por criptografía**. El menor costo por máquina permite rentabilizar puntos donde una vending normal no llega, y escalar rápido.

**Ventaja competitiva:**
1. **Costo de máquina bajo** → más puntos viables, mejor trato al dueño del espacio.
2. **Pago digital nativo** → sin manejo de efectivo, sin arqueos, sin robos de caja.
3. **Offline + firma criptográfica** → funciona sin internet en la máquina, imposible de falsificar sin la llave privada.

## 2. Cómo funciona (resumen técnico)

Web por máquina (`dominio.com/ID`) → cliente paga por Bre-B → servidor confirma pago y **firma un token (JWT Ed25519)** con los productos → se muestra como **QR** → la máquina lo **verifica offline** con su llave pública, comprueba `machine_id`, expiración y anti-reuso (`jti`), y **dispensa**. Detalle en [Software](./departamentos/02-software-web.md) y [Firmware](./departamentos/03-firmware-electronica.md).

## 3. Organización: tú + agentes de IA

Eres la **gerencia general**. Cada "departamento" es un **rol** que operas con ayuda de uno o más agentes/asistentes de IA especializados. No necesitas empleados para empezar; necesitas **disciplina de proceso**.

| Departamento | Tu rol | El agente de IA hace |
|--------------|--------|----------------------|
| [01 Producto/Hardware](./departamentos/01-producto-hardware.md) | Construir y decidir mecánica | CAD, cálculos, cotización de BOM, manuales |
| [02 Software/Web](./departamentos/02-software-web.md) | Dirigir arquitectura, revisar | Escribir el backend Go, front, deploy, simulador |
| [03 Firmware/Electrónica](./departamentos/03-firmware-electronica.md) | Armar y probar | Código ESP32, integración cripto, esquemas |
| [04 Pagos](./departamentos/04-pagos.md) | Decidir estrategia, abrir cuentas | Conciliación por correo, comparar agregadores |
| [05 Operaciones](./departamentos/05-operaciones-logistica.md) | Visitar puntos, surtir | Rutas, alertas de stock, SOPs |
| [06 Comercial/Marketing](./departamentos/06-comercial-marketing.md) | Cerrar puntos, vender | One-pagers, guiones, marca, contenido |
| [07 Finanzas/Legal](./departamentos/07-finanzas-legal.md) | Decidir, validar con profesional | Modelo financiero, investigación legal, plantillas |

**Regla de oro:** tú tomas decisiones y ejecutas lo físico/relacional; la IA produce, investiga y automatiza. Delega todo lo delegable a la IA para mantener el ritmo siendo una sola persona.

## 4. Ruta crítica (qué bloquea a qué)

El corazón técnico es la **interfaz token entre Software (02) y Firmware (03)**. Casi todo lo demás puede avanzar en paralelo. La cadena que no puede fallar:

```
Contrato del token (02+03) ──► Firma en servidor (02) ──► Verificación en ESP32 (03)
                                                              │
        Mecanismo de dispensado (01) ─────────────────────────┤
                                                              ▼
                              Máquina que dispensa con un QV válido
                                                              │
   Cobro por Bre-B (04) ──► Web genera QR real (02) ──────────┤
                                                              ▼
          Punto conseguido (06) + instalada y surtida (05) = PILOTO VENDIENDO
                                                              │
                    Medir unit economics (07) ──► decisión go/no-go de escalar
```

**Primero acordar el contrato del token.** Es la pieza que, si se define bien una vez, desbloquea a Software y Firmware para trabajar en paralelo.

## 5. Roadmap por fases

### Fase 0 — Definición y contrato (semana 0–1)
- Cerrar el **contrato del token** (02+03): campos, Ed25519, encoding, tamaño de QR, manejo de reloj (RTC).
- Elegir **producto piloto** (01+06) → define mecanismo y canales.
- Abrir **llave Bre-B de negocio** y decidir figura jurídica inicial (07).
- **Hito 0:** documento del token firmado + producto piloto elegido.

### Fase 1 — Prototipos en paralelo (semana 1–5)
- **Software:** web `/m/ID`, firma de token, generación de QR, simulador de verificación, deploy barato (02).
- **Firmware:** GM65 leyendo → ESP32 verificando token de prueba → activando 1 motor con sensor (03).
- **Hardware:** 1 canal dispensando fiable → gabinete con 4–6 canales (01).
- **Pagos:** conciliación por correo funcionando en pruebas (04).
- **Hito 1:** demo end-to-end en banco de pruebas (pago simulado → QR → máquina dispensa).

### Fase 2 — Integración y máquina piloto (semana 5–8)
- Unir todo en la máquina física; pruebas de estrés (100+ dispensados).
- Señalización, marca mínima y soporte por WhatsApp (06).
- Cerrar el **punto piloto** (06+05).
- **Hito 2:** máquina instalada en un punto real, vendiendo a clientes reales.

### Fase 3 — Operación y validación (semana 8–14)
- Medir ventas/día, conversión en punto, incidencias, unit economics reales (05+07).
- Iterar sobre fricción de pago y fallos de dispensado.
- Evaluar agregador Bre-B con webhook (04) si el volumen lo pide.
- **Hito 3 (go/no-go):** ¿payback y ventas/día cumplen el umbral? → decidir escalar.

### Fase 4 — Escalado (post-validación)
- Bajar costo por máquina, estandarizar ensamblaje, fabricar 2..N.
- Integración oficial Bre-B (webhooks) + posible pasarela.
- Sistematizar prospección de puntos y rutas de reabastecimiento.

> Los tiempos son estimados para una persona con apoyo de IA; ajústalos a tu disponibilidad real.

## 6. Cómo gestionar el día a día (operativa de gerencia)

- **Un backlog por departamento**: las tareas ya están listadas en cada archivo. Trabaja por **hitos**, no por departamento aislado.
- **Cadencia semanal**: cada semana define 3–5 tareas de la ruta crítica, ejecútalas con los agentes, y actualiza el estado.
- **Revisión de hitos**: no pases de fase sin cerrar el hito anterior. Los hitos son tus "puertas".
- **Fuente única de verdad**: el contrato del token vive en un solo documento versionado (evita que Software y Firmware se desincronicen).
- **Decisiones registradas**: cuando elijas algo importante (mecanismo, algoritmo, agregador), anótalo con la razón. Sirve para no re-discutir y para que los agentes tengan contexto.
- **Herramientas sugeridas**: este repo de `.md` como cerebro; una hoja de cálculo para finanzas; un tablero simple (o los mismos `.md`) para el backlog. Puedes pedirme convertir cualquier plan en tareas accionables o en un tablero.

## 7. Riesgos globales y mitigación

| Riesgo | Impacto | Mitigación |
|--------|---------|-----------|
| **Adopción del pago digital** (cliente no sabe/no quiere) | Alto | Instrucciones ultra simples, promo primer uso, soporte visible, QR estático que lleva directo a la web. |
| **Fricción "pagué y no cayó"** | Alto | Sensor de dispensado + reembolso rápido + WhatsApp de soporte a la vista. |
| **Contrato token desincronizado (02↔03)** | Alto | Un solo documento versionado; simulador de verificación en Software. |
| **Mala ubicación del piloto** | Alto | Criterios estrictos de punto; acuerdo flexible; medir antes de comprometerse. |
| **Requisitos sanitarios de alimentos** | Medio-alto | Resolver sanidad antes de vender comida; considerar producto de baja exigencia al inicio. |
| **Costo de máquina se dispara** | Medio | Diseño simple, pocos canales, materiales locales; el bajo costo es la tesis del negocio. |
| **Conciliación por correo frágil** | Medio | Monto único por orden; migrar a QR dinámico y webhook oficial pronto. |
| **Fraude con captura de pantalla de pago** | Medio | Confiar solo en la notificación real de tu cuenta, nunca en el comprobante que muestra el cliente. |

## 8. Criterios de éxito (cómo saber que vas bien)

- **Fase 1–2:** demo end-to-end funciona; máquina dispensa fiable (≥99%).
- **Fase 3:** ventas/día sostenidas en el punto piloto; conversión razonable de quienes se paran frente a la máquina; unit economics con payback dentro del umbral que definas en [Finanzas](./departamentos/07-finanzas-legal.md).
- **Fase 4:** costo por máquina baja con el volumen; puedes replicar puntos con un guion probado.

## 9. Próximos pasos inmediatos (esta semana)

1. **Cerrar el contrato del token** (te lo puedo redactar como documento técnico versionado).
2. **Elegir el producto piloto** (define mecánica y canales).
3. **Decidir figura jurídica** y abrir **llave Bre-B de negocio**.
4. **Arrancar el simulador de verificación** en Software para probar sin hardware.
5. Pedirme: (a) el esqueleto del backend Go, (b) la hoja de unit economics, o (c) el kit de ventas B2B — lo que quieras atacar primero.

---

### Índice de departamentos
- [01 · Producto / Hardware mecánico](./departamentos/01-producto-hardware.md)
- [02 · Software / Web](./departamentos/02-software-web.md)
- [03 · Firmware / Electrónica](./departamentos/03-firmware-electronica.md)
- [04 · Pagos (Bre-B)](./departamentos/04-pagos.md)
- [05 · Operaciones / Logística](./departamentos/05-operaciones-logistica.md)
- [06 · Comercial / Marketing](./departamentos/06-comercial-marketing.md)
- [07 · Finanzas y Legal](./departamentos/07-finanzas-legal.md)
