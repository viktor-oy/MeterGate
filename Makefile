.PHONY: help test test-unit test-intg test-controlplane test-dataplane smoke-test

help:
	@printf '%s\n' "MeterGate targets: test test-unit test-intg test-controlplane test-dataplane smoke-test"

test: test-controlplane test-dataplane

test-unit:
	@echo "Running unit tests across all services..."
	npm run test
	cd src/dataplane && go test -v -short ./...

test-intg:
	@echo "Running integration tests..."
	npm run test:e2e
	cd src/dataplane && go test -v -tags=integration ./...

test-controlplane:
	@echo "Running Control Plane tests..."
	npm run test
	npm run test:e2e

test-dataplane:
	@echo "Running Data Plane tests..."
	cd src/dataplane && go test -v -race ./...

smoke-test:
	@echo "Running smoke tests..."
	./scripts/smoke-proxy.sh
	./scripts/smoke-provider.sh