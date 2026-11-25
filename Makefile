BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
COMMIT := $(shell git log -1 --format='%H')
APPNAME := nira
BINARY := $(APPNAME)d
BUILDDIR ?= $(CURDIR)/build
GOVULNCHECK_VERSION := v1.1.4

# go-ethereum's secp256k1 needs cgo (a C compiler)
export CGO_ENABLED := 1

# do not override user values
ifeq (,$(VERSION))
  VERSION := $(shell git describe --exact-match 2>/dev/null)
  # if VERSION is empty, then populate it with branch name and raw commit hash
  ifeq (,$(VERSION))
    VERSION := $(BRANCH)-$(COMMIT)
  endif
endif

ldflags = -X github.com/cosmos/cosmos-sdk/version.Name=$(APPNAME) \
	-X github.com/cosmos/cosmos-sdk/version.AppName=$(BINARY) \
	-X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
	-X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT)

BUILD_FLAGS := -ldflags '$(ldflags)' -trimpath

.DEFAULT_GOAL := help

help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

#################
###  Install  ###
#################

all: install

install: ## Build and install nirad into $(go env GOPATH)/bin
	@echo "--> ensure dependencies have not been modified"
	@go mod verify
	@echo "--> installing $(BINARY) $(VERSION)"
	@go install $(BUILD_FLAGS) -mod=readonly ./cmd/$(BINARY)

build: ## Build nirad into ./build/
	@mkdir -p $(BUILDDIR)
	@go build $(BUILD_FLAGS) -mod=readonly -o $(BUILDDIR)/$(BINARY) ./cmd/$(BINARY)
	@echo "--> $(BUILDDIR)/$(BINARY)"

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
RELEASE_DIR := $(BUILDDIR)/release
release: ## Reproducible release tarball + SHA256SUMS in build/release (needs a clean git tree)
	@test -z "$$(git status --porcelain)" || (echo "release: working tree not clean" && exit 1)
	@mkdir -p $(RELEASE_DIR)
	@go build $(BUILD_FLAGS) -mod=readonly -buildvcs=false -o $(RELEASE_DIR)/$(BINARY) ./cmd/$(BINARY)
	@cd $(RELEASE_DIR) && tar --sort=name --mtime='1970-01-01' --owner=0 --group=0 --numeric-owner \
	  -czf $(BINARY)-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz $(BINARY) && \
	  sha256sum $(BINARY) $(BINARY)-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz > SHA256SUMS && cat SHA256SUMS

clean: ## Remove build outputs
	@rm -rf $(BUILDDIR)

.PHONY: all install build release clean

##############
###  Test  ###
##############

test-unit: ## Go unit tests (modules, invariants, real IAVL proofs)
	@go test -mod=readonly -timeout 30m ./...

test-race: ## Go unit tests with the race detector
	@go test -mod=readonly -race -timeout 30m ./...

test-cover: ## Go unit tests with an HTML coverage report in ./build
	@mkdir -p $(BUILDDIR)
	@go test -mod=readonly -timeout 30m -coverprofile=$(BUILDDIR)/cover.out -covermode=atomic ./...
	@go tool cover -html=$(BUILDDIR)/cover.out -o $(BUILDDIR)/cover.html
	@echo "--> $(BUILDDIR)/cover.html"

FUZZTIME ?= 30s
fuzz: ## Run every fuzz target for FUZZTIME each (default 30s)
	@for f in FuzzVerifyStateProof FuzzVerifyReveal FuzzDeriveTaskAndSelection; do \
	  go test ./x/community/types -run '^$$' -fuzz "^$$f$$" -fuzztime $(FUZZTIME) || exit 1; done
	@go test ./x/community/keeper -run '^$$' -fuzz '^FuzzSubmitTaskProof$$' -fuzztime $(FUZZTIME)

test-genesis: ## Genesis builder validation tests (python3)
	@python3 -m unittest discover -s tests/genesis -p 'test_*.py'

bench:
	@go test -mod=readonly -timeout 30m -bench=. ./...

test: govet test-unit test-genesis ## vet + unit + genesis builder tests

e2e-deps: ## Install Node.js deps for the EVM e2e suites
	@cd tests/evm-e2e && npm ci --silent

e2e-cosmos: ## Cosmos-interface e2e (needs a running localnet)
	@tests/cosmos-e2e/e2e.sh
	@tests/cosmos-e2e/community.sh

e2e-evm: ## EVM-interface e2e (needs a running localnet and `make e2e-deps`)
	@KEY=$$(echo y | $(BINARY) keys unsafe-export-eth-key dev1 --keyring-backend test 2>/dev/null | tail -1); \
	  cd tests/evm-e2e && KEY=$$KEY node e2e.mjs && KEY=$$KEY node community.mjs && KEY=$$KEY node atomicity.mjs

