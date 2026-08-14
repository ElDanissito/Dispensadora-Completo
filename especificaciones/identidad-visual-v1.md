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
5. **Físico — HECHO.** El panel genera el material de cada máquina al vuelo, en dos formatos que
   salen del mismo código (`software/internal/kit`), los dos protegidos por sesión de admin y los dos
   con 401 (no 303) si falta la sesión, porque son archivos y no páginas:

   | Endpoint | Qué es | Para qué |
   |---|---|---|
   | `GET /admin/machines/{id}/qr.svg` · `qr.png` | el QR solo (`?size=`, 128–2048) | pegar / digital |
   | `GET /admin/machines/{id}/kit.zip` | piezas SVG por separado + `LEEME.txt` | editar, reimprimir una pieza |
   | `GET /admin/machines/{id}/kit-imposicion.pdf` | **hoja de imposición** (§8.1) | lo que se le manda a la imprenta |

### 8.1 Hoja de imposición para vinilo (ADR-027)

Un **único pliego** con las **6 piezas de UNA máquina**, a escala **1:1**, listo para enviar sin que
nadie maquete nada. Existe porque la imprenta de vinilo **cobra un área mínima por pedido**: seis
archivos sueltos son seis mínimos, un pliego es uno.

**Las 6 piezas** (medidas físicas; son el contrato con la imprenta y las verifican las pruebas):

| # | Pieza | Medida | Contenido |
|---|---|---|---|
| 1 | `wrap-izquierdo` | 45 × 18 cm | tagline **"Escanea, paga, agárralo."** en tres líneas (la última en verde) + marca compacta en cuadro verde |
| 2 | `wrap-derecho` | 45 × 18 cm | **Réplica de `instrucciones.svg`**: a la izquierda **"Sin efectivo. / Sin datáfono. / Solo tu celular."** (la última en verde) + marca compacta + dominio en mono; **filete divisor**; a la derecha los **3 pasos numerados** en círculos verdes. Barra verde vertical de borde a borde en el canto (no filetes horizontales) y marca fantasma abajo a la derecha, detrás del paso 3 |
| 3 | `instrucciones-3-pasos` | 8 × 18 cm | los **3 pasos numerados** en círculos verdes: ① ESCANEA *(el QR de la máquina)* ② PAGA *(con Bre-B desde tu banco)* ③ MUESTRA *(el QR y agárralo)*. Todo **centrado**, con la jerarquía hecha de contraste y no de alineación: número en círculo, título en display grande, detalle en **mono negrita más pequeña** — los dos en `--fg`, no en `--muted` — y un **filete corto** entre pasos (el último no lleva). Se maqueta **en flujo**, no con retícula fija, porque el paso 2 lleva una línea de detalle más |
| 4 | `cabecera-grabi` | 24 × 6 cm | **solo el wordmark `GRABI.`** con el punto verde. SIN dominio y SIN "pago con Bre-B" |
| 5 | `placa` | 25 × 5 cm | **`GRABI {id}`** (ej. `GRABI M001`) |
| 6 | `qr` | 8 × 8 cm | el QR de `https://grabi.napi.lat/m/{id}` con la marca al centro (ECC **H**, cuadro blanco opaco ≤ 20 % del área) y **"ESCANEA AQUÍ"** debajo |

**Layout del pliego** — 100 × 31 cm:

```
 ┌──────────────────────────────────────────────────────────────────────────┐
 │  ① wrap izquierdo 450×180   ② wrap derecho 450×180        ③ 3 pasos 80×180│
 │                                                                          │
 ├──────────────────────────────────────────────────────────────────────────┤
 │  ④ cabecera 240×60           ⑥ QR 80×80                                  │
 │  ⑤ placa    250×50                                                       │
 └──────────────────────────────────────────────────────────────────────────┘
```

