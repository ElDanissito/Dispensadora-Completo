# Guía de trabajo en paralelo con IA (Claude Code en VS Code)

> Cómo operar la empresa tú solo, haciendo que **varios agentes de IA avancen a la vez** sin
> pisarse, usando este repo de GitHub como cerebro compartido.

---

## Idea central

Cada **departamento** es una **sesión de IA** con un rol fijo. Le das un "brief" (ver los
archivos `brief-*.md` de esta carpeta), la IA lee su contexto (`CLAUDE.md` + su plan +
el contrato del token) y produce trabajo. Tú revisas, decides y haces commit.

Trabajas "en paralelo" en el sentido práctico: **mientras una sesión programa el backend,
otra investiga lo legal y otra prepara el kit de ventas**. El cuello de botella eres tú
revisando; por eso las tareas están troceadas y los briefs son autónomos.

## Los 3 frentes que pueden avanzar YA (sin gastar dinero)

| Frente | Brief | Qué produce sin costo |
|--------|-------|-----------------------|
| **Software** | [`brief-software.md`](./brief-software.md) | Backend Go, web `/m/ID`, firma de token, **simulador de verificación** |
| **Firmware** | [`brief-firmware.md`](./brief-firmware.md) | Prueba de verificación Ed25519 **en PC** (antes de comprar hardware) |
| **Negocio** | [`brief-negocio.md`](./brief-negocio.md) | Investigación legal/sanitaria, cuenta Bre-B, kit de ventas, hoja de unit economics |

El **Hardware físico** (comprar ESP32/GM65, construir) espera a que decidas presupuesto; mientras
tanto su parte de **diseño/CAD/BOM** puede hacerla la sesión de Firmware o una propia.

## Cómo lanzar una sesión en Claude Code (VS Code)

1. Abre la carpeta del repo en VS Code. Claude Code **lee `CLAUDE.md` automáticamente**.
2. Abre el panel de Claude Code y **pega el contenido del brief** correspondiente (o escribe:
   "Lee `agentes/brief-software.md` y actúa como ese agente").
3. La IA leerá su plan de departamento y el contrato del token, y propondrá su primera tarea.
4. Trabajas con ella, revisas los cambios (diff), y confirmas.

## Trabajar en paralelo sin chocar (Git)

**Opción A — Simple (recomendada para empezar):**
Trabaja **un frente a la vez** en `main`, con commits pequeños y frecuentes. Menos que
"paralelo real", pero cero complicación. Suficiente al inicio.

**Opción B — Paralelo real con ramas:**
Una **rama por frente** para que dos sesiones no editen lo mismo:
```
git checkout -b sw/backend       # frente Software
git checkout -b fw/verificacion  # frente Firmware
git checkout -b biz/legal        # frente Negocio
```
Cada sesión trabaja en su rama; al terminar una tanda, `merge` a `main`. Como cada frente
toca **carpetas distintas** (backend en `/software`, firmware en `/firmware`, negocio en
`/departamentos` y `/negocio`), los conflictos son raros.

**Opción C — Paralelo de verdad (avanzado): git worktrees.**
Permite tener **varias carpetas del mismo repo abiertas a la vez**, cada una en su rama y su
ventana de VS Code:
```
git worktree add ../disp-software sw/backend
git worktree add ../disp-firmware fw/verificacion
```
Abres cada carpeta en una ventana de VS Code distinta, con su propia sesión de Claude Code.
Eso es literalmente tres agentes trabajando simultáneamente. Úsalo cuando ya tengas ritmo.

## Regla de oro de coordinación

- Lo que cruza frentes (el **contrato del token**) solo se cambia versionándolo y anotándolo en
  `DECISIONS.md`. Así Software y Firmware nunca se desincronizan aunque trabajen aparte.
- Si una sesión decide algo que afecta a otra, lo escribe en `DECISIONS.md` y/o en una sección
  "Notas para otros departamentos" de su archivo.

## Ritmo semanal sugerido

1. **Lunes (gerencia, 20 min):** eliges 3–5 tareas de la ruta crítica (ver `plan-maestro.md`).
2. **Entre semana:** lanzas las sesiones por frente, revisas y haces commit/push.
3. **Viernes (10 min):** actualizas estado de tareas y `DECISIONS.md`; defines lo de la próxima semana.

## Sincroniza entre PCs

Como todo está en GitHub: `git pull` al empezar en cualquier PC y `git push` al terminar.
El repo se explica solo (por `CLAUDE.md` + `README.md`), así que retomas donde ibas.

## Higiene del repo

- Añade un `.gitignore` para secretos y binarios (ver `brief-software.md`).
- **La llave privada del servidor NUNCA se sube al repo.** Va en variable de entorno / gestor de secretos.
