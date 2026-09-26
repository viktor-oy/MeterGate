# Use mise for environment management if it's installed, otherwise fallback to system tools
MISE_EXEC ?= $(shell command -v mise >/dev/null 2>&1 && echo "mise x -- " || echo "")

CP_REPLICAS ?= 1
DP_REPLICAS ?= 1
CLUSTER_MODE ?= 0

ifneq ($(CP_REPLICAS), 1)
  CLUSTER_MODE = 1
endif
ifneq ($(DP_REPLICAS), 1)
  CLUSTER_MODE = 1
endif

ifeq ($(CLUSTER_MODE), 1)
  export REDIS_CLUSTER_MODE := true
  INFRA_COMPOSE := infra/docker-compose.cluster.yml
  DEPS_TARGETS := postgres redis redis-node-2 redis-node-3 redis-cluster-init prometheus grafana
else
  INFRA_COMPOSE := infra/docker-compose.infra.yml
  DEPS_TARGETS := postgres redis prometheus grafana
endif

COMPOSE_ARGS := -f $(INFRA_COMPOSE) -f infra/docker-compose.common.yml

.PHONY: help test test-unit test-intg test-cp test-dp smoke-test infra-up infra-deps-up infra-down infra-deps-down db-migrate-dev db-migrate-deploy dev test-telemetry

help:
	@printf '%s\n' "MeterGate targets: test test-unit test-intg test-cp test-dp smoke-test infra-up infra-deps-up infra-down infra-deps-down db-migrate-dev db-migrate-deploy"
infra-up: infra-deps-up
	@echo "Starting app..."
	docker-compose $(COMPOSE_ARGS) up $(if $(BUILD),--build,) -d

dev:
	@chmod +x scripts/dev.sh
	./scripts/dev.sh $(if $(OVERRIDE),--override-already-running,)

infra-down:
	@echo "Stopping infrastructure..."
	docker-compose $(COMPOSE_ARGS) down -v

infra-deps-up:
	@echo "Starting infrastructure dependencies only..."
	docker-compose $(COMPOSE_ARGS) up -d $(DEPS_TARGETS)
	@echo "Waiting for databases to initialize..."
	sleep 5
	@echo "Applying database schema and seeding..."
	DATABASE_URL=postgresql://metergate:metergate@localhost:5432/metergate?schema=public $(MISE_EXEC)npx prisma db push --accept-data-loss
	DATABASE_URL=postgresql://metergate:metergate@localhost:5432/metergate?schema=public $(MISE_EXEC)npm run prisma:seed

infra-deps-down:
	@echo "Stopping infrastructure dependencies only..."
	docker-compose $(COMPOSE_ARGS) rm -f -s -v $(DEPS_TARGETS)

db-migrate-dev:
	@echo "Creating Prisma migration..."
	DATABASE_URL=postgresql://metergate:metergate@localhost:5432/metergate?schema=public $(MISE_EXEC)npx prisma migrate dev

db-migrate-deploy:
	@echo "Deploying Prisma migrations..."
	DATABASE_URL=postgresql://metergate:metergate@localhost:5432/metergate?schema=public $(MISE_EXEC)npx prisma migrate deploy

test: smoke-test test-cp test-dp

test-cp-unit:
	@echo "Running Control Plane unit tests..."
	$(MISE_EXEC)npm run test:unit

test-cp-intg: infra-deps-up
	@echo "Running Control Plane integration tests..."
	$(MISE_EXEC)npm run test:integration

test-cp: test-cp-unit test-cp-intg

test-dp-unit:
	@echo "Running Data Plane unit tests..."
	cd src/dataplane && $(MISE_EXEC)go test -v -short ./...
	@echo "Running Escape Analysis (Zero Allocation Check)..."
	cd src/dataplane && $(MISE_EXEC)make check-escapes

test-dp-intg: infra-deps-up
	@echo "Running Data Plane integration tests..."
	cd src/dataplane && $(MISE_EXEC)go test -v -tags=integration -run=Integration ./...

test-dp: test-dp-unit test-dp-intg

test-unit: test-cp-unit test-dp-unit
test-intg: test-cp-intg test-dp-intg

smoke-test: infra-deps-up
	@echo "Running smoke tests..."
	./tests/smoke/smoke.sh

TEST_API_KEY ?= $(shell node -e "console.log(require('./seed.json').apiKey.plaintext)")
TEST_HEADER_NAME ?= x-api-key

test-telemetry:
	@echo "Running simple load generator to populate telemetry dashboard..."
	@for i in {1..50}; do \
		curl -s -o /dev/null -w "Req 1: %{http_code} " -H "$(TEST_HEADER_NAME): $(TEST_API_KEY)" http://localhost:8080/graphql; \
		curl -s -o /dev/null -w "Req 2: %{http_code} " -H "$(TEST_HEADER_NAME): $(TEST_API_KEY)" http://localhost:8080/tickets; \
		curl -s -o /dev/null -w "Req 3: %{http_code}\n" http://localhost:8080/graphql; \
		sleep 0.1; \
	done
	@echo "Load generation complete. Check Grafana at http://localhost:3000 (admin:admin)"
