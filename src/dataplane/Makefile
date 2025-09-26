.PHONY: check-escapes pre-commit test lint

check-escapes:
	@echo "Running Escape Analysis..."
	go build -gcflags="-m -m" ./...

test:
	@echo "Running tests with race detector..."
	go test -v -race ./...

lint:
	@echo "Running govet..."
	go vet ./...

pre-commit: lint test check-escapes
	@echo "Pre-commit checks passed successfully."
