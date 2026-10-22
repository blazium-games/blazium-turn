# syntax=docker/dockerfile:1
FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /turn ./cmd/turn

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /turn /turn
EXPOSE 3478/udp 3478/tcp 3479
ENTRYPOINT ["/turn"]
CMD ["-addr", "0.0.0.0:3478", "-control-addr", "0.0.0.0:3479", "-auth=open"]
