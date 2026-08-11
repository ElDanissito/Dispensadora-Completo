# static/vendor — dependencias de terceros del front

Se **autohospedan** (no CDN): el escáner debe cargar aunque el celular del cliente
esté detrás de un DNS que bloquee CDNs, y un `<script>` a un tercero es superficie
de ataque en la página que decide a dónde navegar (ADR-025).

Van **embebidas en el binario** (`//go:embed static` en `web.go`) y se sirven en
`/static/vendor/` con `Cache-Control: public, max-age=86400`.

| Archivo | Paquete | Versión | Licencia | SHA-256 (base64) |
| --- | --- | --- | --- | --- |
| `jsQR.js` | [`jsqr`](https://www.npmjs.com/package/jsqr) (`dist/jsQR.js`) | 1.4.0 | Apache-2.0 (`jsQR.LICENSE.txt`) | `vEDIoVGWI2sjFNsIVvcsoLSZgM1UE7jIUqc0n1/uCFk=` |

El hash es el `integrity` que publica npm/unpkg para ese archivo exacto. Para
verificar una copia:

```sh
openssl dgst -sha256 -binary jsQR.js | openssl base64
```

Para actualizar: descarga `https://unpkg.com/jsqr@<version>/dist/jsQR.js`, compara
el hash contra el `?meta` de unpkg, reemplaza el archivo y actualiza esta tabla.
