# Vectores de prueba — resultados esperados

> Generado por `dsp vectors`. Fuente: contrato-token.md v1.
> El simulador de verificación (02) y el firmware (03) DEBEN dar exactamente estos resultados.

## Parámetros de evaluación

- **MACHINE_ID de la máquina de prueba:** `M001`
- **kid → llave pública:** `k1` → ver `llave-publica-k1.txt`
  - base64: `L46okF6kj8SN742gYX9zNQD1P+V2ryNh0j4Yw29c9js=`
- **NOW de referencia (RTC simulado):** `1752460900` (epoch s)
- **jti usados al inicio:** ninguno (lista vacía)

> Los tokens llevan `exp` FIJO. Para reproducir los resultados hay que evaluar
> con `now = 1752460900`, no con la hora real (si no, `token-valido` aparecería expirado).

## Casos

| Archivo | Resultado esperado | Por qué |
|---------|--------------------|---------|
| `token-valido.txt` | `OK` | Firma válida, iss/mid correctos, `now (1752460900) ≤ exp (1752461100)`, jti no usado. Dispensa items `[{s:3,q:1},{s:5,q:2}]`. |
| `token-expirado.txt` | `EXPIRED` | Firma válida pero `now (1752460900) > exp (1752460300)`. Se rechaza en el paso 8. |
| `token-firma-mala.txt` | `BAD_SIGNATURE` | Igual que el válido pero con la firma corrompida. Se rechaza en el paso 4 (antes de mirar exp). |

## Nota sobre segundo uso (ALREADY_USED)

Si se verifica `token-valido.txt` DOS veces con el mismo registro de `jti`,
la primera da `OK` y la segunda `ALREADY_USED` (el jti `ord_valid01` queda marcado).

## Tokens (para inspección)

```
token-valido:     eyJhbGciOiJFZERTQSIsInR5cCI6IkRTUCIsImtpZCI6ImsxIn0.eyJpc3MiOiJkaXNwZW5zYWRvcmFzLmNvIiwibWlkIjoiTTAwMSIsImp0aSI6Im9yZF92YWxpZDAxIiwiaWF0IjoxNzUyNDYwODAwLCJleHAiOjE3NTI0NjExMDAsIml0ZW1zIjpbeyJzIjozLCJxIjoxfSx7InMiOjUsInEiOjJ9XX0.DJDLd3L20yEW4hO9IdbCjnQCEuxi3dceh4V9sfs6Be534a0edAS5nXI0WS1VmdL1Vw2zgDCD94TLAGZSz3pAAg
token-expirado:   eyJhbGciOiJFZERTQSIsInR5cCI6IkRTUCIsImtpZCI6ImsxIn0.eyJpc3MiOiJkaXNwZW5zYWRvcmFzLmNvIiwibWlkIjoiTTAwMSIsImp0aSI6Im9yZF9leHAwMSIsImlhdCI6MTc1MjQ2MDAwMCwiZXhwIjoxNzUyNDYwMzAwLCJpdGVtcyI6W3sicyI6MywicSI6MX0seyJzIjo1LCJxIjoyfV19._IwQ9THP53cmrG1Jmi1lAhtKs9Us9Q25qBl_lGA8f4cpHwo_Dk28oC9QbxqRO-lENaq2aC4KNkZGcxahrJNGCQ
token-firma-mala: eyJhbGciOiJFZERTQSIsInR5cCI6IkRTUCIsImtpZCI6ImsxIn0.eyJpc3MiOiJkaXNwZW5zYWRvcmFzLmNvIiwibWlkIjoiTTAwMSIsImp0aSI6Im9yZF9iYWRzaWcwMSIsImlhdCI6MTc1MjQ2MDgwMCwiZXhwIjoxNzUyNDYxMTAwLCJpdGVtcyI6W3sicyI6MywicSI6MX0seyJzIjo1LCJxIjoyfV19.Pmra3j3Hfv8MXHjCVNIgUNi5oWnF48qm7k1O2AANPWljjd64S56kcvx4A2ebsV3ecJhryqbMVr0OGxB7TRNGBA
```
