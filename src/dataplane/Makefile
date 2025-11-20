.PHONY: check-escapes pre-commit test lint

check-escapes:
	@echo "Running Escape Analysis (Enforcing Zero-Allocation on Hot Path)..."
	@go build -gcflags="-m" ./... 2>&1 | \
		grep "escapes to heap" | \
		grep "github.com/anahvictoronyedikachi/metergate/src/dataplane" | \
		grep -v "internal/config" | \
		grep -v "internal/diagnostics" | \
		grep -v "internal/telemetry" | \
		grep -v "main.go" | \
		grep -v "evalCtx leaks" || true; \
	if go build -gcflags="-m" ./... 2>&1 | grep "escapes to heap" | grep "github.com/anahvictoronyedikachi/metergate/src/dataplane" | grep -v "internal/config" | grep -v "internal/diagnostics" | grep -v "internal/telemetry" | grep -v "main.go" | grep -v "evalCtx leaks" | grep -q .; then \
		echo "ERROR: Heap allocation detected in hot path!"; \
		exit 1; \
	else \
		echo "SUCCESS: Hot path is allocation-free!"; \
		exit 0; \
	fi

test:
	@echo "Running tests with race detector..."
	go test -v -race ./...

lint:
	@echo "Running govet..."
	go vet ./...

pre-commit: lint test check-escapes
	@echo "Pre-commit checks passed successfully."
