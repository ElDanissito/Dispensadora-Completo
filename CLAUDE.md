# CLAUDE.md — Contexto del proyecto para cualquier agente de IA

> Este archivo lo lee automáticamente Claude Code (y cualquier IA con la que abras el repo).
> Es el **contexto base**: quién somos, qué construimos, las reglas y dónde está cada cosa.
> Si eres un agente trabajando en este repo, **lee esto primero**, luego el archivo de tu
> departamento en `departamentos/`, y respeta el [contrato del token](./especificaciones/contrato-token.md).

---

## Qué es este proyecto

Empresa (en fase piloto) de **máquinas expendedoras de bajo costo** para Colombia.
Diferenciador: **sin billetero, monedero ni datáfono**. El pago se hace en una web por
máquina (`dominio.com/ID`) con **Bre-B**, y la compra se entrega mostrando un **QR firmado**
que la máquina **verifica offline** y dispensa.

Flujo: web muestra productos de la máquina → cliente paga por Bre-B → el servidor firma un
**token (JWT Ed25519)** con los productos + `machine_id` + expiración + id único → se muestra
como QR → la máquina lo verifica con su **llave pública** (sin internet), comprueba que no se
haya usado, y dispensa.

**Fundador:** Daniel. Trabaja solo, apoyado en agentes de IA (uno por "departamento").
**Herramienta de código:** Claude Code (extensión de VS Code).

## Estado actual

- **Fase:** 0→1 (definición + piloto de 1 máquina).
- **Presupuesto:** sin fijar por ahora; priorizar todo lo que avanza **sin gastar** (software, investigación, diseño, drafts).
- **Pagos:** empezar con conciliación Bre-B por notificación/correo; meta = QR dinámico y luego webhook de agregador.

## Estructura del repo

```
/README.md                     Índice general (empieza por aquí para leer humano)
/plan-maestro.md               Rumbo, roadmap por fases, gerencia. LEER.
/CLAUDE.md                     Este archivo (contexto para IA).
/DECISIONS.md                  Bitácora de decisiones. Consúltala y actualízala.
/departamentos/                Un plan por departamento (misión, tareas, KPIs).
/especificaciones/             Contratos técnicos versionados (fuente de verdad).
   contrato-token.md           Interfaz Software↔Firmware. NO romper sin versionar.
/agentes/                      Briefs para lanzar sesiones de IA en paralelo.
```

## Reglas para agentes de IA (importantes)

1. **Contexto antes de actuar:** lee este archivo + tu `departamentos/NN-*.md` + `DECISIONS.md`.
2. **El contrato del token es sagrado.** Software (02) y Firmware (03) DEBEN cumplir
   `especificaciones/contrato-token.md`. Si algo debe cambiar, se versiona (v1→v2) y se
   anota en `DECISIONS.md`; nunca se cambia en silencio.
3. **Registra decisiones:** cualquier elección importante (algoritmo, proveedor, mecanismo)
   va a `DECISIONS.md` con fecha y razón.
4. **Seguridad no negociable:** la **llave privada nunca** sale del servidor ni entra al repo.
   La máquina solo tiene la **pública**. Nunca confiar en el "pantallazo de pago" del cliente,
   solo en la notificación real de la cuenta.
5. **Mantén la coherencia:** si tu cambio afecta a otro departamento, anótalo al final de tu
   archivo en una sección "Notas para otros departamentos".
6. **Commits pequeños y descriptivos.** Este repo es el trabajo compartido de la empresa; debe
   poder abrirse desde otro PC y entenderse solo.
7. **Idioma:** español.

## Decisiones técnicas ya tomadas (resumen — detalle en DECISIONS.md)

- **Firma:** Ed25519 (asimétrica). La máquina solo guarda la llave pública. Verificación con
  librería tipo Monocypher en el ESP32.
- **Backend:** Go. Front ligero (templates/HTMX). DB: SQLite en piloto → Postgres al escalar.
- **MCU:** ESP32. **Lector:** GM65 (u similar). **Reloj:** RTC DS3231 para validar expiración offline.
- **Anti-fraude del token:** `machine_id` + `exp` corto + `jti` de un solo uso (registro en memoria no volátil).

## Cómo se trabaja en paralelo

Ver `agentes/guia-trabajo-paralelo.md`. En resumen: cada departamento es una **sesión de IA**
independiente que trabaja sobre su carpeta; se usan **ramas de Git** para no chocar y se
integra en `main`. El humano (Daniel) revisa y decide.

## Glosario rápido

- **Token / JWT de dispensado:** el dato firmado que autoriza a la máquina a entregar productos.
- **`machine_id` (mid):** identificador único de una máquina; el token solo sirve en ESA máquina.
- **`jti`:** id único de la orden; garantiza que un QR se use una sola vez.
- **Bre-B:** sistema de pagos inmediatos interoperado del Banco de la República (Colombia).
- **Llave (Bre-B):** alias para recibir pagos (celular, cédula, correo, código de comercio).
