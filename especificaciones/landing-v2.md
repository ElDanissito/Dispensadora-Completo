# Especificación — Landing pública v2 (experiencia + B2B)

> Sucede a [`landing-v1.md`](./landing-v1.md), que queda como referencia histórica. v1 definió
> "explicar la marca + captar interesados"; v2 reordena la página en dos discursos: el **hero y el
> recorrido venden la experiencia de compra**, y la **sección B2B vende el negocio** a quien puede
> poner una máquina. Server-rendered (Go templates + JS vanilla mínimo, ADR-011bis), reutilizando el
> design system de `internal/web/templates/base.html`. **Móvil primero.** No toca el contrato del
> token ni la conciliación.

## 1. Orden del scroll

1. **Hero** — la experiencia de compra.
2. **Cómo funciona** — 4 pasos con **teléfono sticky** que cambia de pantalla.
3. **Puente** — "¿Y si esta máquina estuviera en tu edificio?".
4. **Para tu negocio (B2B)** — "Ten tu propia GRABI".
5. **Por qué GRABI** — confianza del comprador.
6. **Contacto** — captura de lead B2B.
7. **Estado del piloto** — "Piloto activo en Cali" expandido.
8. **Footer**.

**Navbar:** Cómo funciona · Para tu negocio · Por qué GRABI · Contacto (mismo orden que el scroll).

## 2. Hero

- Headline y paleta **no se tocan**: "Escanea, paga, agárralo."
- Subheadline: *"Pagas con Bre-B desde tu celular en segundos. Sin monedas, sin billetes, sin datáfono."*
- Línea de apoyo bajo el subheadline + `title` en la palabra: **"Bre-B: sistema de pagos inmediatos
  del Banco de la República."**
- **CTAs dobles:** primario sólido **"Quiero una máquina →"** → `#negocio`; secundario outline
  **"Ver cómo funciona ↓"** → `#como`. En móvil se apilan, el primario arriba.
- Stats: **Sin billeteros · Sin datáfonos · Sin filas · 100% Bre-B** (sin métricas inventadas).
- **Chip "Piloto activo en Cali"** junto a los CTAs, enlazando a la sección de piloto: es la única
  prueba social *above the fold* y es lo que retiene a un lead B2B en los primeros segundos.
- Bloque de texto centrado verticalmente.

### 2.1 Composición de dos teléfonos

- **Máximo 2 teléfonos.** Trasero: **pantalla de la tienda**, rotado ~4°, `opacity .55` y blur sutil.
  Delantero (protagonista): **pantalla del QR firmado** con "✓ pago confirmado".
- Inclinación **máx. 4–6°** (el texto interno debe seguir legible). Glow verde detrás y fondo de
  pantalla más claro que el de la página, para separar los dispositivos del lienzo.
- **Fallback:** por debajo de 1180px de ancho queda **un solo teléfono** (el del QR).
- Las pantallas son **réplicas de las pantallas reales** (`machine_public`, `machine_pago`,
  `machine_qr`): mismo copy, mismos elementos, mismo orden. Nunca capturas con productos sin imagen
  ni con "AGOTADO".

## 3. Cómo funciona (teléfono sticky)

- Dos columnas: pasos numerados a un lado, **un teléfono sticky** al otro. **Sin carrusel ni
  auto-rotación.** La pantalla cambia con el scroll (fade/slide en CSS, sin WebGL).
- Pasos y pantalla asociada:
  1. **Escanea el QR de la máquina** → cámara sobre el QR pegado en la máquina.
  2. **Elige tus productos** → tienda de la máquina.
  3. **Paga con Bre-B desde tu app bancaria** → pantalla de espera con monto exacto y llave.
  4. **Recibe tu QR firmado, acércalo al lector y agárralo** → pantalla del QR.
- El copy debe dejar claros los **dos QR con roles distintos**: el de la máquina **abre la tienda**
  (no es comprobante); el firmado **retira el producto**. Incluye la línea de seguridad de
  producción: *"El QR de retiro aparece solo cuando el banco confirma tu pago. Nunca por pantallazo."*
- `prefers-reduced-motion`: transiciones instantáneas.
- **Móvil:** pasos apilados, cada uno con su **captura estática** debajo. Sin sticky.

## 4. Sección B2B — "Ten tu propia GRABI"

Beneficios en lenguaje de administrador/dueño: sin efectivo (nada que robar ni cuadrar) · sin
datáfono ni comisiones de adquirencia · QR firmado de un solo uso que **valida sin internet** en el
punto · instalación y soporte incluidos durante el piloto · ideal para conjuntos, oficinas y
espacios donde los grandes operadores no llegan · surtido definido con el dueño del punto.

