# Build and install

## Supported platforms

| Platform | Status |
|---|---|
| Linux x86-64 (Ubuntu 24.04, also under WSL2) | **Tested.** All builds and test suites in this repo run here |
| Linux arm64 | Expected to work (pure Go + cgo), not tested |
| macOS (Apple Silicon / Intel) | Expected to work with Xcode Command Line Tools, not tested |
| Windows | Use **WSL2** (Ubuntu). A native Windows build needs a MinGW cgo toolchain and is not tested |

## Prerequisites

| Tool | Version | Needed for | Check |
|---|---|---|---|
| Go | **1.26.7+** (from `go.mod`) | building `nirad` | `go version` |
| C compiler (gcc or clang) | any recent | **required**: go-ethereum's secp256k1 uses cgo | `gcc --version` |
| make, git | any | build scripts, version stamping | |
| Python | 3.11+ (`tomllib`) | genesis builder, test scripts | `python3 --version` |
| Node.js + npm | 18+ (tested 22) | EVM e2e tests only | `node --version` |
| curl | any | e2e scripts | |
| Ignite CLI | optional | only to regenerate protobuf code (`make proto-gen`) | `ignite version` |

### Ubuntu / Debian / WSL2

```bash
sudo apt-get update
sudo apt-get install -y build-essential git make curl python3
# Go (adjust the version/arch as needed)
curl -LO https://go.dev/dl/go1.26.7.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.26.7.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc && source ~/.bashrc
# Node.js (only for the EVM e2e tests), e.g. via your package manager or nvm
```

On WSL2, keep the repository on the Linux filesystem (`~/…`), not under `/mnt/c` or `/mnt/d`. Builds across the Windows mount are much slower, and some file renames fail there.

### macOS

```bash
xcode-select --install          # clang for cgo
brew install go python node git
```

## Build

```bash
git clone https://github.com/nnlgsakib/nira-chain.git
cd nira-chain
make install        # → $(go env GOPATH)/bin/nirad
nirad version
```

Make sure `$(go env GOPATH)/bin` is on your `PATH`. `make build` writes the binary to `./build/nirad` instead.

The version string is the git tag, or `<branch>-<commit>` for an untagged build.

## Make targets

Run `make help` for the full list.

| Target | What it does |
|---|---|
| `make install` / `make build` | Build `nirad` (to GOPATH/bin or `./build/`) |
| `make test` | `go vet` + Go unit tests + genesis builder tests |
| `make test-race`, `make test-cover` | Race detector / HTML coverage report (`build/cover.html`) |
| `make localnet` | Install, then initialise a 1-validator dev chain in `~/.nira`. Start it with `nirad start` |
| `make e2e-deps` | `npm ci` for the EVM e2e suites |
| `make e2e` | All Cosmos and EVM e2e suites against a running localnet |
| `make localtest` / `localtest-check` / `localtest-stop` | 4-validator network through the full genesis ceremony, its checks, and stopping it |
| `make genesis-check NET=genesis/networks/<net>.toml` | Validate a network file and print the allocation report |
| `make release` / `scripts/verify-reproducible.sh` | Release tarball + SHA256SUMS; check bit-for-bit reproducibility from two clean clones |
| `make lint`, `make govet`, `make govulncheck` | Static checks and the dependency vulnerability scan |
| `make proto-gen` | Regenerate Go code from `proto/` (needs Ignite CLI) |

## Verify your build

```bash
make test                         # unit + invariant + genesis-builder tests
make localnet && nirad start      # in one terminal
make e2e-deps && make e2e         # in another: 32 checks across the Cosmos and EVM interfaces
```

## Troubleshooting

| Symptom | Cause / fix |
|---|---|
| `undefined: secp256k1.RecoverPubkey` | cgo is disabled or there is no C compiler. Install gcc/clang; the Makefile sets `CGO_ENABLED=1` |
| `invalid key algorithm, got secp256k1, expected eth_secp256k1` | The key was created by another tool (for example `ignite chain serve`). Create keys with `nirad keys add <name>`, which defaults to eth_secp256k1 |
| `go: go.mod requires go >= 1.26.7` | Upgrade Go, or let the Go toolchain download it (`GOTOOLCHAIN=auto`) |
| JSON-RPC `:8545` refuses connections | `[json-rpc] enable = false` in `~/.nira/config/app.toml`. `scripts/localnet.sh` enables it for dev |
| `nirad: command not found` | Add `$(go env GOPATH)/bin` to `PATH` |
| `Permission denied` when renaming files under `/mnt/<drive>` (WSL2) | A Windows process holds the folder. Work on the Linux filesystem |
