# lms-workflow

> Overdue-detection + penalty-application saga

Part of the **LMS Library** distributed system — team `lms-library`, Grupo 2.
Governance and documentation live in [`library-docs`](https://github.com/code-corhuila/library-docs).

Go, hexagonal, orchestrated saga (no message broker exists in this system — `AT-002`,
`05-architecture/overview.md`). **Scope correction from this repo's original README:** this
service does **not** orchestrate the full loan lifecycle (loan → renewal → return → penalty) —
`ADR-008-circulation-saga-scope.md` narrows that down to exactly two steps, triggered once a loan
is found overdue: mark the loan late in `lms-circulation-api`, then suspend the student in
`lms-membership-api`. Registration and return stay direct, synchronous calls between those
services, unchanged.

## Structure

```
cmd/worker/              → entry point (main.go) — no HTTP server, an internal ticker instead
migrations/              → this domain's own schema migrations (see "Migration scope" below)
internal/
├── domain/saga/          → Run aggregate (the saga's state machine), ports
├── application/usecase/  → ApplyPenalty
└── infrastructure/
    ├── postgres/           → RunRepository (secondary adapter)
    ├── httpclient/          → CirculationClient, MembershipClient (plain net/http, internal JWT)
    ├── scheduler/            → the ticker that drives ApplyPenalty
    └── logger/                → structured (zap) logger
```

## Saga state

One row per `loan_id` in `workflow.saga_run` (`PENDING → MARKED_LATE → COMPLETED` or `FAILED`),
per `ADR-008`. `loan_id` is the idempotency key: a run already `COMPLETED` is skipped entirely; a
run stuck at `MARKED_LATE` resumes straight at the suspend step. **Forward-recovery only** — a
failed suspend never rolls back an already-applied "marked late."

## Known integration gap

This saga calls `GET /api/v1/loans/overdue` (already real, HU-08) and `POST
/api/v1/loans/{id}/mark-late` (**not yet built** on `lms-circulation-api`) to complete step one.
Building that endpoint is out of this repo's own scope — flagged here so it isn't silently
assumed already wired up end-to-end.

## Migration scope

**From scratch** — nothing in `lms-library` covers this today (new scope, not a relocation).

This saga's `workflow.saga_run` table needs to be durable per `ADR-008`, which directly
supersedes `ADR-006`'s blanket "workflow is stateless" line for this one subsystem specifically
(`ADR-006`'s own text is not yet corrected to say so — a known, flagged documentation gap,
out of scope for this change). The course's fixed 18-repo topology never assigned `lms-workflow`
a sibling `-db` repo the way the four domain contexts have one, so this repo's own `migrations/`
folder owns a `workflow` schema on the same shared PostgreSQL instance every other domain's schema
lives on (Anexo J) — not a new database server, just a new schema, applied via this repo's own
`deploy/compose.yml` Liquibase executor (same pattern as every `-db` repo, simplified to a flat
changelog since the scope is one table).

## Development

```bash
go mod download
go run ./cmd/worker/...
```

## Tests

```bash
go test ./...
```

---

## Branching

Three permanent branches. **None of them accepts a direct commit** — you enter through a child
branch and leave through a Pull Request.

```
develop  <--PR--  feat/... fix/... chore/...
qa       <--PR--  qa/...
main     <--PR--  release/...  hotfix/...
```

Promotion happens **by re-application** (`git cherry-pick -x`), never by merging one permanent
branch into another: `merge develop -> qa` and `merge qa -> main` do not exist in this model.

`main` requires **1 approval from `ariel5253`**. On `develop` and `qa` the team sets its own review
rule.

Full policy: `00-governance/branching-policy.md` in `library-docs`.

## Correlations

* Saga scope and state design → `library-docs/05-architecture/decisions/records/ADR-008-circulation-saga-scope.md`
* Repo decomposition → `library-docs/05-architecture/decisions/records/ADR-006-repo-per-context-decomposition.md`
* No message broker anywhere in this system → `library-docs/09-microservices/communication-patterns.md`
