# MeterGate

MeterGate is a CV-grade, high-performance API metering and quota enforcement gateway. It demonstrates FAANG-level engineering standards by strictly separating management operations from the high-velocity data path.

## System Architecture

The architecture is split into two distinct planes:

1. **Control Plane (NestJS):** Handles all CRUD operations for Tenants, Plans, and API Keys. It uses PostgreSQL as the source of truth and asynchronously publishes state changes to the Redis cluster.
2. **Data Plane (Go 1.22+):** A highly concurrent, zero-allocation reverse proxy that sits on the hot path. It validates API keys and enforces distributed rate limits at over 50,000 QPS using local L1 caches and a sharded Redis L2 topology.

## Engineering Decisions & Trade-offs

### 1. Dual-Protocol Control Plane API
The NestJS Control Plane serves two distinct audiences using the exact same underlying service logic:
- **OpenAPI REST (`/docs`):** Optimized for DevOps, CI/CD automation, and Infrastructure-as-Code (Terraform/Pulumi). We intentionally avoided building a React "Admin Dashboard" because elite infrastructure products manage state via code, not clicks.
- **GraphQL (`/graphql`):** Optimized for the Traffic Simulator UI to prevent data over-fetching when querying nested Tenant and Plan data.

### 2. Elite Operational Patterns
The system adheres to strict Site Reliability Engineering (SRE) patterns:
- **Crash-Only Philosophy:** The Go Data Plane instantly panics (`log.Fatal`) if it loses connectivity to critical infrastructure on boot, delegating recovery to Docker/K8s restart policies.
- **Strict Config Validation:** The system refuses to boot with missing or malformed configuration variables.
- **Pre-Commit Shift-Left:** The Go `Makefile` strictly enforces `check-escapes` (Escape Analysis) on every commit to mathematically guarantee zero heap allocations in the proxy hot path.

## Local Development

MeterGate uses `.mise.toml` to strictly lock development tooling versions.
Ensure you have `mise` installed, then simply run `mise install` to provision Go 1.22 and Node.js.

### Starting the Stack
*(Docker Compose instructions will be populated in subsequent stages as the Go Data Plane is containerized).*
