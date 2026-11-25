# Nira node image: builds nirad from source (cgo for go-ethereum secp256k1).
#   docker build -f deploy/docker/Dockerfile -t nira:local --build-arg VERSION=$(git describe --always) .
FROM golang:1.26-bookworm AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -trimpath -mod=readonly -buildvcs=false \
      -ldflags "-X github.com/cosmos/cosmos-sdk/version.Name=nira -X github.com/cosmos/cosmos-sdk/version.AppName=nirad -X github.com/cosmos/cosmos-sdk/version.Version=${VERSION}" \
      -o /out/nirad ./cmd/nirad

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl python3 iproute2 procps \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/nirad /usr/local/bin/nirad
COPY scripts/genesis/build_genesis.py /opt/nira/build_genesis.py
COPY scripts/node-profile.py /opt/nira/node-profile.py
COPY deploy/testnet/gen.py deploy/testnet/community.sh /opt/nira/
EXPOSE 26656 26657 1317 9090 8545 8546 26660
ENTRYPOINT ["nirad"]
CMD ["start", "--home", "/data/node"]