- **Fila de arriba:** los dos wraps y el panel de 3 pasos (todos de 180 mm de alto).
- **Fila de abajo:** cabecera y placa **apiladas**, y el **QR a su derecha**.
- **Márgenes: 3 mm entre piezas y 5 mm al borde** del pliego.
- **El pliego mide 100 × 31 cm, no 100 × 30.** El mínimo de la imprenta son 30 cm de alto, pero las
  alturas de las piezas suman **290 mm** (180 + 60 + 50) y con el margen de 5 y las dos separaciones
  de 3 hacen **306**, que no entra en 300. 310 mm es el alto más cercano que respeta la retícula, y
  sigue **por encima** del mínimo, así que no cambia el precio del pedido.
- **El QR se separa de la pieza más ancha de la columna apilada** (la placa, 250), no de la cabecera:
  midiéndolo contra la cabecera se monta sobre la placa en cuanto la cabecera es más estrecha.
- La retícula se **calcula** desde `ImpMargen`/`ImpGap`, no está cableada: cambiar una medida
  recoloca el pliego solo, y las pruebas fallan si dos piezas quedan a menos de 3 mm.

**Guías de corte kiss-cut** — el borde de cada pieza, en **capa aparte**, trazo de **0,25 mm**:

- Van en su propio **grupo de contenido opcional** ("GRABI · corte kiss-cut"), así la imprenta puede
  apagarlas para ver solo el arte.
- El color es un **plano con nombre, `KissCut`** (magenta como alternativo), no un magenta de
  cuatricromía: el plóter de corte busca una **separación con nombre**; un magenta de proceso se
  imprimiría como tinta encima del arte.
- Las esquinas van **redondeadas 3 mm**: en punto no se despegan bien y se levantan con el uso. El
  arte se pinta hasta el rectángulo completo, así que las esquinas cortadas quedan con demasía.

**Especificación técnica:**

- **Escala 1:1** en milímetros; `MediaBox` = `TrimBox` = `BleedBox` = el pliego. Se imprime al
  **100 %, sin "ajustar a la página"**.
- **Todo vectorial: cero imágenes rasterizadas.** No hay un DPI que se quede corto — el QR, la marca
  y el texto se rasterizan a la resolución del RIP, sea 300 o 1440 ppp.
- **Color RGB con los hex exactos de §4, no CMYK** — y es deliberado, ver ADR-027: no hay perfil ICC
  en el repo (son licenciados), así que convertir aquí sería a ciegas e **irreversible** (el RIP pasa
  el DeviceCMYK a plancha tal cual). En RGB, el RIP convierte él con el perfil del **material**, que
  en vinilo tiene más gamut que SWOP y reproduce mejor el verde `#3BE87F` (que está fuera del gamut
  CMYK). Además así el pliego especifica **el mismo color que los SVG del `kit.zip`**.
- **NO es PDF/X-1a** y no lo declara: exigiría PDF 1.3 (sin capas), un perfil ICC incrustado y las
  tipografías incrustadas. Ninguna de las tres es posible hoy sin meter archivos licenciados al repo.
- **Tipografías no incrustadas:** Helvetica-Bold (por Archivo 900 / Space Grotesk 700) y
  **Courier-Bold** (por IBM Plex Mono). Si la imprenta las sustituye, hay que pasarlas a curvas —
  igual que en los SVG. La mono va **siempre en negrita**: la redonda tiene el trazo demasiado fino
  para vinilo y a cuerpos pequeños sobre fondo oscuro el impreso se la come.
- **Reproducible byte a byte:** sin fecha de creación, para poder comparar lo que se mandó a imprimir
  con lo que genera el servidor hoy.
- **El pliego no lleva nada fuera de las piezas** (decisión de Daniel, 2026-08-13): ni línea de
  identificación ni marcas en el margen. Todo lo que se imprime es sticker. **Las advertencias de
  arriba (1:1, capa de corte, RGB, tipografías) hay que dárselas a la imprenta por fuera** — el
  archivo ya no las lleva encima.

**No incluye QRs de otras máquinas** (ADR-027): lo que se imprime en un pliego sirve solo para esa
máquina.

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
