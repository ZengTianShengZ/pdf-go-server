FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/pdf-go-server .

FROM gotenberg/gotenberg:8

USER root
WORKDIR /app

COPY --from=builder /out/pdf-go-server /app/pdf-go-server
COPY template /app/template
COPY docker-entrypoint.sh /app/docker-entrypoint.sh

RUN chmod +x /app/pdf-go-server /app/docker-entrypoint.sh

# Bind :80 requires root (WeChat Cloud Run / single-port deploy).
USER root

ENV PORT=80 \
    GOTENBERG_URL=http://127.0.0.1:3000 \
    TEMPLATE_PATH=/app/template/v1-home-inspection.html \
    PDF_TIMEOUT_SEC=60

EXPOSE 80

ENTRYPOINT ["/app/docker-entrypoint.sh"]
