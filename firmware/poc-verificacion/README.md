# PoC de verificación del token — Firmware (Dept. 03)

Prueba de concepto **en el PC** de que la verificación criptográfica del
[contrato del token](../../especificaciones/contrato-token.md) (**v2**) funciona,
**antes de comprar hardware**. Este mismo código de verificación se portará tal
cual al ESP32 (Monocypher es portable a microcontroladores).

> **Actualizada al contrato v2 (ADR-006):** el payload ya no lleva `iss` ni `iat`.
> La verificación pasa de firma → `mid` → `exp` → `jti` (se elimina el paso de
> `iss` y el código `BAD_ISSUER`). Vectores regenerados por el agente 02.

**Criterio de éxito:** el resultado de cada vector coincide *exactamente* con
[`resultados-esperados.md`](../../especificaciones/vectores-prueba/resultados-esperados.md),
es decir, el firmware (03) da lo mismo que el simulador del backend (02).

## Qué hace

Implementa la verificación EXACTA del contrato §5 y devuelve los códigos de §7:

1. Parte el token en 3 (`header.payload.firma`) → `MALFORMED` si no hay 3 partes.
2. Decodifica el header; exige `alg="EdDSA"`, `typ="DSP"`; obtiene `kid`.
3. Busca la llave pública de ese `kid` → `UNKNOWN_KEY` si no la tiene.
4. Verifica la **firma Ed25519** sobre `header.payload` → `BAD_SIGNATURE` si falla.
5. Decodifica el payload y valida `mid` → `exp` (vs. RTC) → `jti` no usado (v2: sin `iss`).
6. Marca el `jti` como usado **antes** de "dispensar" (imprime los `items`).

## Compatibilidad de firma (importante)

El servidor firma con `crypto/ed25519` de Go = **Ed25519 estándar RFC 8032
(SHA-512)**. Por eso la PoC usa `crypto_ed25519_check()` del módulo **opcional**
`monocypher-ed25519` (EdDSA + SHA-512), y **no** `crypto_eddsa_check()` del
núcleo de Monocypher, que por defecto usa BLAKE2b y **no** sería compatible.

## Estructura

```
firmware/poc-verificacion/
  verificar.c        Programa de la PoC (este es el código que se porta al ESP32).
  run-poc.sh         Compila + corre los 3 vectores + caso de reuso, y compara (Linux/macOS/MSYS).
  run-poc.ps1        Igual que run-poc.sh, pero para Windows con MSVC (cl.exe).
  monocypher/        Monocypher 4.0.2 vendorizado (un solo módulo, sin dependencias):
    monocypher.c/.h              núcleo
    monocypher-ed25519.c/.h      Ed25519 estándar (SHA-512)  <-- el que usamos
  README.md
```

## Cómo compilar

Necesitas un compilador de C. Cualquiera de estos sirve (`gcc` es el de referencia):

```bash
# gcc / clang
gcc -O2 -Wall -o verificar \
    verificar.c monocypher/monocypher.c monocypher/monocypher-ed25519.c -Imonocypher

# alternativa portable (sin instalar toolchain completo): Zig como compilador C
zig cc -O2 -o verificar \
    verificar.c monocypher/monocypher.c monocypher/monocypher-ed25519.c -Imonocypher
```

En **Windows** tienes tres caminos:
- **MSVC (Visual Studio / Build Tools):** usa el runner dedicado `run-poc.ps1`
  (detecta `vcvars64.bat` automáticamente). Es la vía probada en este proyecto.
- **MSYS2/MinGW-w64:** te da `gcc`; usa el comando de arriba o `bash run-poc.sh`.
- **Zig** (`winget install -e --id zig.zig`) y `zig cc`.

Compilar a mano con MSVC (desde un *Developer Command Prompt*):

```bat
cl /O2 /W3 /I monocypher /Fe:verificar.exe ^
   verificar.c monocypher\monocypher.c monocypher\monocypher-ed25519.c
```

## Cómo ejecutar

Lo más fácil: el script hace todo (detecta el compilador, compila, corre y compara).

```bash
# Linux / macOS / MSYS2 (gcc, clang o zig cc)
bash run-poc.sh
```

```powershell
# Windows con MSVC (Visual Studio / Build Tools)
powershell -ExecutionPolicy Bypass -File run-poc.ps1
```

Salida esperada:

```
Casos individuales:
  ✔ token-valido                     OK
  ✔ token-expirado                   EXPIRED
  ✔ token-firma-mala                 BAD_SIGNATURE

Anti-reuso (mismo token dos veces en la misma sesión):
  ✔ 1er uso                          OK
  ✔ 2do uso                          ALREADY_USED

RESULTADO: TODO COINCIDE con resultados-esperados.md ✅
```

Uso manual del binario:

```bash
./verificar --pubkey ../../especificaciones/vectores-prueba/llave-publica-k1.txt \
            --now 1752460900 \
            ../../especificaciones/vectores-prueba/token-valido.txt
```

- `--now <epoch_s>`: hora simulada (en el ESP32 vendrá del **RTC DS3231**).
  Por defecto usa `1752460900`, el `NOW` de referencia de los vectores. Sin este
  valor de referencia, `token-valido` aparecería como `EXPIRED` (su `exp` es fijo).
- Se pueden pasar **varios** tokens: comparten el registro de `jti` usados, lo que
  permite demostrar `ALREADY_USED` pasando el mismo token dos veces.

## Limitaciones de la PoC (y qué cambia en el ESP32)

Esto es una prueba de la **lógica cripto**, no el firmware final. Al portar:

- **Registro de `jti`**: aquí es un arreglo en memoria; en el ESP32 va a **NVS/FRAM**
  (persistente) y se poda por `exp` vencido (contrato §5 del departamento).
- **Reloj**: aquí `now` es un parámetro; en el ESP32 viene del **RTC DS3231**.
- **Parseo de JSON**: la PoC usa un escáner simple de `"clave":valor` suficiente
  para el JSON compacto y controlado por el servidor. En el ESP32 conviene un parser
  robusto (p. ej. jsmn) o migrar a un formato binario firmado (COSE/CBOR, ver §6 del
  contrato) para reducir además el tamaño del QR.
- **Entrada**: aquí el token se lee de un archivo; en la máquina llega por **UART**
  desde el lector **GM65**.
- **Salida**: aquí se imprime `items`; en la máquina se acciona el **motor** y se
  espera el **sensor** de confirmación (con `DISPENSE_FAIL` como incidencia de soporte).

## Procedencia de Monocypher

Vendorizado desde la etiqueta estable **4.0.2** del repositorio oficial
(`github.com/LoupVaillant/Monocypher`, dominio público / CC0-BSD). Se incluye en el
repo para que la PoC sea reproducible sin dependencias externas.
