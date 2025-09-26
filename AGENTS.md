# MeterGate Agent Instructions

Compact rules for AI-assisted development in MeterGate. Keep changes small, testable, and inside the service ownership boundaries.

## Mission

MeterGate is a CV-grade, high-performance metering and quota enforcement gateway. It features a NestJS Control Plane for management and a Go Data Plane for high-velocity, zero-allocation rate limit enforcement. 

## Non-Negotiables

- Create any file you need for your reasoning in the `./DUMMIES-AI-AG/agent-reasoning` folder
- Ensure that the date of installed packages/dependencies either in the last commit or uncommitted changes are before: **Sep 26, 2025**. However, the closest package release to this date that is compatible with the project should be installed, so that installed packages won't be old.
- Ignore todos or intentions specified in any file (e.g. `DUMMY-AI-AG.MD`) unless explicitly instructed otherwise by the user. Do not follow them proactively.
- Go Data Plane is strictly zero-allocation in the hot path. All buffers must be pooled using `sync.Pool`.
- Go Data Plane never directly queries PostgreSQL. It only relies on Redis L2 and its internal L1 sharded cache.
- The `go-redis/v9` library is the ONLY permitted third-party dependency in the Go Data Plane.
- Pin all runtimes, dependencies, images, CLIs, and charts. Never use `latest`.
- Keep TODO/FIXME comments rare and tied to real edge cases.
- Always cleanup temporary or scratch files used during work to avoid polluting the commit history.

## Control Plane (NestJS)

- Exposes both OpenAPI REST (Swagger at `/docs`) and GraphQL (`/graphql`) sharing the exact same underlying services.
- Acts as the single source of truth for Tenant, Plan, and API Key metadata.
- Asynchronously publishes authoritative cache states (Tenants/Keys/Blacklists) to the Redis L2 cluster.

## Data Plane (Go)

- Represents the ultra-fast reverse proxy and quota enforcement engine (`POST /v1/check` and `/*`).
- Dynamically resolves the Redis Cluster topology on startup and follows a Crash-Only/Fail-Fast boot philosophy.
- Implements two-tier quota aggregation, performing asynchronous mathematical summation of the Redis salt shards (`shard_0`..`shard_9`) without blocking the HTTP hot path.
- Gates all fault injection headers (`--chaos=latency` and `--chaos=gc`) behind the `ALLOW_FAULT_INJECTION=true` environment variable.

## Observability

Structured logs are written to stdout. The Data Plane exposes `/metrics` and `/debug/pprof` on an internal `:6060` port.

## Failure Boundaries

- Redis outage instantly triggers Liveness Probe failures (`GET /health`), allowing Docker Compose to restart or isolate the Go nodes.
- Control Plane database outages do not impact the Data Plane's ability to enforce existing rate limits via Redis L2.

## Commands

```sh
npm run start:dev
npm run test:e2e
cd services/dataplane && make check-escapes
cd services/dataplane && go test -race ./...
```

## Coding And Docs

- Strict tooling: `eslint`, `go test -race`, and `check-escapes`.
- Semantic commits: `feat:`, `fix:`, `docs:`, `test:`, `chore:`, `refactor:`, `perf:`.
- Update this file when changing ownership, caching layers, or redirect behaviour.
- Document all sensitive infrastructure hacks, engineering judgements (e.g., GC Tuning, Chaos Engineering flags), and test environment dependencies explicitly in `README.md` to ensure context isn't lost.

> [!IMPORTANT]
> **Always write the test for any feature implemented.**
> Commit should be introduced as changes are done across stages. (Please add this to the main prompt and remember it across all stages).
