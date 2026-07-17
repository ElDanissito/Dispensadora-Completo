# Especificación de UI Web — v1 (GRABI)

> Requisitos de interfaz para el agente de **Software (02)**. Implementar sobre lo que ya existe,
> **sin reescribir a SPA** (ADR-011bis): Go templates server-rendered + CSS + un poco de JS
> vanilla/HTMX para interacciones (cantidad, copiar, popups). **Móvil primero** (la mayoría de
> pedidos son desde el celular frente a la máquina). Mantener y extender el design system actual
> de `base.html` (variables CSS, tema oscuro, tarjetas).

---

## 1. Página pública de venta — `GET /m/{id}`

### 1.1 Vista tipo "máquina dispensadora" (cuadrícula)

- Mostrar los productos en una **cuadrícula (grid)** que **simula la máquina**: cada celda = un
  espacio/slot de la máquina.
- Orden de las celdas **por número de slot** (para que se parezca a la disposición física).
  Idealmente configurable el nº de columnas por máquina (ej. 3–4 en móvil, más en pantalla grande).
- **Responsive:** en celular 2 columnas (o 1 si el contenido lo exige), en pantalla grande 3–4.
  Objetivos de tap grandes (botones cómodos con el dedo).

### 1.2 Contenido de cada celda (producto)

- **Imagen** del producto (foto). Placeholder si no tiene imagen.
- **Nombre** del producto.
- **Slot** (número).
- **Stock** (unidades disponibles). Si stock = 0 → celda en estado **"Agotado"** (deshabilitada,
  atenuada, sin control de cantidad).
- **Precio**.
- Control de cantidad **"– 1 +"**: muestra cuántas unidades de ese producto ha añadido el cliente.
  - No permitir superar el **stock** disponible.
  - `–` no baja de 0.

### 1.3 Carrito / resumen

- Un resumen (barra inferior fija en móvil o tarjeta) con **total de ítems y monto total**.
- Botón **"Pagar"** que lleva a la zona de pago. Deshabilitado si no hay ítems.

## 2. Zona "Paga tu compra"

> **Actualizado por ADR-018:** el matching de conciliación pasó de *monto único* a **monto exacto
> (redondo) + nombre de quien transfiere**. Por eso: (a) el formulario de compra pide un campo
> **obligatorio "nombre de quien hace la transferencia"**; (b) la zona de pago muestra el **monto
> exacto** (ya no un valor con "pesos de verificación") y **el nombre** con el que debe llegar el
> pago. El *monto único* queda como fallback (`GRABI_MATCH_MODE=unique_amount`), en cuyo caso se
> vuelve a mostrar el desglose del desambiguador.

- Mostrar el resumen de la compra (productos + total).
- **Llave Bre-B de destino** (la de esa máquina, ej. `GRABI M001`) **destacada visualmente**:
  fondo **verde sombreado** (usar/introducir una variable de acento verde coherente con el tema),
  texto grande y legible.
- Al lado de la llave, **botón/ícono de "copiar al portapapeles"** (Clipboard API). Al copiar,
  feedback visual ("¡Copiado!"). Para que el cliente la pegue en su app del banco / Bre-B.
- Mostrar también el **monto exacto a transferir** (el **valor redondo** de la compra, clave para
  la conciliación) igualmente con opción de copiar.
- **[ADR-018] Nombre del pagador:** mostrar, destacado, el **nombre con el que debe llegar la
  transferencia** (el que el cliente escribió al comprar). Sirve para que el pago se concilie.
- Instrucciones cortas de 3 pasos: (1) copia la llave, (2) transfiere el **monto exacto** por Bre-B
  **desde la cuenta de ese nombre**, (3) espera el QR y muéstralo a la máquina.
- **[ADR-018] Pago ambiguo:** si un pago casa con ≥2 órdenes, no se entrega QR; mostrar una
  **pantalla de "pago en revisión"** con instrucciones de soporte (no un error técnico).
