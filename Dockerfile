# Multi-stage build para producción en Railway
FROM golang:alpine AS builder

WORKDIR /app

# Descargar módulos de Go
COPY go.mod go.sum ./
RUN go mod download

# Copiar el código fuente y compilar binario estático minimalista
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/gateway ./cmd/gateway

# Imagen final ultraligera (<25 MB) con huella de RAM mínima (~18 MB)
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/gateway /app/gateway

EXPOSE 8000

CMD ["/app/gateway"]
