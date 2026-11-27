FROM golang:1.27.3-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /spill-api ./cmd/main.go


FROM alpine:3.20

RUN apk add --no-cache ca-certificates wget \
    && adduser -D -u 10001 appuser

WORKDIR /app
COPY --from=builder /spill-api /app/spill-api

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8080/health >/dev/null 2>&1 || exit 1

CMD ["/app/spill-api"]