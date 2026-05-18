# TradeMind API — multi-stage build (do not COPY .env into the image; inject at runtime).
FROM golang:1.26-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/trademind-backend ./cmd/server

FROM debian:bookworm-slim AS runtime

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates tzdata curl \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/trademind-backend /app/trademind-backend

ENV TZ=UTC

EXPOSE 8080

CMD ["/app/trademind-backend"]
