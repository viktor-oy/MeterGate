# Stage 1 Commit Strategy

The user explicitly requested that commits contain **only related files** and unnecessary files should be excluded.

Because I performed a `git reset --soft` to the root, there are over 70 files in the staging area encompassing the Control Plane, Data Plane, Sample Apps (React/Laravel), and Tests.

To strictly adhere to the "related files only" rule, I will break the initialization into logical, cohesive commits instead of one giant squash:

1. **Commit 1 (Control Plane Foundation):** 
   - Root configuration (`.mise.toml`, `AGENTS.md`, `package.json`, `tsconfig`, etc).
   - NestJS core structure (`src/`, `prisma/`, `config/`).
   - The dual-protocol setup (`src/main.ts`, `src/app.module.ts`).
   - The Go Data Plane scaffolding (`services/dataplane/go.mod`, `Makefile`).
   - The updated `README.md`.
2. **Commit 2 (To be done in a later stage):** 
   - The `sample-app/` architecture.
3. **Commit 3 (To be done in a later stage):**
   - The test suites (`tests/`).

I will unstage everything, then specifically stage ONLY the files relevant to the Control Plane Foundation.
