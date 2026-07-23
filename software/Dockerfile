# syntax=docker/dockerfile:1

# ---- build ----------------------------------------------------------------
# Compilación estática (CGO_ENABLED=0). El servidor usa Postgres vía pgx y QR/
# tokens en Go puro, así que no necesitamos cgo → imagen final mínima y portable.
FROM golang:1.25-alpine AS build
WORKDIR /src

# Cache de dependencias: copiar go.mod/go.sum primero.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Binario estático para la arquitectura destino. TARGETOS/TARGETARCH los rellena
# BuildKit automáticamente, así que la misma imagen compila bien tanto en EC2 x86
# (t3.micro) como ARM/Graviton (t4g.micro) sin tocar el Dockerfile (ADR-021).
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# Carpeta de uploads con dueño 'nonroot'. Al montar aquí un volumen persistente
# VACÍO (docker-compose.prod.yml), Docker copia el dueño de este directorio al
# volumen → el proceso (nonroot) puede escribir las fotos. Sin esto, el volumen
# nace root y las subidas fallarían con "permission denied" (ADR-021).
RUN mkdir -p /data/uploads

# ---- runtime --------------------------------------------------------------
# distroless static: sin shell ni gestor de paquetes → superficie de ataque
# mínima. Corre como usuario 'nonroot'.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/server /app/server
# Punto de montaje de las fotos subidas, propiedad de nonroot (ver build stage).
COPY --from=build --chown=nonroot:nonroot /data /data

# El servidor escucha en 8080; en la EC2 el reverse-proxy (Caddy/nginx) enruta ahí.
EXPOSE 8080

# Config por defecto para producción:
#  -concil        : arranca el loop de conciliación por correo (siempre activo).
#  -uploads=/tmp  : ruta de subidas DENTRO del contenedor. En la EC2 (ADR-021) se
#                   monta sobre un volumen/bind-mount persistente, así que las fotos
#                   SOBREVIVEN al redeploy (a diferencia de App Runner). Mover a S3
#                   queda como opción al escalar, no urgente.
# Los secretos (DATABASE_URL, GRABI_IMAP_*, DSP_PRIVATE_KEY, ADMIN_PASS,
# GRABI_BREB_KEY_*) se inyectan como variables de entorno desde el .env
# git-ignored de la EC2 (permisos 600), nunca horneados en la imagen.
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
CMD ["-addr=:8080", "-concil", "-uploads=/tmp/uploads"]
