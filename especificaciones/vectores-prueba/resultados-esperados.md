# Vectores de prueba — resultados esperados

> Generado por `dsp vectors`. Fuente: contrato-token.md v2.
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
| `token-valido.txt` | `OK` | Firma válida, `mid` correcto, `now (1752460900) ≤ exp (1752461100)`, jti no usado. Dispensa items `[{s:3,q:1},{s:5,q:2}]`. |
| `token-expirado.txt` | `EXPIRED` | Firma válida pero `now (1752460900) > exp (1752460300)`. Se rechaza en el paso 7 (v2). |
| `token-firma-mala.txt` | `BAD_SIGNATURE` | Igual que el válido pero con la firma corrompida. Se rechaza en el paso 4 (antes de mirar exp). |

## Nota sobre segundo uso (ALREADY_USED)

Si se verifica `token-valido.txt` DOS veces con el mismo registro de `jti`,
la primera da `OK` y la segunda `ALREADY_USED` (el jti `ord_valid01` queda marcado).

## Tokens (para inspección)

```
token-valido:     eyJhbGciOiJFZERTQSIsInR5cCI6IkRTUCIsImtpZCI6ImsxIn0.eyJtaWQiOiJNMDAxIiwianRpIjoib3JkX3ZhbGlkMDEiLCJleHAiOjE3NTI0NjExMDAsIml0ZW1zIjpbeyJzIjozLCJxIjoxfSx7InMiOjUsInEiOjJ9XX0.0vNclYuX8H9Bg09DZPuXCi62EV6MAsk2-c_IxHGxMIsdi9fT7BNNPwI9dfX8xjulkohcX3vgDHkvsSakShy3Cg
token-expirado:   eyJhbGciOiJFZERTQSIsInR5cCI6IkRTUCIsImtpZCI6ImsxIn0.eyJtaWQiOiJNMDAxIiwianRpIjoib3JkX2V4cDAxIiwiZXhwIjoxNzUyNDYwMzAwLCJpdGVtcyI6W3sicyI6MywicSI6MX0seyJzIjo1LCJxIjoyfV19.LUGxQIXV8ZBSLnA4S4RB77cJ57mScTrKXOzgvgRjvzpi-FKanyKAACnUBCLBHoKjUNY0bhDLJ444nd40tVjqCw
token-firma-mala: eyJhbGciOiJFZERTQSIsInR5cCI6IkRTUCIsImtpZCI6ImsxIn0.eyJtaWQiOiJNMDAxIiwianRpIjoib3JkX2JhZHNpZzAxIiwiZXhwIjoxNzUyNDYxMTAwLCJpdGVtcyI6W3sicyI6MywicSI6MX0seyJzIjo1LCJxIjoyfV19.w4Qh48yFKVdl2yymAfjkBPCuJKQukvWKuSTl-FVrwbUVQcIjvxm-RWauyhYxlWy95z0KYzFbLXCa7ezoTc74DA
```
