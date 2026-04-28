<<<<<<< Updated upstream
# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /ledger-engine ./cmd/api

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app
COPY --from=builder /ledger-engine .
COPY migrations ./migrations

USER appuser
EXPOSE 8080
CMD ["./ledger-engine"]
=======
# ---- Build stage ----
FROM golang:1.22 AS builder

WORKDIR /app

# Copy go mod files first
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o ledger-api ./cmd/api

# ---- Run stage ----
FROM gcr.io/distroless/base-debian12

WORKDIR /app

COPY --from=builder /app/ledger-api /app/ledger-api

EXPOSE 8080

CMD ["/app/ledger-api"]
>>>>>>> Stashed changes
