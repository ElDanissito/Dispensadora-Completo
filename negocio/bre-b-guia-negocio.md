# Guía Bre-B para el negocio — cómo cobrar en el piloto

> Autor: Agente de Negocio · Fecha: 2026-07-14 · Ciudad piloto: **Cali**
> Estado: investigación (no es asesoría financiera/legal; validar cuenta y trámites con la entidad).
> Relacionado: [`04-pagos.md`](../departamentos/04-pagos.md) · [`spec-conciliacion-correo.md`](./spec-conciliacion-correo.md) · [`agregadores-bre-b-comparativa.md`](./agregadores-bre-b-comparativa.md) · ADR-004 en [`DECISIONS.md`](../DECISIONS.md)

---

## 0. TL;DR (lo que hay que hacer para cobrar ya)

1. **Abre una cuenta/producto dedicado al negocio** (no tu cuenta personal de uso diario) y **registra una Llave Bre-B de negocio** en ella. Es **gratis**.
2. Para el **MVP de conciliación por correo**, lo que decide la entidad es **si envía un correo en tiempo real, por cada pago entrante, que se pueda parsear**. **Bancolombia** (alertas y notificaciones por correo en tiempo real) encaja mejor que Nequi para ese enfoque. Ver §4.
3. El anti-fraude de conciliación es **monto único por orden** (centavos aleatorios): el cliente paga a tu llave un valor irrepetible y el sistema casa pago↔orden por monto + ventana de tiempo. Ver [`spec-conciliacion-correo.md`](./spec-conciliacion-correo.md).
4. **Nunca** confíes en el "pantallazo" del cliente: solo en la notificación real de tu cuenta (ADR-004).
5. Cuando el volumen lo pida, migra a **QR dinámico** y luego a **webhook de un agregador** (Mono/Cobre/MOVii). Ver comparativa.

---

## 1. Qué es Bre-B y por qué nos sirve

