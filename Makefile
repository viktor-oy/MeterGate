MODE ?= proxy

# Use mise for environment management if it's installed, otherwise fallback to system tools
MISE_EXEC ?= $(shell command -v mise >/dev/null 2>&1 && echo "mise x -- " || echo "")

ifeq ($(MODE),proxy)
COMPOSE_ARGS := -f infra/docker-compose.infra.yml -f infra/docker-compose.common.yml -f infra/docker-compose.proxy.yml
else ifeq ($(MODE),provider)
COMPOSE_ARGS := -f infra/docker-compose.infra.yml -f infra/docker-compose.common.yml -f infra/docker-compose.provider.yml
else
$(error MODE must be proxy or provider)
endif

.PHONY: help test test-unit test-intg test-cp test-dp smoke-test infra-up infra-deps-up infra-down infra-deps-down db-migrate-dev db-migrate-deploy dev

help:
	@printf '%s\n' "MeterGate targets: test test-unit test-intg test-cp test-dp smoke-test infra-up infra-deps-up infra-down infra-deps-down db-migrate-dev db-migrate-deploy"
infra-up: infra-deps-up
	@echo "Starting app in $(MODE) mode..."
	docker-compose $(COMPOSE_ARGS) up $(if $(BUILD),--build,) -d

dev:
	@chmod +x scripts/dev.sh
	./scripts/dev.sh $(if $(OVERRIDE),--override-already-running,)

infra-down:
	@echo "Stopping infrastructure..."
	docker-compose $(COMPOSE_ARGS) down -v

infra-deps-up:
	@echo "Starting infrastructure dependencies only..."
	docker-compose $(COMPOSE_ARGS) up -d postgres redis
	@echo "Waiting for databases to initialize..."
	sleep 5
	@echo "Applying database schema and seeding..."
	DATABASE_URL=postgresql://metergate:metergate@localhost:5432/metergate?schema=public $(MISE_EXEC)npx prisma db push --accept-data-loss
	DATABASE_URL=postgresql://metergate:metergate@localhost:5432/metergate?schema=public $(MISE_EXEC)npm run prisma:seed

infra-deps-down:
	@echo "Stopping infrastructure dependencies only..."
	docker-compose $(COMPOSE_ARGS) rm -f -s -v postgres redis

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