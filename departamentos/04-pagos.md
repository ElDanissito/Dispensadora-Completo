# 04 · Pagos (Bre-B) e integración

**Responsable:** Daniel + agente de IA de integraciones/conciliación.
**Misión:** Cobrar de la forma más barata y confiable posible usando **Bre-B**, y confirmar el pago para que la web pueda emitir el QR. Evolucionar de conciliación semi-manual a integración oficial.

---

## 1. Contexto: qué es Bre-B (por qué encaja perfecto)

Bre-B es el **Sistema de Pagos Inmediatos Interoperado** del Banco de la República (operando desde oct-2025). Permite transferencias **inmediatas** (máx. 20 s), interoperables entre bancos y billeteras, usando **"llaves"** (celular, cédula, correo, código alfanumérico o **código de comercio**). A ene-2026 ya tenía 218 entidades vinculadas y cientos de millones de operaciones. Límite por operación: 1.000 UVB.

**Lo más relevante para nosotros:** Bre-B soporta **QR dinámicos** (uno por transacción con **monto específico**) que el cliente escanea desde su app bancaria, paga y recibe **confirmación inmediata**, sin redirección. Eso encaja exactamente con el flujo "paga desde el celular".

> Fuentes al pie del documento.

## 2. Estrategia por fases (de lo barato a lo integrado)

### Fase 0 — MVP: conciliación semi-automática por notificación/correo (lo que pediste)

El cliente paga por Bre-B a **tu llave** (tu celular/cédula/código de comercio). Tú recibes una **notificación** (correo o push del banco). Un proceso automático concilia:

- **Cómo verificar el pago:** un servicio (agente/script) lee el **correo de notificación** de tu banco (vía API de Gmail/IMAP) y extrae **remitente + monto + hora + referencia**. Si coincide con una orden pendiente (monto exacto + ventana de tiempo), marca la orden como pagada y dispara la emisión del QR.
- **Truco clave para conciliar sin ambigüedad:** hacer que cada orden pida un **monto único** (p. ej. añadir centavos aleatorios: $2.347, $2.348...) o pedir al cliente una **referencia/nota** en la transferencia. Así el matching monto↔orden es inequívoco aunque lleguen varios pagos parecidos.
- **Riesgos de esta fase:** depende de que el correo del banco llegue y sea parseable; latencia variable; el cliente debe hacer una transferencia manual. Es aceptable para **validar el negocio** con pocas máquinas, no para escalar.

> ⚠️ Nota legal/operativa: recibir pagos en una cuenta personal escala mal y complica la contabilidad. Ver [Finanzas/Legal](./07-finanzas-legal.md). Usar desde el inicio una **cuenta/llave dedicada al negocio**.

### Fase 1 — QR dinámico Bre-B

Generar en la web un **QR dinámico de Bre-B con el monto exacto** de la orden. El cliente lo escanea desde su app bancaria y paga. Reduce fricción (no teclea monto ni destinatario) y hace el matching más limpio. La confirmación puede seguir llegando por notificación mientras no haya API.

### Fase 2 — Integración oficial vía agregador (recaudos con webhook)

Integrarse con un **proveedor/orquestador Bre-B** que ofrezca **API de recaudos y webhooks de confirmación**: cuando el pago se acredita, tu backend recibe un **webhook** inmediato y emite el QR automáticamente. Candidatos a evaluar (existen en el mercado colombiano): **Mono, Cobre, MOVii, Minka/Payments Hub, nodos como Visionamos/Passport**. Esto elimina el parseo de correos y es lo que permite **escalar con confianza**.

### Fase 3 (opcional) — Pasarela de pagos general

Añadir una **pasarela** (Wompi, Mercado Pago, ePayco) para aceptar también **tarjetas, PSE y otras billeteras**. Más comisión, pero amplía cobertura. Útil cuando el volumen lo justifique.

## 3. Contrato con el resto del sistema

- **Entrada:** una orden pendiente (id, monto, machine_id, items) creada por [Dept. 02](./02-software-web.md).
- **Salida:** evento "orden PAGADA" (con referencia del pago) que dispara la firma del JWT y la generación del QR.
- El módulo de pagos **no** genera el QR de dispensado; solo confirma el pago. Mantener esa separación.

## 4. Tareas — Fase MVP

- [ ] Abrir/registrar **llave Bre-B dedicada al negocio** (idealmente código de comercio; ver Legal para figura jurídica).
- [ ] Definir el mecanismo de conciliación: monto único por orden **o** referencia obligatoria.
- [ ] Construir el lector de notificaciones (API Gmail/IMAP) que extrae remitente/monto/hora/referencia.
- [ ] Motor de matching orden↔pago con ventana de tiempo y tolerancia 0 en monto.
- [ ] Manejo de casos borde: pago tardío, monto incorrecto, pago duplicado, orden expirada → política de reembolso/soporte.
- [ ] Registro/auditoría de todos los pagos para conciliación contable.
- [ ] Investigar y cotizar **agregadores Bre-B** para Fase 2 (requisitos, comisiones, tiempos de integración).

## 5. Entregables

- Servicio de conciliación funcionando (Fase 0).
- Documento de comparación de agregadores Bre-B (comisión, API, requisitos, onboarding).
- Política de reembolsos y manejo de disputas.

## 6. KPIs

- **Tiempo pago→QR** (objetivo Fase 0 < 30 s; Fase 2 casi instantáneo).
- **Tasa de conciliación automática** (% de pagos casados sin intervención). Objetivo > 98%.
- **Costo por transacción** (Bre-B directo ≈ muy bajo/gratis P2P; agregador y pasarela cobran comisión — medir).
- Pagos no conciliados / disputas por semana (minimizar).

## 7. Riesgos y mitigación

- **Correo del banco no parseable o con retraso** → migrar a QR dinámico y luego a webhook oficial cuanto antes.
- **Cliente paga monto equivocado** → monto único + instrucciones claras + flujo de corrección.
- **Fraude "captura de pantalla de pago"** → nunca confiar en el comprobante que muestra el cliente; **solo** confiar en la notificación real de tu cuenta.
- **Límites/regulación de recibir pagos** → formalizar figura de negocio (Legal) antes de escalar.

## 8. Dependencias

- **Con Dept. 02:** interfaz orden↔pago y disparo de emisión de QR.
- **Con Finanzas/Legal:** cuenta de negocio, comisiones en el unit economics, facturación.

---

### Fuentes
- [Bre-B — Banco de la República](https://www.banrep.gov.co/es/bre-b) · [¿Qué es Bre-B?](https://www.banrep.gov.co/es/bre-b/que-es)
- [Documento técnico Bre-B (feb 2026, PDF)](https://d1b4gd4m8561gs.cloudfront.net/sites/default/files/publicaciones/archivos/documento-tecnico-bre-b-febrero-2026.pdf)
- [Bre-B para empresas — Mouv (guía 2026)](https://www.mouvlatam.com/recursos/bre-b-empresas)
- [Pagos y recaudos con Bre-B — Mono](https://www.mono.la/bre-b) · [Bre-B — MOVii](https://www.empresas.movii.com.co/soluciones/bre-b)
- [Bre-B: diseño e implementación — Colombia Fintech](https://colombiafintech.co/2026/03/05/bre-b-sistema-de-pagos-inmediatos-interoperado-de-colombia-diseno-implementacion-y-perspectivas/)