**Bre-B** es el **Sistema de Pagos Inmediatos Interoperado** del Banco de la República (en operación desde oct-2025). Permite transferencias **inmediatas (< 20 s)**, **24/7/365**, **interoperables** entre cualquier banco, cooperativa, fintech o billetera vinculada (218+ entidades a ene-2026), usando **"llaves"** (celular, cédula, correo, código alfanumérico o **código de comercio/establecimiento**). ([Banrep — ¿Qué es Bre-B?](https://www.banrep.gov.co/es/bre-b/que-es))

**Costo:** para usuarios y empresas es **gratuito durante al menos los primeros 3 años**; después, el Banrep empezará a cobrar una tarifa **a las entidades financieras** por transacción (Cobre menciona ~$6,46 por transacción a partir del 4.º año — cifra a confirmar y que aplica a la entidad, no necesariamente trasladada al comercio). ([Cobre — Llaves Bre-B para empresas](https://www.cobre.com/blog/llaves-bre-b-para-empresas))

**Límite por operación:** 1.000 UVB. Para nuestro ticket de vending (≈ $2.000–6.000 COP) el límite es **irrelevante**.

**Por qué encaja con nuestro flujo:** el cliente paga desde su propio celular, con su app bancaria, a nuestra llave, y el dinero llega en segundos. No necesitamos billetero, monedero ni datáfono. Bre-B soporta además **QR dinámicos con monto específico** por transacción, que es a donde queremos llegar (Fase 1).

---

## 2. Tipos de llave (y cuál nos conviene)

| Llave | Quién la puede tener | Nota para nosotros |
|---|---|---|
| **Celular** | Persona natural / negocio | Simple; sirve para recibir. |
| **Cédula (CC/CE/PPT)** | Persona natural | Simple; sirve para recibir. |
| **Correo electrónico** | Persona natural / negocio | Útil, e-commerce/facturación. |
| **Alfanumérica** | Ambos (la asigna la entidad) | Genérica. |
| **NIT** | Empresa / persona jurídica | Requiere formalización (SAS/RUT con NIT). |
| **Código de establecimiento / comercio (Merchant ID)** | Negocio | **La ideal a mediano plazo:** identifica al comercio, facilita conciliación contable y habilita QR de negocio. |

Fuentes: [Cobre — Llaves Bre-B para empresas](https://www.cobre.com/blog/llaves-bre-b-para-empresas), [La República — Llaves para negocios de Bre-B](https://www.larepublica.co/finanzas/llaves-para-negocios-de-bre-b-4241983), [Banrep — Preguntas frecuentes](https://www.banrep.gov.co/es/bre-b/preguntas-frecuentes).

**Recomendación por fase:**
- **Piloto (persona natural):** empezar con **llave de negocio sobre persona natural** (p. ej. **Nequi Negocios** o **Bancolombia** con producto/punto de venta). Cero costo, arranca hoy.
- **Al formalizar (SAS):** migrar a **llave NIT + código de establecimiento** de una cuenta empresarial. Coordinar con [`07-finanzas-legal.md`](../departamentos/07-finanzas-legal.md) (la figura jurídica es decisión pendiente).

> ⚠️ **No mezclar dinero personal y del negocio** desde el día 1: complica contabilidad y conciliación. Aunque sea persona natural, usa un producto **dedicado** solo a los cobros de la máquina.

---

## 3. Opciones de entidad para el piloto (persona natural, sin gastar)

### Opción A — Bancolombia (recomendada para el MVP de conciliación por correo)
- **Registro de llave de negocio:** app *Mi Bancolombia* → "Bre-B / Tus llaves" → **Negocios** → *Registrar llaves* → elegir punto de venta/cuenta → aceptar tratamiento de datos → *Registrar llave* y generar **Código QR**. Gratis. ([Bancolombia — Cómo registrar tu negocio en Bre-B](https://www.bancolombia.com/centro-de-ayuda/preguntas-frecuentes/como-registrar-negocio-en-bre-b))
- **Notificaciones:** Bancolombia envía **alertas en tiempo real por correo/SMS/push** cada vez que llega dinero (configurable, con topes mínimos): *Sucursal Virtual → Ajustes → Seguridad → Alertas y Notificaciones*. ([Bancolombia — Alertas y notificaciones](https://www.bancolombia.com/personas/aprender-es-facil/como-usar-banco/seguridad/alertas-notificaciones/1000))
- **Por qué encaja:** el **correo por transacción** es lo que el MVP parsea. Es la razón por la que Bancolombia es la primera opción del piloto.

### Opción B — Nequi Negocios (excelente para recibir, ojo con la conciliación por correo)
- **QR Negocios:** requiere ser **persona natural** con **CC, CE o PPT**; el registro y uso es **gratis**. Muestras tu QR de negocio y recibes de cualquier banco/billetera Bre-B. ([Nequi — QR Negocios en Bre-B](https://ayuda.nequi.com.co/hc/es/articles/40068240132749-Todo-lo-que-necesitas-saber-sobre-tu-QR-Negocios-en-Bre-B))
- **Notificación:** llega **push/in-app** en tiempo real; el **reporte de ventas por correo es solo a solicitud** (hasta 5 días hábiles). → **No hay un correo automático por cada pago**, así que el parseo de correo del MVP **no aplica directo** con Nequi (habría que reenviar la notificación push, lo que agrega fragilidad).
- **Conclusión:** Nequi es ideal para **recibir** y para la **Fase 1 (QR dinámico de negocio)**, pero para la **Fase 0 (conciliación por correo)** conviene Bancolombia.

### Opción C — Daviplata / Davivienda QR Bre-B para pymes
- Davivienda también empuja **QR Bre-B para pymes**. ([Misión Pyme — Davivienda potencia pymes con QR Bre-B](https://misionpyme.com/noticias/finanzas/pagos-inmediatos-negocios-que-crecen-davivienda-potencia-a-las-pymes-con-qr-bre-b/)) Alternativa válida; evaluar sus notificaciones por correo antes de elegirla para el MVP.

> **Nota Cali:** todas estas entidades operan a nivel nacional; no hay diferencia por ciudad para abrir la llave. Lo local (ICA, prospección de puntos) se aborda en Legal/Comercial.

---

## 4. Decisión práctica para el piloto

**Cuenta de cobro del MVP = Bancolombia (persona natural, producto dedicado) con alertas de correo en tiempo real activadas**, más monto único por orden para conciliar. Esto permite arrancar **hoy, gratis**, y que el agente de Software implemente la [spec de conciliación por correo](./spec-conciliacion-correo.md).

**Ruta de evolución:**
1. **Fase 0 (hoy):** llave de negocio en Bancolombia + conciliación por correo (monto único).
2. **Fase 1:** **QR dinámico** de negocio con monto exacto (Nequi Negocios / Bancolombia) → menos fricción, matching más limpio; confirmación aún por notificación.
3. **Fase 2:** **webhook de agregador** (Mono/Cobre/MOVii) → confirmación instantánea y automática, sin parsear correos. Ver [comparativa](./agregadores-bre-b-comparativa.md).

---

## 5. Riesgos y puntos a vigilar

- **Fragilidad del correo:** si el banco cambia el formato del correo, se rompe el parseo. Mitigación: pruebas con correos reales + plan de migración a webhook. (Detalle en la spec.)
- **Estafas Bre-B por SMS/correo falsos:** hay olas de *phishing* que simulan notificaciones Bre-B. El sistema **solo** debe confiar en correos del **remitente oficial verificado** de la entidad y **nunca** seguir enlaces ni pedir datos. ([Infobae — alerta Bancolombia por estafas Bre-B](https://www.infobae.com/colombia/2025/07/24/bancolombia-alerto-por-estafas-digitales-con-llaves-bre-b-usuarios-alegan-que-les-llegan-mensajes-de-texto/))
- **Riesgo regulatorio (impuestos):** existe un **decreto propuesto** para aplicar **retención/cobro (~1,5%)** a pagos por Nequi, DaviPlata, PSE y Bre-B. Si prospera, afecta el unit economics. **Vigilar** y trasladar a Finanzas. ([El Colombiano — decreto retención pagos digitales](https://www.elcolombiano.com/negocios/decreto-minhacienda-retencion-fuente-pagos-nequi-daviplata-pse-y-bre-b-ED30221656))
- **Informalidad al recibir pagos:** recibir en cuenta personal escala mal. Formalizar (cuenta/llave de negocio, y SAS cuando toque) antes de crecer. Ver Legal.

---

## 6. Checklist accionable (piloto)

- [ ] Abrir producto Bancolombia **dedicado** a los cobros de la máquina (persona natural por ahora).
- [ ] Registrar **Llave Bre-B de negocio** en ese producto (gratis).
- [ ] Activar **alertas por correo en tiempo real** para abonos/transferencias entrantes, con tope mínimo bajo (que notifique incluso montos pequeños).
- [ ] Hacer **3 transferencias de prueba** desde otro banco y guardar los correos reales → insumo para el parser (ver spec §"Muestras").
- [ ] Entregar al agente de Software la [spec de conciliación](./spec-conciliacion-correo.md) + las muestras de correo.
- [ ] Cuando Daniel apruebe: registrar en `DECISIONS.md` la entidad y llave del piloto.

---

## Fuentes

- [Banrep — ¿Qué es Bre-B?](https://www.banrep.gov.co/es/bre-b/que-es) · [Preguntas frecuentes Bre-B](https://www.banrep.gov.co/es/bre-b/preguntas-frecuentes) · [Documento técnico Bre-B (feb 2026, PDF)](https://d1b4gd4m8561gs.cloudfront.net/sites/default/files/publicaciones/archivos/documento-tecnico-bre-b-febrero-2026.pdf)
- [Bancolombia — Cómo registrar tu negocio en Bre-B](https://www.bancolombia.com/centro-de-ayuda/preguntas-frecuentes/como-registrar-negocio-en-bre-b) · [Alertas y notificaciones](https://www.bancolombia.com/personas/aprender-es-facil/como-usar-banco/seguridad/alertas-notificaciones/1000) · [Preguntas frecuentes Bre-B negocios](https://blog.bancolombia.com/negocios/preguntas-frecuentes-breb-negocios/)
- [Nequi — QR Negocios en Bre-B](https://ayuda.nequi.com.co/hc/es/articles/40068240132749-Todo-lo-que-necesitas-saber-sobre-tu-QR-Negocios-en-Bre-B) · [Nequi — Negocios QR](https://www.nequi.com.co/negocios/negocios-qr)
- [Cobre — Llaves Bre-B para empresas](https://www.cobre.com/blog/llaves-bre-b-para-empresas)
- [La República — Llaves para negocios de Bre-B](https://www.larepublica.co/finanzas/llaves-para-negocios-de-bre-b-4241983)
- [Misión Pyme — Davivienda QR Bre-B pymes](https://misionpyme.com/noticias/finanzas/pagos-inmediatos-negocios-que-crecen-davivienda-potencia-a-las-pymes-con-qr-bre-b/)
- [Infobae — alerta estafas Bre-B](https://www.infobae.com/colombia/2025/07/24/bancolombia-alerto-por-estafas-digitales-con-llaves-bre-b-usuarios-alegan-que-les-llegan-mensajes-de-texto/)
- [El Colombiano — decreto retención pagos digitales](https://www.elcolombiano.com/negocios/decreto-minhacienda-retencion-fuente-pagos-nequi-daviplata-pse-y-bre-b-ED30221656)
- [Mouv — Bre-B para empresas (guía 2026)](https://www.mouvlatam.com/recursos/bre-b-empresas)
