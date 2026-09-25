FROM golang:1.26.8-bookworm@sha256:bc6beb46032d45f421cf400036bf031cdc64f683ba9cdc124e31d063e71670bd AS builder

WORKDIR /vt/src/vitess.io/vitess

COPY . .

RUN CGO_ENABLED=0 go build -trimpath -o /vt/bin/vtctldclient ./go/cmd/vtctldclient

FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get upgrade -qq && \
    apt-get install -qq --no-install-recommends jq curl ca-certificates && \
    apt-get autoremove && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /vt/bin/vtctldclient /usr/bin/

RUN groupadd -r --gid 2000 vitess && \
    useradd -r -g vitess --uid 1000 vitess

USER vitess

CMD ["/usr/bin/vtctldclient"]