### 4.1 Separación de audiencias (no repetir argumentos)

**"Para tu negocio" habla al dueño del punto; "Por qué GRABI" habla a quien compra.** No pueden
repetir los mismos puntos (efectivo, Bre-B, QR firmado, offline) o el visitante siente déjà vu.
"Por qué GRABI" cubre solo lo que protege al comprador: **su plata sale de su propio banco**, **no
deja datos de tarjeta ni crea cuentas**, y **su QR no se falsifica** (y si expira, genera otro sin
volver a pagar). Tres tarjetas, no cuatro: la rejilla es de 3 columnas y una cuarta queda huérfana.

## 5. Captura de lead B2B

- **Form corto:** **Nombre · Tipo de espacio** (select: conjunto / oficina / negocio / otro) **·
  Ciudad · WhatsApp**. Los cuatro obligatorios. Sustituye al formulario correo+celular de v1 §3:
  el canal real de contacto es WhatsApp.
- CTA: **"Quiero una GRABI en mi espacio"**.
- **Validación en vivo** por campo (borde y mensaje) y botón habilitado solo cuando los cuatro son
  válidos. El servidor valida igual: el select solo acepta las claves de la lista.
- **Anti-spam:** honeypot oculto + rate-limit por IP (5 envíos / 10 min). Sin CAPTCHA.
- **Botón alterno de WhatsApp** con mensaje precargado ("Hola, quiero saber más sobre tener una
  máquina GRABI"). Solo se muestra si `GRABI_WHATSAPP` está configurado: **nunca se publica un
  contacto inventado**.
- **Privacidad:** son datos personales. Aviso breve bajo el formulario; los leads nunca se exponen
  en páginas públicas.

## 6. Estado del piloto

"Piloto activo en Cali" expandido con hechos verificables (una máquina, cuatro canales, pago
conciliado, verificación offline). **Foto real de la máquina cuando exista**; hasta entonces, un
aviso explícito de que no se ponen imágenes de relleno.

## 7. Accesibilidad, tipografía y responsive

- **Regla tipográfica:** la **mono** es solo para *eyebrows*, labels y datos (montos, contadores,
  chips). **Todo párrafo va en sans**: en bloques largos la mono cansa. Excepción: el interior de
  las maquetas de teléfono, que replica las pantallas reales (donde `.hint`/`.note` sí son mono).
- **Ritmo del recorrido:** cada paso ocupa ~30vh. Más alto deja tramos de scroll con la columna
  vacía y la página parece quedarse sin contenido.
- Contraste **AA** en el texto pequeño (eyebrow, stats, avisos): nada de gris `--faint` para
  texto informativo.
- Anclajes con **scroll suave** y `scroll-margin-top` por el nav sticky.
- `label` en todos los campos, foco visible, `aria-invalid` al fallar la validación.
- Todo funciona **sin JS**: los bloques se revelan, el botón queda habilitado y el servidor valida.

## 8. Prohibido

- Cambiar paleta o tipografía del headline. · Precios en la landing. · Métricas o logos inventados.
- Capturas con productos sin imagen o agotados. · Carruseles con auto-rotación. · Volver el hero B2B.

## 9. Pendiente (no bloquea)

- **Botón de WhatsApp en contacto.** El código ya está y se activa solo: en cuanto se defina
  `GRABI_WHATSAPP` (número en formato internacional, sin símbolos) aparece bajo el formulario como
  vía secundaria, con el mensaje precargado. **Falta el número.** En Colombia hay gente que no llena
  formularios y sí escribe por WhatsApp, así que conviene cerrarlo pronto.
- **Pantalla de tienda del mockup (paso 2), dos ajustes de fidelidad:**
  1. Muestra **3 productos y la máquina del piloto tiene 4 canales** (ADR-019). Un cuarto producto
     la hace fiel y de paso llena el vacío bajo la lista.
  2. Falta el campo **"Nombre de quien transfiere"**, que en producción va en esa misma pantalla
     (encima de la barra de pago) y es **obligatorio** para pagar (ADR-018). Hoy el nombre aparece
     en la pantalla de pago del paso 3 sin que se haya visto dónde se escribe.
- **Tabla `leads` + sección "Interesados" del admin** (v1 §4–§5). Hasta entonces el lead se registra
  en el **log del servidor**, no en la base.
- Fotos reales de producto: los thumbnails de la pantalla de tienda son ilustraciones SVG.
