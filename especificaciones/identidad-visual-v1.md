# Identidad visual — GRABI (v1)

> Guía de marca **fuente de verdad**. Formaliza lo que ya vive en el design system de
> `software/internal/web/templates/base.html` (para que marca y producto se sientan una sola cosa) y
> fija lo que falta para producir assets. **Digital-first** (piloto); el físico va como sección a
> futuro. El diseño de piezas lo hace Daniel en **Claude Design** a partir de esta guía; un agente de
> software cablea los assets (favicon/OG/manifest) en el sitio.

---

## 1. Esencia

- **Qué es:** GRABI son máquinas expendedoras de bajo costo **sin efectivo, sin datáfono**: escaneas,
  pagas por **Bre-B** desde el celular y retiras con un **QR firmado**.
- **Personalidad:** tech pero cercana, directa, confiable, colombiana. Nada corporativo-frío.
- **Tagline:** **"Escanea, paga, agárralo."** (no se traduce ni se reescribe en piezas oficiales).
- **Nombre/escritura:** siempre **GRABI** en mayúsculas. En el logotipo lleva **punto** ("GRABI.").

## 2. Logotipo (wordmark)

- Marca principal = **wordmark "GRABI."** en tipografía **Archivo (900)**, con el **punto en verde
  acento**. El punto no es decorativo: es el remate de marca (evoca el "listo/hecho" del retiro).
- **Mayúsculas siempre.** No inclinar, no condensar, no cambiar la tipografía, no separar letras.
- Versiones:
  - **Sobre oscuro (primaria):** texto `--fg` (#EAF4EE) + punto `--accent` (#3BE87F). Es la del sitio.
  - **Sobre claro (secundaria):** texto casi negro `--accent-ink` (#05130B) + punto verde.
- **Área de respeto:** margen libre alrededor ≥ la altura de la "G". **Tamaño mínimo legible:** ~72px
  de ancho en pantalla (por debajo de eso, usar la marca compacta, §3).

## 3. Marca compacta / símbolo — DECISIÓN TOMADA: Ruta B (2026-07-26)

**La marca compacta oficial es la Ruta B: el punto verde dentro de un visor de escaneo (lente/scan).**
Motivos: es legible a 16px (geometría pura), es distintiva (una "G" sola sería genérica) y **cuenta el
producto** — el visor dice "escanea", que es el paso 1; además funciona en la máquina como señal de
"escanea aquí". El **punto verde** es el elemento compartido con el wordmark, así que el sistema
coherer. De aquí salen favicon, ícono de app y avatar.

El wordmark **no funciona** en tamaños chicos (favicon 16–32px, ícono de app, avatar); por eso la
marca compacta. Rutas que se exploraron:

- **Opción A — "G." :** la G de Archivo + el punto verde. Máxima continuidad con el wordmark.
- **Opción B — solo el punto/lente:** el punto verde convertido en símbolo (círculo/lente que evoca
  "escanear"). Muy limpio a 16px, pero más abstracto.
- **Opción C — glifo scan/grab:** un ícono simple (esquinas de mira de QR, o una mano/agarre estilizado).

> Recomendación: explorar **A** y **B**; elegir la que siga legible a 16px sobre el verde y sobre el
> oscuro. Cuando se elija, se documenta aquí y de ahí salen favicon, ícono de app y avatar.

## 4. Paleta (tokens reales de `base.html`)

**Base (tema oscuro "kiosko"):**

| Rol | Token | Hex |
|-----|-------|-----|
| Fondo | `--bg` | `#0A0E0C` |
| Superficie | `--surface` | `#0D1210` |
| Superficie 2 | `--surface-2` | `#121814` |
| Líneas | `--line` / `--line-2` | `#26332B` / `#3A4A40` |
| Texto | `--fg` | `#EAF4EE` |
| Texto atenuado | `--muted` | `#8FA79A` |

**Acento (verde GRABI):**

| Rol | Token | Hex |
|-----|-------|-----|
| Acento | `--accent` | `#3BE87F` |
| Acento claro (hover) | `--accent-2` | `#7FF2AC` |
| Tinta sobre verde | `--accent-ink` | `#05130B` |

**Semánticos:** ok `#2ED3B7` · warn `#FFB454` · crítico `#FF6B6B` · info `#5AB8FF`.

> Regla de contraste: **sobre el verde acento siempre va tinta oscura** (`--accent-ink`), nunca texto
> claro. El verde es para acentos y CTAs, no para grandes áreas de texto.

## 5. Tipografía

- **Archivo** (900) — marca y titulares grandes (display).
- **Space Grotesk** (400/500/700) — **texto y UI**. Todo párrafo va aquí.
- **IBM Plex Mono** (400–700) — **solo** *eyebrows*, labels, y datos (montos, contadores, chips,
  códigos). En bloques largos la mono cansa: no usarla para párrafos (regla de `landing-v2.md` §7).

## 6. Iconografía e ilustración

- Estilo **line/SVG**, trazo limpio, coherente con el tema oscuro. **SVG primero** (escala y pesa poco).
- Mientras no haya **fotos reales** de producto/máquina, se usan **ilustraciones SVG** (como los
  thumbnails de la tienda). **Prohibido relleno inventado:** no fotos de stock genéricas, no logos ni
  métricas falsas (regla de `landing-v2.md` §8).

## 7. Tono de voz

- Claro, corto, en español colombiano, sin jerga. Verbos de acción ("escanea", "paga", "agárralo").
- Refuerza confianza sin marear: la línea de seguridad canónica es *"El QR de retiro aparece solo
  cuando el banco confirma tu pago. Nunca por pantallazo."*

## 8. Assets a producir (prioridad digital-first)

1. **Favicon / ícono de app:** set multi-tamaño (16/32/48), `favicon.ico`, `apple-touch-icon` (180),
   e íconos de `manifest` (192/512). Salen de la **marca compacta** (§3).
2. **Imagen social / OG** `1200×630` (para previsualización de `grabi.napi.lat` al compartir) + `twitter:card`.
3. **Wordmark** en SVG: lockup sobre oscuro y sobre claro, con área de respeto.
4. **Avatar cuadrado** para redes (marca compacta centrada sobre `--surface` o verde).
5. *(A futuro, no ahora)* **físico:** vinilo/wrap de la máquina, sticker del QR, impresos. Se define
   cuando el piloto valide; requiere versiones en alta resolución y CMYK.

## 9. Dónde viven y cómo se integran

- Los archivos estáticos (favicon, OG, svg) van en la carpeta estática del sitio (ej.
  `software/internal/web/static/brand/`), servidos por el backend Go.
- El agente de software añade los `<link rel="icon">`, `apple-touch-icon`, `manifest.webmanifest` y
  las metas `og:*` / `twitter:*` en el `<head>` de `base.html`. Es capa de presentación: no toca el
  contrato del token ni la conciliación.

## 10. Prohibido (resumen)

Cambiar tipografía o color del wordmark · texto claro sobre el verde · estirar/inclinar/condensar la
marca · imágenes de relleno o métricas/logos inventados · introducir un segundo sistema de color
distinto al de `base.html`.
