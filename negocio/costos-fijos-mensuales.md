# Costos fijos / recurrentes — GRABI

> Registro **vivo** de gastos fijos y suscripciones. Antes no existía un lugar único (la hoja
> `unit-economics-grabi-m001.xlsx` tiene supuestos de OPEX, pero no un ledger de suscripciones).
> **Regla contable:** separar tres tipos —
> **(A) Overhead de empresa / herramientas** (I+D, no depende de cuántas máquinas haya),
> **(B) Infra del servicio** (hosting/dominio, escala con el software),
> **(C) OPEX por máquina** (arriendo del punto, reposición, datos — eso vive en el unit economics).
> El overhead de herramientas (A) **NO se carga al unit economics por máquina**; es gasto de empresa.

## Suscripciones y gastos fijos (actual)

| Concepto | Tipo | USD/mes (aprox.) | Notas |
|----------|------|------------------|-------|
| **Claude** (plan Pro) | A · herramienta | ~$20 | IA que impulsa el desarrollo (agentes, docs). Confirmar el valor exacto de tu plan. |
| **Flux AI** (plan básico) | A · herramienta | ~$15 | Diseño de PCB. **Cancelable** cuando no estés diseñando hardware — no es permanente. Confirmar valor. |
| **AWS** (EC2 + Postgres, ADR-021) | B · infra | **~$0 el 1er año** (free tier), luego ~$6–8 | Hosting del backend/landing. |
| **Dominio** `grabi.napi.lat` | B · infra | $0 | Subdominio de `napi.lat` que Daniel ya posee (ADR-019). |
| **Gmail** `grabibot` (conciliación) | B · infra | $0 | Cuenta gratuita. |
| **GitHub** (repo + Actions) | B · infra | $0 | Free tier suficiente para el piloto. |

**Total fijo actual (aprox.):** **~$35/mes** hoy (Claude $20 + Flux $15, con AWS en free tier).
Baja a **~$20/mes** si se cancela Flux al no estar diseñando PCB; sube a **~$41–43/mes** cuando
termine el free tier de AWS (año 1).

> **Conversión a COP:** montos en USD; convertir a la **tasa del mes** al consolidar (no se fija una
> tasa aquí para no quedar desactualizada). A modo grueso, ~$35 USD ≈ COP ~140.000 con una tasa
> aproximada de referencia — **ajustar a la tasa real**.

## Cómo se relaciona con el modelo

- **A (herramientas):** overhead de empresa. Con 1 máquina, come del neto total, pero **no** entra en
  el margen unitario por máquina. Al escalar, se diluye entre más máquinas.
- **B (infra):** hoy ~$0 (free tier + dominio propio). Vigilar cuando termine el free tier de AWS o al
  crecer el tráfico.
- **C (por máquina):** arriendo/comisión del punto, reposición, datos móviles → **esos sí** viven en
  [`unit-economics-grabi-m001.xlsx`](./unit-economics-grabi-m001.xlsx) (hoja Supuestos), no aquí.

## Mantenimiento

Actualizar esta tabla cuando se agregue/cancele una suscripción o cambie un plan. Referencia del
departamento: [`departamentos/07-finanzas-legal.md`](../departamentos/07-finanzas-legal.md).
