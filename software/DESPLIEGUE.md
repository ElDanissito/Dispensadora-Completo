# Despliegue de GRABI — EC2 + Docker Compose (ADR-021)

Guía para poner el backend en producción sobre una **EC2 pequeña** con el
`docker-compose.prod.yml` de este repo. Elegimos EC2 (no App Runner, cerrado a
clientes nuevos el 2026-04-30) por **control total** y costo casi nulo en el
piloto (free tier).

## Arquitectura

```
Internet ──▶ Caddy (80/443, TLS auto)  ──▶  web (:8080, Go)  ──▶  db (Postgres 16)
             grabi.napi.lat                  -concil               volumen grabi_pgdata
             volumen caddy_data              volumen grabi_uploads
```

Todo corre en **una** EC2 con Docker Compose. Solo Caddy expone puertos (80/443);
`web` y `db` quedan en la red interna. Postgres es un contenedor (no RDS todavía);
las fotos y la base viven en **volúmenes persistentes** que sobreviven a redeploys.

## Fase 2 — Pasos para levantar (se hace CON Daniel)

### 1. Crear la instancia
- Tipo: **`t4g.micro`** (ARM/Graviton) o **`t3.micro`** (x86) — ambas free tier.
  El `Dockerfile` compila para la arquitectura de la caja automáticamente.
- SO: Amazon Linux 2023 o Ubuntu 24.04.
- **Elastic IP** asociada (para que la IP no cambie al reiniciar).
- **Security group:** entrada `22` (SSH, idealmente solo tu IP), `80` y `443` (Caddy).
  NO abrir 5432 ni 8080 al exterior.

### 2. Instalar Docker + Compose en la EC2
```bash
# Amazon Linux 2023:
sudo dnf install -y docker git
sudo systemctl enable --now docker
sudo usermod -aG docker ec2-user   # reconectar la sesión SSH tras esto
# Plugin de compose:
sudo dnf install -y docker-compose-plugin   # o descargar el plugin manualmente
```

### 3. DNS
Apuntar un registro **A** de `grabi.napi.lat` a la **Elastic IP** de la EC2.
Verificar con `dig +short grabi.napi.lat` ANTES de levantar (Caddy necesita que
el dominio resuelva para emitir el certificado TLS).

### 4. Clonar el repo y configurar secretos
```bash
git clone <repo> && cd <repo>/software
cp .env.example .env
nano .env            # rellenar TODOS los valores (ver .env.example)
chmod 600 .env       # solo el dueño lee los secretos
```
Valores críticos del `.env`:
- `POSTGRES_PASSWORD` — contraseña fuerte del Postgres interno.
- `ADMIN_PASS` — contraseña del panel `/admin`.
- `DSP_PRIVATE_KEY` — base64 de la llave privada Ed25519 (de `dsp keygen`). **Sin
  ella no se pueden emitir QR.** La pública ya está en el firmware.
- `GRABI_IMAP_PASS` — App Password de Gmail de `grabibot`.
- `GRABI_BREB_KEY_M001` — alias Bre-B de la máquina M001 (se muestra al pagar).

### 5. Levantar
```bash
docker compose -f docker-compose.prod.yml up -d --build
docker compose -f docker-compose.prod.yml logs -f
```
Esperar a que Caddy emita el certificado (primer arranque tarda unos segundos).

### 6. Validar
- `https://grabi.napi.lat/m/M001` carga con candado (TLS OK).
- Panel: `https://grabi.napi.lat/admin/login`.
- Log de `web`: "conciliación por correo" activa (NO "DESHABILITADA").
- **Prueba de pago real** por Bre-B → conciliación emite QR → dispensa.

## Operación

- **Ver logs:** `docker compose -f docker-compose.prod.yml logs -f web`
- **Redeploy tras `git pull`:** `docker compose -f docker-compose.prod.yml up -d --build`
  (los volúmenes persisten: base, fotos y certificados NO se pierden).
- **Backup de la base (pendiente de automatizar en Fase 4):**
  ```bash
  docker compose -f docker-compose.prod.yml exec db \
    pg_dump -U grabi grabi | gzip > backup-$(date +%F).sql.gz
  ```

## Pendiente (fases siguientes)
- **Fase 3 — CI/CD:** GitHub Actions + OIDC para redeploy automático en push a `main`.
- **Fase 4 — Backups + hardening:** `pg_dump` por cron → S3; endurecer SSH.