- Mantener la lógica de emisión del QR tras conciliar el pago (no cambiarla más allá del criterio de
  matching descrito).

## 3. Panel admin — CRUD completo de productos

- **Crear** producto: nombre, precio, slot, stock, **imagen** (subida de archivo), (opcional) descripción.
- **Editar** producto: todos los campos, incluida la imagen.
- **Eliminar** producto.
- Listado claro de productos por máquina con su slot, stock, precio e imagen (miniatura).
- **Gestión de stock / refill:** poder fijar el stock de cada producto (esto es el **recuento de
  refill**, que es la "verdad" del inventario según **ADR-012**).

### 3.1 Imágenes

- Subida desde el panel; guardar en disco (carpeta servida estáticamente, ej. `/uploads` o
  `/static/img`) o en la DB. Referenciar la ruta en el producto. Validar tipo/tamaño.
- **Las imágenes de productos SÍ pueden ir al repo** solo si son genéricas; las subidas por el
  admin en operación van a una carpeta de datos (evaluar `.gitignore` para no ensuciar el repo).

## 4. Avisos / popups al admin (cuando un cambio afecta la máquina física)

Mostrar un **popup/aviso** (modal o banner) según la acción, para que el admin sepa que debe ir a
ajustar la máquina o tener algo en cuenta:

| Acción del admin | Aviso a mostrar |
|------------------|-----------------|
| Cambiar el **slot** de un producto, o asignar producto a un slot | ⚠️ "El slot está **físicamente** conectado a un canal/motor de la máquina. Verifica que el producto cargado en ese canal coincida con lo que configuras aquí." |
| Ajustar **stock** (refill) | ℹ️ "Este número debe coincidir con las unidades que **cargaste físicamente** en la máquina. El conteo del sistema es un estimado que se corrige en cada recarga (ADR-012)." |
| Cambiar **precio** | ℹ️ "El precio define el monto a cobrar. Evita precios idénticos entre productos para no dificultar la conciliación por **monto único**." |
| **Eliminar** producto con **stock > 0** | ⚠️ "Aún hay unidades físicas en la máquina. Retíralas del canal antes de eliminar el producto del sistema." |
| Producto asignado a un slot **sin motor conectado** (ej. slots aún sin cablear) | ⚠️ "Este slot todavía **no dispensa** (motor no conectado). No lo publiques para venta hasta cablearlo." |
| Crear producto **sin imagen** | 💡 "Recomendado: sube una foto del producto para que el cliente lo reconozca en la cuadrícula." |

> El agente puede afinar los textos, pero la **intención** de cada aviso debe respetarse. Idealmente
> un aviso que requiere acción física (⚠️) exige una confirmación explícita ("Entendido").

## 5. Login del admin como página propia

- Reemplazar el mecanismo actual (si es Basic Auth / prompt del navegador) por una **página de
  login** propia: formulario usuario + contraseña, con el estilo de la marca, manejo de error
  ("credenciales inválidas"), y sesión (cookie). Rutas admin protegidas → redirigen al login si no
  hay sesión.
- Credenciales del admin **por variable de entorno / config** (no hardcodeadas, no al repo).

## 6. Notas técnicas y de estilo

- **Mantener el design system** de `base.html` (variables CSS). Añadir una variable de **verde
  destacado** para la llave y estados OK.
- **Móvil primero:** probar en viewport de celular; controles grandes; nada que dependa de hover.
- Interacciones sin recargar toda la página: **HTMX o JS vanilla** (cantidad ±, copiar, popups).
  Nada de framework SPA (ADR-011bis).
- No romper el flujo existente de orden → conciliación → QR. Este trabajo es **capa de presentación
  + CRUD admin**, no cambia el contrato del token ni la conciliación.
- Accesibilidad básica: contraste suficiente, `alt` en imágenes, foco visible en botones.
