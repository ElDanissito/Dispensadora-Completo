# software — backend y herramientas del token de dispensado

Módulo Go del [Departamento 02 · Software/Web](../departamentos/02-software-web.md).
Esta primera tanda entrega el núcleo criptográfico: **firmar y verificar** el token de
dispensado (JWS Ed25519) según [`especificaciones/contrato-token.md`](../especificaciones/contrato-token.md) v1,
generar su **QR** y producir los **vectores de prueba** para que Firmware valide sin hardware.

## Estructura

```
software/
  cmd/dsp/            CLI `dsp` (keygen, sign, verify, qr, vectors)
  internal/dsptoken/  firma + verificación (referencia canónica del contrato)
  internal/qr/        generación de QR PNG
  .keys/              llaves privadas — IGNORADO por git (nunca commitear)
```

La verificación en `internal/dsptoken` sigue **exactamente** el orden de validaciones del
contrato §5 y los códigos de error del §7. Es la implementación de referencia que el
firmware (Dept. 03) debe reproducir para dar resultados idénticos sobre los vectores.

## Uso

Compilar:

```sh
cd software
go build -o dsp ./cmd/dsp      # en Windows: dsp.exe
```

### Generar el par de llaves

```sh
./dsp keygen
```

Guarda la **privada** en `software/.keys/private-k1.key` (ignorada por git) y la **pública**
en `especificaciones/vectores-prueba/llave-publica-k1.txt`. La privada también puede
inyectarse por la variable de entorno `DSP_PRIVATE_KEY` (base64), sin tocar el disco.

> **Regla no negociable:** la llave privada nunca entra al repo ni sale del servidor.

### Firmar un token (+ QR opcional)

```sh
./dsp sign -mid M001 -items "3:1,5:2" -qr orden.png
```

Imprime el token en stdout y, por stderr, el `jti`, `iat`, `exp` y la longitud en caracteres.
Por defecto `exp = iat + 300s` (ventana de 5 min del contrato) y el `jti` es aleatorio.

### Verificar un token (simulador de la máquina)

```sh
./dsp verify -mid M001 -in especificaciones/vectores-prueba/token-valido.txt -now 1752460900
```

Imprime `OK` o el código de error (`EXPIRED`, `BAD_SIGNATURE`, `WRONG_MACHINE`, …) y sale
con código 3 si no es `OK` (útil para scripts). `-now` fija la hora del RTC simulado.

### Regenerar los vectores de prueba

```sh
./dsp vectors
```

Escribe `token-valido`, `token-expirado`, `token-firma-mala` y `resultados-esperados.md`
en `especificaciones/vectores-prueba/`.

## Pruebas

```sh
go test ./...
```

Los tests de `internal/dsptoken` cubren cada código de error del contrato y el orden de
validación (p. ej. que la firma se valida antes que la expiración).

## Estado y pendientes

Entregado en esta tanda: keygen, sign, verify, qr, vectors + tests.
Siguiente (ver plan del Dept. 02 §6): esquema de datos, endpoint `GET /m/{id}`, integración
de pago con Dept. 04, panel admin y deploy con TLS.

Ver el hallazgo sobre **tamaño del token vs. presupuesto del QR** en
[`DECISIONS.md`](../DECISIONS.md) (propuesta ADR-006).
