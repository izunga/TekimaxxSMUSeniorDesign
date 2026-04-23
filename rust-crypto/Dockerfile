FROM rust:1.75 AS builder

WORKDIR /app

COPY . .

RUN cargo build --release

FROM debian:bookworm-slim

RUN groupadd --gid 1001 appgroup && useradd --uid 1001 --gid appgroup --no-create-home appuser

WORKDIR /app
COPY --from=builder /app/target/release/rust-crypto ./rust-crypto

USER appuser
CMD ["./rust-crypto"]