e2e-gov: ## Governance e2e: param change (CLI + EVM vote), false-alert slashing (running localnet)
	@tests/cosmos-e2e/governance.sh

BENCH_CHAIN_ID ?= nira_47201-1
BENCH_HOME ?= $(HOME)/.nira
bench-load: ## Throughput baseline (native, evm, mixed) against a running network → docs/benchmarks/raw
	@go build -o $(BUILDDIR)/loadgen ./tools/loadgen
	@KEY=$$(echo y | $(BINARY) keys unsafe-export-eth-key dev1 --keyring-backend test --home $(BENCH_HOME) 2>/dev/null | tail -1); \
	  for m in native evm mixed; do $(BUILDDIR)/loadgen --mode $$m --accounts 100 --txs 50 --funder-key $$KEY \
	    --chain-id $(BENCH_CHAIN_ID) | tee docs/benchmarks/raw/latest-$$m.json; done

e2e: e2e-cosmos e2e-evm e2e-gov ## All e2e suites against a running localnet

export-import: install ## Genesis export → reset → import with state checks (stop other nodes first)
	@tests/upgrade/export_import.sh

upgrade-rehearsal: install ## Software upgrade with a real binary switch (stop other nodes first)
	@go build $(BUILD_FLAGS) -tags rehearsal -o $(BUILDDIR)/$(BINARY)-rehearsal ./cmd/$(BINARY)
	@tests/upgrade/rehearsal.sh

.PHONY: fuzz test test-unit test-race test-cover test-genesis bench e2e-deps e2e-cosmos e2e-evm e2e-gov e2e upgrade-rehearsal export-import bench-load

###################
###  Networks  ###
###################

localnet: install ## Init a 1-validator dev chain in ~/.nira (start it with `nirad start`)
	@scripts/localnet.sh

localtest: install ## 4-validator network via the full genesis ceremony (ports base+100·i)
	@scripts/genesis/localtest.sh

localtest-check: ## Verify the 4-validator network (allocations, BFT fault tolerance)
	@tests/genesis/check_localtest.sh

validators-test: install ## 16 validators (15 active): rotation, live cap, double-sign + downtime slashing
	@N=16 MAX_VALIDATORS=15 scripts/genesis/localtest.sh
	@sleep 20 && tests/consensus/check_validators.sh; status=$$?; scripts/genesis/localtest.sh stop; exit $$status

sync-test: install ## State sync (verified snapshot) + archive vs pruned node on a 4-validator net
	@scripts/genesis/localtest.sh
	@tests/sync/check_sync.sh; status=$$?; scripts/genesis/localtest.sh stop; exit $$status

localtest-stop: ## Stop the 4-validator network
	@scripts/genesis/localtest.sh stop

genesis-check: ## Validate a network file: make genesis-check NET=genesis/networks/mainnet.example.toml
	@python3 scripts/genesis/build_genesis.py check $(or $(NET),genesis/networks/mainnet.example.toml) --out $(BUILDDIR)/genesis

.PHONY: localnet localtest localtest-check localtest-stop validators-test sync-test genesis-check

##########################
###  Docker testnet  ###
##########################
VALIDATORS ?= 8
COMMUNITY ?= 6
docker-build: ## Build the nira:local image from source
	@python3 deploy/testnet/testnet.py build

testnet-up: docker-build ## Docker testnet: VALIDATORS (one behind a sentry) + COMMUNITY prove-loop nodes + RPC node
	@python3 deploy/testnet/testnet.py up --validators $(VALIDATORS) --community $(COMMUNITY)

testnet-test: ## Scenarios on the running Docker testnet: SCENARIO=health|sentry|community|partition|soak|all
	@python3 deploy/testnet/testnet.py test $(or $(SCENARIO),all)

testnet-down: ## Stop the Docker testnet and delete its data volume
	@python3 deploy/testnet/testnet.py down

.PHONY: docker-build testnet-up testnet-test testnet-down

##################
###  Protobuf  ###
##################

proto-gen: ## Regenerate Go code from proto/ (needs Ignite CLI)
	@ignite generate proto-go --yes

.PHONY: proto-gen

#################
###  Linting  ###
#################

lint: ## golangci-lint
	@go tool github.com/golangci/golangci-lint/cmd/golangci-lint run ./... --timeout 15m

lint-fix:
	@go tool github.com/golangci/golangci-lint/cmd/golangci-lint run ./... --fix --timeout 15m

govet: ## go vet
	@go vet ./...

govulncheck: ## Known-vulnerability scan of dependencies
	@go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

.PHONY: lint lint-fix govet govulncheck
