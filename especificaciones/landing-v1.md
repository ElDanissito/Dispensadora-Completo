# Especificación — Landing pública + captura de interesados (v1)

> ⚠️ **Superada por [`landing-v2.md`](./landing-v2.md)** (ADR-024) en lo que toca a estructura de
> secciones (§2), campos del formulario (§3) y hero. Siguen vigentes de este documento: §4
> (sección "Interesados" del admin), §5 (tabla `leads`), §6 (errores/404) y §7 (notas técnicas),
> que aún están **pendientes de implementar**.

> Requisitos para el agente de **Software (02)**. Implementar sobre lo que ya existe,
> **server-rendered** (Go templates + JS mínimo, **sin SPA**, ADR-011bis) y **reutilizando el
> design system** de `internal/web/templates/base.html` (variables CSS, tema kiosko oscuro/neón,
> tipografías). **Móvil primero.** No toca el contrato del token ni la conciliación: es capa nueva.

## 1. Objetivo y ubicación

- **Home pública de la marca** en `GET /` de `grabi.napi.lat` (hoy la raíz no tiene página; el app
  solo sirve `/m/{id}`). La landing pasa a ser la **home**.
- Doble función: (a) **explicar qué es GRABI** al público, (b) **captar interesados** (leads), y
  (c) servir de **destino del botón "volver a inicio"** en las páginas de error/404.
- **Objetivo primario:** claridad de marca + confianza. El CTA es dejar los datos (no vender aquí:
  la venta ocurre en `/m/{id}` frente a la máquina).

## 2. Contenido (secciones sugeridas)

El agente puede afinar textos y orden; la **intención** de cada bloque debe respetarse.

1. **Hero:** marca **GRABI** + tagline **"Escanea, paga, agárralo."** + una frase de qué es:
   *máquinas expendedoras sin efectivo, sin datáfono — pagas por Bre-B desde tu celular.*
2. **Cómo funciona (3 pasos):** (1) escanea el QR de la máquina, (2) paga por **Bre-B** el monto
   exacto, (3) muestra el QR que te llega y **agárralo**.
3. **Por qué / confianza:** sin efectivo ni tarjeta; pago por Bre-B; **el QR de retiro solo aparece
   cuando el banco confirma el pago — nunca por pantallazo**; la máquina verifica el QR **offline**.
   (Reusar el tono de seguridad que ya está en la página de compra.)
4. **Formulario "¿Quieres saber más / una GRABI en tu punto?":** ver §3.
5. **Footer:** marca, año, y (si aplica) enlaces/contacto.

## 3. Formulario de interesados (leads)

- **Campos:** **correo** y **celular** (obligatorios). Opcional: **nombre** y **mensaje corto**.
- **Envío:** `POST` server-rendered → **guarda el lead** y muestra estado de éxito ("¡Gracias! Te
  contactamos pronto.") sin salir del sitio. Validación básica de formato (email/celular).
- **Anti-spam mínimo:** campo *honeypot* oculto + rate-limit ligero por IP. Sin CAPTCHA por ahora.
- **Privacidad:** son **datos personales (PII)**. Aviso breve bajo el formulario ("usamos tus datos
  solo para contactarte, no los compartimos"). **Nunca** exponer leads en páginas públicas.

## 4. Sección "Interesados" en el admin (semilla de CRM)

- Nueva entrada en el panel admin (junto a Máquinas / Órdenes / Movimientos): **"Interesados"**.
- **Lista** de leads con: fecha, correo, celular, nombre y mensaje (si los hay), y **origen**
  (`source`, ej. "landing"). Orden: más recientes primero. Misma estética del panel (tabla en
  escritorio, tarjetas en móvil, como en Órdenes/Movimientos).
- Es la **semilla de un CRM ligero**: dejar el modelo simple pero extensible (a futuro: estado del
  lead —nuevo/contactado/descartado—, notas). No hace falta implementar esos estados ahora, solo no
  cerrar la puerta.
- Solo visible para el **admin autenticado** (rutas protegidas como el resto del panel).

## 5. Modelo de datos (nueva tabla)

- Tabla **`leads`** (PII mínima): `id`, `name` (opcional), `email`, `phone`, `message` (opcional),
  `source` (ej. "landing"), `created_at`. Mismo estilo del `schema.sql` (Postgres, epoch en BIGINT).
- No se relaciona con órdenes ni máquinas: es un registro independiente.

## 6. Errores / 404

- Las páginas de error existentes (`notfound.html`, `machine_notfound.html`, y equivalentes) llevan
  un botón **"Volver a inicio"** que apunta a **`/`** (la nueva landing). Revisar que el CTA de esas
  páginas exista y apunte bien.

## 7. Notas técnicas

- **Reutilizar `base.html`** y sus variables; no introducir un segundo sistema de estilos.
- **Sin framework SPA** (ADR-011bis): Go templates + JS vanilla mínimo (validación/UX del formulario).
- **No romper** el flujo existente `/m/{id}` → orden → conciliación → QR. Esto es una **capa nueva**
  (home + leads + sección admin), no cambia el contrato del token.
- Accesibilidad básica: contraste, `label` en inputs, foco visible (igual que el resto del sitio).
- Registrar lo implementado en `DECISIONS.md`/README y, si define esquema nuevo, mantener el estilo
  idempotente del `schema.sql`.
