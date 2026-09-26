.PHONY: check-escapes pre-commit test lint

check-escapes:
	@echo "Running Escape Analysis (Enforcing Zero-Allocation on Hot Path)..."
	@go build -gcflags="-m" ./... 2>&1 | \
		grep "escapes to heap" | \
		grep -E "^(internal/policy|internal/cache|internal/proxy)/" | \
		grep -vF -f .escapeignore > .escapes_filtered || true; \
	if [ -s .escapes_filtered ]; then \
		echo "ERROR: Heap allocation detected in hot path!"; \
		cat .escapes_filtered; \
		rm -f .escapes_filtered; \
		exit 1; \
	else \
		echo "SUCCESS: Hot path is allocation-free!"; \
		rm -f .escapes_filtered; \
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
