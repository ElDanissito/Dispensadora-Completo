-- Se ejecuta SOLO cuando el volumen de datos está vacío (primer arranque o tras
-- `docker compose down -v`). Crea una base APARTE para los tests, para que
-- `go test` (que hace TRUNCATE) nunca borre los datos de la app en `grabi`.
CREATE DATABASE grabi_test;
