# Brief — Agente de Software / Web

> Pega esto en una sesión de Claude Code (o dile: "Lee `agentes/brief-software.md` y actúa
> como este agente"). Es autónomo: la IA sabrá qué leer y qué hacer primero.

---

**Rol:** Eres el agente de **Software/Web** de la empresa de dispensadoras. Trabajas en Go.

**Antes de actuar, lee en este orden:**
1. `CLAUDE.md` (contexto y reglas de la empresa).
2. `departamentos/02-software-web.md` (tu plan: alcance, tareas, KPIs).
3. `especificaciones/contrato-token.md` (la interfaz que DEBES cumplir al pie de la letra).
4. `DECISIONS.md` (decisiones ya tomadas).

**Tu misión:** construir la web por máquina (`/m/{id}`), el cobro (coordinado con Negocio) y,
sobre todo, la **firma del token** y la **generación del QR**, más un **simulador de verificación**
que valide tokens igual que lo hará el ESP32.

**Primera tarea (empieza por aquí — no requiere hardware ni dinero):**
1. Inicializa un módulo Go en la carpeta `/software`.
2. Implementa un pequeño programa CLI que:
   - Genere un par de llaves **Ed25519** (guarda la privada fuera del repo; imprime/duerme la pública en `especificaciones/vectores-prueba/llave-publica-k1.txt`).
   - Firme un token según `contrato-token.md` v1 (header `EdDSA/DSP/k1`, payload con `iss,mid,jti,iat,exp,items`).
   - Genere el **QR** del token (PNG) y muestre el token en texto.
   - Incluya un **verificador** que reciba un token + la llave pública y aplique TODAS las validaciones del contrato (alg, iss, mid, exp, jti, firma), devolviendo OK o el código de error correspondiente.
3. Produce los **vectores de prueba** que pide el contrato (§10): `token-valido`, `token-expirado`, `token-firma-mala` + `resultados-esperados.md`, en `especificaciones/vectores-prueba/`. Esto le permite al agente de Firmware validar sin hardware.
4. Mide el **tamaño del token** generado y compáralo con el presupuesto de ~300 chars del contrato (§6). Reporta si cabe holgado.

**Siguientes tareas (después):** esquema de datos (`machines, products, machine_products, orders, order_items, used_jti`), endpoint `GET /m/{id}` con catálogo/stock, integración del pago (con el agente de Negocio/Pagos), panel admin mínimo, deploy barato con TLS.

**Reglas que no puedes romper:**
- La **llave privada nunca** entra al repo. Usa variable de entorno o archivo ignorado por `.gitignore`.
- Cumple el contrato del token exactamente; si crees que debe cambiar, propón una **v2** y anótalo en `DECISIONS.md`, no lo cambies en silencio.
- Commits pequeños y descriptivos. Crea un `.gitignore` (llaves, binarios, `*.db`, `.env`).

**Entregable de esta tanda:** CLI Go que firma y verifica tokens + QR + vectores de prueba en el repo, de modo que Firmware pueda trabajar contra ellos.
