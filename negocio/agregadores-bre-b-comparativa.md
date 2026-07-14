# Comparativa de agregadores Bre-B (para Fase 2 — webhook)

> Autor: Agente de Negocio · Fecha: 2026-07-14 · Estado: investigación preliminar.
> Objetivo: elegir, **cuando el volumen lo justifique**, un proveedor con **API de recaudos + webhook de confirmación** que elimine el parseo de correos.
> Contexto: [`04-pagos.md`](../departamentos/04-pagos.md) §Fase 2 · [`bre-b-guia-negocio.md`](./bre-b-guia-negocio.md) · ADR-004 en [`DECISIONS.md`](../DECISIONS.md).

---

## 0. Cuándo activar esto (no ahora)

La conciliación por correo (Fase 0) y el QR dinámico (Fase 1) **no cuestan comisión** y bastan para el piloto de 1 máquina. Un agregador **cobra comisión por transacción** y exige onboarding (KYC, a veces contrato/mínimos). **Solo tiene sentido migrar** cuando:
- suben los `PARSE_FALLIDO`/pagos huérfanos, o
- el volumen hace inmanejable la operación manual, o
- se quiere confirmación **instantánea y automática** (webhook) para mejorar la experiencia y escalar a varias máquinas.

---

## 1. Qué buscamos en el proveedor

| Requisito | Por qué |
|---|---|
| **API de recaudos Bre-B** | Crear cobros/QR dinámicos con monto por orden. |
| **Webhook de confirmación** | Disparar `orden.pagada` al instante, sin parsear correos. |
| **Generación de QR/llave dinámica** | Menos fricción para el cliente. |
| **Comisión baja por transacción** | Ticket de vending pequeño ($2–6k): una comisión alta se come el margen. |
| **Onboarding simple / sin mínimos** | Somos pequeños; evitar contratos pesados. |
| **Buena documentación** | Menos tiempo de integración (backend Go). |
| **Liquidación rápida** | Flujo de caja del piloto. |

> **Comisión** es el factor crítico para vending por el ticket bajo. Ninguno publica tarifas cerradas: **son negociables** y hay que **cotizar directamente**. Este documento deja la estructura lista para llenar con cotizaciones reales.

---

## 2. Tabla comparativa (preliminar — confirmar con cotización)

| Criterio | **Mono** | **Cobre** | **MOVii** |
|---|---|---|---|
| Enfoque | Infraestructura financiera vía **API** (wallets, tarjetas, pagos en tiempo real con Bre-B) | Tesorería/pagos y **recaudos** con llaves Bre-B, API documentada | Billetera + **recaudos** y transferencias 24/7 con una integración |
| API de recaudos Bre-B | Sí (pagos y recaudos inmediatos) | Sí (pagos/recaudos con llaves Bre-B) | Sí (recaudos en tiempo real) |
| Webhook de confirmación | Sí (tiempo real) — confirmar detalle | Sí — confirmar detalle en docs | Sí — confirmar detalle |
| QR dinámico / llaves | Sí | Sí (llaves Bre-B) | Sí |
| Documentación pública | Sí (dev docs) | **Sí, docs detalladas** (docs.cobre.com) | Parcial (orientado a empresas/comercial) |
| Comisión por transacción | ❓ negociable — **cotizar** | ❓ negociable — **cotizar** | ❓ negociable — **cotizar** |
| Requisitos de onboarding | ❓ (KYC empresa; confirmar si aceptan persona natural pequeña) | ❓ (confirmar) | ❓ (confirmar) |
| Mínimos / contrato | ❓ confirmar | ❓ confirmar | ❓ confirmar |
| Tiempo estimado de integración | ❓ | ❓ | ❓ |
| Liquidación (T+?) | ❓ | ❓ | ❓ |
| Notas | Alianza con Bancoomeva para llevar Bre-B a empresas | Fuerte en tesorería/conciliación automática | Fuerte en billetera/recaudo ágil |

Leyenda: ❓ = **pendiente de cotización/confirmación directa** con el proveedor.

Otros a tener en el radar (mencionados en `04-pagos.md`): **Minka / Payments Hub**, nodos como **Visionamos / Passport**, y pasarelas generales (**Wompi, Mercado Pago, ePayco**) si en el futuro se quiere aceptar también tarjetas/PSE (Fase 3, más comisión pero más cobertura).

---

## 3. Preguntas exactas para pedir cotización (mismo cuestionario a los 3)

Para poder comparar manzanas con manzanas, enviar a cada proveedor:

1. ¿Ofrecen **API de recaudos Bre-B con webhook** de confirmación de pago acreditado? ¿Latencia típica del webhook?
2. ¿Pueden generar **QR dinámico con monto exacto** por orden?
3. **Comisión por transacción** para tickets pequeños ($2.000–$6.000 COP): ¿% + fijo? ¿hay mínimo por transacción?
4. ¿**Cuota mensual/fija**, mínimos de volumen o contrato de permanencia?
5. **Requisitos de onboarding/KYC**: ¿aceptan **persona natural** / negocio pequeño, o exigen SAS con NIT?
6. **Tiempo de liquidación** de los recaudos a nuestra cuenta (T+0 / T+1…).
7. **Ambiente de pruebas (sandbox)** y calidad de la **documentación** para backend en **Go**.
8. **Tiempo estimado de integración** y soporte técnico.

---

## 4. Recomendación de proceso

1. **No integrar agregador todavía** — el piloto avanza con Fase 0/1 sin comisión.
2. Cuando se cumpla algún gatillo del §0, **enviar el cuestionario §3 a Mono, Cobre y MOVii** y llenar la tabla §2 con datos reales.
3. Priorizar por **(a) comisión efectiva sobre nuestro ticket**, **(b) simplicidad de onboarding para nuestra figura jurídica** y **(c) calidad de docs/webhook**. Cobre parte con ventaja en documentación; Mono en amplitud de infraestructura; MOVii en recaudo ágil — pero **decide la cotización real**.
4. Registrar la elección en `DECISIONS.md` (decisión pendiente: "Agregador Bre-B para la fase 2").

---

## Fuentes

- [Mono — Pagos y recaudos inmediatos con Bre-B](https://www.mono.la/bre-b) · [El País — Mono y Bancoomeva llevan Bre-B a empresas](https://www.elpais.com.co/economia/pagos-y-cobros-en-tiempo-real-la-apuesta-de-mono-y-bancoomeva-para-llevar-bre-b-a-las-empresas-0506.html)
- [Cobre — Bre-B para empresas](https://www.cobre.com/breb-empresas) · [Docs Cobre — Pagos Bre-B](https://docs.cobre.com/es/pagos-bre-b-con-instrumentos-de-pago-de-tu-ecosistema-1957689m0) · [Cobre — Llaves Bre-B para empresas](https://www.cobre.com/blog/llaves-bre-b-para-empresas)
- [MOVii — Bre-B para empresas](https://www.empresas.movii.com.co/soluciones/bre-b) · [MOVii — Recaudo](https://www.empresas.movii.com.co/soluciones/recaudo)
- [Mouv — Bre-B para empresas (guía 2026)](https://www.mouvlatam.com/recursos/bre-b-empresas) · [PCMI — impacto de Bre-B en Colombia](https://paymentscmi.com/insights/bre-b-impacto-en-colombia/)
