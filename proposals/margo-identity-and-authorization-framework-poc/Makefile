.PHONY: all build test unit conformance integration e2e up up-ra up-interop up-spire up-daemon-test dev-up dev-env down clean tidy pki e2e-daemon

GO ?= go
GOFLAGS ?=
BIN := bin

all: build

build:
	mkdir -p $(BIN)
	$(GO) build $(GOFLAGS) -o $(BIN)/mis ./cmd/mis
	$(GO) build $(GOFLAGS) -o $(BIN)/device-agent ./cmd/device-agent
	$(GO) build $(GOFLAGS) -o $(BIN)/spire-workload ./cmd/spire-workload

unit:
	$(GO) test $(GOFLAGS) ./pkg/... ./cmd/...

conformance:
	$(GO) test $(GOFLAGS) ./test/conformance/...

integration:
	$(GO) test $(GOFLAGS) -tags=integration ./test/integration/...

e2e:
	$(GO) test $(GOFLAGS) -tags=e2e ./test/e2e/...

test: unit conformance

up: pki
	COMPOSE_PROFILES=intermediate docker compose up -d --build

up-ra: pki
	COMPOSE_PROFILES=ra docker compose up -d --build

up-interop: pki
	COMPOSE_PROFILES=ra,spire docker compose up -d --build

up-spire: pki
	COMPOSE_PROFILES=spire docker compose up -d --build

up-daemon-test: pki
	COMPOSE_PROFILES=daemon-test docker compose up -d --build

e2e-daemon: build up-daemon-test
	MIAF_E2E_DAEMON=1 $(GO) test $(GOFLAGS) -tags=e2e -run TestDaemon_ ./test/e2e/ -timeout 10m

DEV_ETC_DIR  := var/dev/mis-etc
DEV_DATA_DIR := var/dev/mis
DEV_AGENT_DIR := var/dev/device-agent

# dev-up starts step-ca in Docker, populates $(DEV_ETC_DIR)/ with all config
# and key material MIS needs, and creates the device-agent state directory.
#
# After this target completes, run MIS locally with:
#   source <(make -s dev-env) && ./bin/mis
dev-up: pki build
	docker compose up -d step-ca --wait
	mkdir -p $(DEV_ETC_DIR)/ca $(DEV_ETC_DIR)/stepca $(DEV_DATA_DIR) $(DEV_AGENT_DIR)
	docker compose run --rm --no-deps \
		-v $(CURDIR)/$(DEV_ETC_DIR)/stepca:/stepca-out \
		-v $(CURDIR)/$(DEV_ETC_DIR)/ca:/ca-out \
		--entrypoint sh \
		step-ca -c 'cp "$$CRED_OUT/provisioner.json" "$$CRED_OUT/password" /stepca-out/ && cp "$$CRED_OUT/root.crt" /ca-out/'
	COMPOSE_PROFILES=intermediate docker compose run --rm --no-deps \
		-v $(CURDIR)/$(DEV_ETC_DIR)/ca:/out \
		mis-intermediate-bootstrap
	@echo ""
	@echo "────────────────────────────────────────────────────────────"
	@echo " Local dev quick-start"
	@echo "────────────────────────────────────────────────────────────"
	@echo " MIS (RA mode):"
	@echo "   source <(make -s dev-env) && ./bin/mis"
	@echo ""
	@echo " MIS (intermediate mode):"
	@echo "   source <(make -s dev-env) && MIS_ISSUER_BACKEND=intermediate ./bin/mis"
	@echo ""
	@echo " device-agent:"
	@echo "   ./bin/device-agent --data-dir $(DEV_AGENT_DIR) --mis-trust-anchor $(DEV_ETC_DIR)/tls/server.crt <command>"
	@echo ""
	@echo " SPIRE interop (device must be enrolled):"
	@echo "   make up-spire"
	@echo "   ./bin/device-agent --data-dir $(DEV_AGENT_DIR) --mis-trust-anchor $(DEV_ETC_DIR)/tls/server.crt interop-call --server localhost:8444"
	@echo "────────────────────────────────────────────────────────────"

# Emit shell exports for running MIS locally against the dev step-ca.
# Covers both backends; override MIS_ISSUER_BACKEND to switch modes:
#   source <(make -s dev-env) && ./bin/mis                                  # RA mode (default)
#   source <(make -s dev-env) && MIS_ISSUER_BACKEND=intermediate ./bin/mis  # intermediate mode
dev-env:
	@echo 'export MIS_ETC_DIR=$(CURDIR)/$(DEV_ETC_DIR)'
	@echo 'export MIS_DATA_DIR=$(CURDIR)/$(DEV_DATA_DIR)'
	@echo 'export MIS_ISSUER_BACKEND=stepca-ra'
	@echo 'export MIS_TRUST_DOMAIN=margo.example.com'
	@echo 'export MIS_BIND_ADDRESS=0.0.0.0:8443'
	@echo 'export MIS_ADVERTISE_URL=https://localhost:8443'
	@echo 'export MIS_ADMIN_BIND_ADDRESS=127.0.0.1:8445'
	@echo 'export MIS_ADMIN_URL=http://127.0.0.1:8445'
	@echo 'export MIS_SVID_TTL=24h'
	@echo 'export MIS_ALLOW_KEY_ROTATION=true'
	@echo 'export MIS_ALLOW_NEW_ENROLLMENT=true'
	@echo 'export MIS_STEPCA_URL=https://localhost:9000'
	@echo 'export MIS_STEPCA_PROVISIONER_NAME=margo-mis'

pki:
	./scripts/gen-stepca-password.sh
	./scripts/gen-mis-tls.sh
	./scripts/gen-factory-pki.sh
	./scripts/gen-mis-jwtsvid-key.sh

.PHONY: pki

down:
	COMPOSE_PROFILES=intermediate,ra,spire,daemon-test docker compose down -v

clean:
	rm -rf $(BIN) var/dev/

tidy:
	$(GO) mod tidy
