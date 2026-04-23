# StackPR — Technical Design Document

---

## 1. Background

Modern software teams practice trunk-based development, where work is merged into a main branch (e.g., `dev` or `main`) frequently. However, in fast-moving teams, the pace of code authoring often outstrips the pace of code review. A developer may have 3–5 pieces of work in flight simultaneously, each building on the previous one.

To unblock themselves, developers adopt **stacked pull requests** — a pattern where each PR targets the branch of the previous PR rather than `main`. This creates a chain:

```
dev (main branch)
 └─ feature/auth-model       → PR #1
     └─ feature/auth-api     → PR #2
         └─ feature/auth-ui  → PR #3
```

This is a well-understood workflow used at companies like Meta, Google, and modern dev-tool teams (Graphite, Aviator, Linear).

---

## 2. Problem

The stacked PR pattern, while powerful, introduces significant **maintenance friction**:

### 2.1 Cascading Updates
When `dev` pushes a hotfix or `PR #1` receives new commits from review feedback, **every downstream PR branch must be manually updated**. In a 3-deep stack, this means:
1. Merge `dev` into `PR #1` branch
2. Merge `PR #1` branch into `PR #2` branch
3. Merge `PR #2` branch into `PR #3` branch

Each step is manual, error-prone, and time-consuming.

### 2.2 Conflict Surface
When a child PR (`PR #2`) receives new commits from its own review, merging parent changes into it may produce conflicts. These conflicts must be resolved before the chain can continue, and each resolution may cascade further conflicts downstream.

### 2.3 No Native Tooling
GitHub has no native concept of "stacked PRs." There is no UI to visualize the chain, no automated cascade update, and no notification when a parent-child sync is needed.

### 2.4 Cognitive Load
Developers must mentally track: which branch depends on which, what the current sync state is, and which PRs are blocked by conflicts. This is pure overhead on top of the actual development work.

---

## 3. Approach & Solution

### 3.1 Solution Overview

**StackPR** is a Go CLI tool (with a planned Astro + React web UI phase) that:
1. Tracks relationships between PRs as an ordered stack
2. Monitors GitHub for PR events via webhooks (merge, push, update)
3. Automatically cascades merge updates down the stack
4. Detects and surfaces conflicts without silently breaking the chain
5. Provides a clear view of the entire stack's health at any time

### 3.2 Tech Stack

| Layer | Technology | Rationale |
|---|---|---|
| CLI | Go + [Cobra](https://github.com/spf13/cobra) | Single binary, fast, excellent CLI ergonomics |
| GitHub API | [google/go-github](https://github.com/google/go-github) | Type-safe, comprehensive GitHub REST API coverage |
| Git Operations | [go-git](https://github.com/go-git/go-git) + system `git` | Pure Go for reads; system git for merge/push |
| Storage | PostgreSQL via [jackc/pgx](https://github.com/jackc/pgx) | Shared concurrent access across team members; production-grade; native support on GCP Cloud SQL and AWS RDS |
| Webhook Server | Go `net/http` (built-in) | No external dependency needed |
| Auth | GitHub PAT or OAuth via `golang.org/x/oauth2` | Standard GitHub auth pattern |
| Config | YAML via [viper](https://github.com/spf13/viper) | Per-repo or global config file |
| Web UI (Phase 2) | [Astro](https://astro.build) + React | Astro for static shell + islands; React for interactive stack graph |
| UI Data | REST API served from same Go binary | Go serves JSON endpoints consumed by Astro frontend |

### 3.3 Architecture

```
┌────────────────────────────────────────────────────────────┐
│                      StackPR Binary                        │
│                                                            │
│  ┌─────────────┐    ┌──────────────┐   ┌───────────────┐  │
│  │ CLI (Cobra) │    │ Webhook HTTP │   │ REST API (UI) │  │
│  └──────┬──────┘    └──────┬───────┘   └──────┬────────┘  │
│         │                  │                   │           │
│         └──────────────────┼───────────────────┘           │
│                            ▼                               │
│          ┌──────────────────────────────────┐              │
│          │          Core Engine             │              │
│          │  - Stack resolver                │              │
│          │  - Cascade sync orchestrator     │              │
│          │  - Conflict detector             │              │
│          │  - Base retargeter (on merge)    │              │
│          └──────────┬───────────────────────┘              │
│                     │                                      │
│             ┌───────┴────────┐                             │
│             ▼                ▼                             │
│         ┌──────────┐  ┌───────────┐                        │
│         │PostgreSQL│  │ GitHub API│                        │
│         │   (DB)   │  │  Client   │                        │
│         └──────────┘  └───────────┘                        │
└────────────────────────────────────────────────────────────┘

Phase 2: Astro UI (read-only dashboard)
Phase 2.5: Astro UI (full management — CLI parity)
┌──────────────────────────────────────────┐
│  Astro + React (embedded)                │
│  - Stack dependency graph                │
│  - PR status per node                    │
│  - Manual sync trigger button            │
│  - Conflict indicators                   │
│  - Create / delete stacks         (2.5)  │
│  - Add / remove PR entries        (2.5)  │
│  - Mark PR merged + cascade sync  (2.5)  │
└──────────────────────────────────────────┘
```

### 3.4 CLI Commands

```bash
stackpr init                          # Initialize StackPR in current repo
stackpr stack create <name>           # Create a new named stack
stackpr stack add <pr-number>         # Add a PR to current stack (in order)
stackpr stack remove <pr-number>      # Remove a PR from the stack
stackpr stack list                    # List all stacks in current repo
stackpr stack status [stack-name]     # Show health of a stack (synced, conflict, pending)
stackpr stack sync [stack-name]       # Manually trigger a cascade sync
stackpr serve                         # Start webhook listener for auto-sync
```

### 3.5 Database Schema (PostgreSQL)

**`stacks`** — Represents a named PR chain
```sql
CREATE TABLE stacks (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name        TEXT NOT NULL,
  repo_owner  TEXT NOT NULL,
  repo_name   TEXT NOT NULL,
  created_at  TIMESTAMPTZ DEFAULT NOW()
);
```

**`stack_entries`** — Ordered list of PRs within a stack
```sql
CREATE TABLE stack_entries (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  stack_id    UUID NOT NULL REFERENCES stacks(id) ON DELETE CASCADE,
  pr_number   INTEGER NOT NULL,
  branch_name TEXT NOT NULL,
  position    INTEGER NOT NULL,        -- 0 = root (targets dev/main), 1 = first child, etc.
  status      TEXT NOT NULL DEFAULT 'synced'
                CHECK (status IN ('synced', 'conflict', 'pending', 'merged')),
  last_synced TIMESTAMPTZ,
  UNIQUE(stack_id, position)
);
```

**`sync_events`** — Audit log of all sync operations
```sql
CREATE TABLE sync_events (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  stack_id      UUID NOT NULL REFERENCES stacks(id),
  triggered_by  INTEGER NOT NULL,     -- PR number that triggered the cascade
  started_at    TIMESTAMPTZ NOT NULL,
  finished_at   TIMESTAMPTZ,
  status        TEXT NOT NULL CHECK (status IN ('success', 'partial', 'failed')),
  error_message TEXT
);
```

Connection is provided via the `DATABASE_URL` environment variable:
```
DATABASE_URL=postgres://user:password@host:5432/stackpr
```

### 3.6 Core Algorithm: Cascade Merge Sync

When a PR at position `N` is updated (new commits pushed or parent branch merged into it):

```
1. Load the stack from DB for the affected PR
2. Find its position N in stack_entries
3. For each child at position N+1, N+2, ... (in order):
   a. Call GitHub API:
      PUT /repos/{owner}/{repo}/pulls/{pr}/update-branch
      (merges parent branch HEAD into child branch)
   b. Poll PR.mergeable_state until not "unknown" (GitHub computes async)
   c. If mergeable_state == "dirty" (conflict):
      - Mark stack_entry.status = 'conflict'
      - Post comment on the conflicted PR notifying the author
      - STOP cascade — do not process further children
   d. If mergeable_state == "clean":
      - Mark stack_entry.status = 'synced'
      - Continue to next child
4. Write a sync_event record with final outcome
```

**Cascade halts on conflict (does not skip)** — This is intentional. Silently skipping a conflicted node would produce an incorrect downstream chain.

**Conflict comment posted automatically to the PR:**
```
[StackPR] Sync conflict detected

The branch `feature/auth-api` could not be automatically updated from
`feature/auth-model` (PR #1) due to a merge conflict.

Please resolve locally:
  git checkout feature/auth-api
  git merge feature/auth-model
  # resolve conflicts
  git push

Downstream PRs (#14) are paused until this is resolved.
```

### 3.7 Base Retargeting on Merge

When `PR #1` (targeting `dev`) is merged:
1. `PATCH /repos/{owner}/{repo}/pulls/{PR2_number}` with `{ "base": "dev" }`
2. PR #2 is now retargeted from `feature/auth-model` → `dev`
3. PR #2 becomes the new root of the stack
4. Cascade sync triggers from PR #2 downward

### 3.8 GitHub Webhook Events

The `stackpr serve` command listens for:

| Event | Action | Handler |
|---|---|---|
| `pull_request` | `synchronize` | Commits pushed to a tracked PR → cascade sync to children |
| `pull_request` | `closed` (merged=true) | PR merged → retarget child's base, cascade sync |
| `pull_request` | `closed` (merged=false) | PR closed without merge → mark stack as broken, notify |

Webhooks are verified using HMAC-SHA256 signature (`X-Hub-Signature-256` header) against the configured secret.

---

## 4. Deployment

### Phase 1 — Local Docker (CLI + Backend)

All services run via a single `docker compose up`. No web UI in this phase.

**Services:**
- `stackpr` — Go binary (CLI + webhook server)
- `postgres` — PostgreSQL 16

**`docker-compose.yml` (Phase 1):**
```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: stackpr
      POSTGRES_USER: stackpr
      POSTGRES_PASSWORD: stackpr
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

  stackpr:
    build: .
    environment:
      DATABASE_URL: postgres://stackpr:stackpr@postgres:5432/stackpr
      GITHUB_TOKEN: ${GITHUB_TOKEN}
      WEBHOOK_SECRET: ${WEBHOOK_SECRET}
    ports:
      - "8080:8080"
    depends_on:
      - postgres
    command: ["serve", "--port", "8080"]

volumes:
  pgdata:
```

**Usage:**
```bash
cp .env.example .env        # Set GITHUB_TOKEN and WEBHOOK_SECRET
docker compose up -d

# Run CLI commands via exec
docker compose exec stackpr stackpr stack create "auth-feature"
docker compose exec stackpr stackpr stack add 12
docker compose exec stackpr stackpr stack status auth-feature
```

---

### Phase 2 — Local Docker with Web UI

Extends Phase 1 by adding the Astro + React frontend as a third service.

**Additional service:**
- `web` — Astro UI served at `http://localhost:3000`

The Go binary serves a REST API at `/api/*` (port 8080). The Astro frontend consumes this API.

**`docker-compose.yml` (Phase 2, additions):**
```yaml
services:
  # ... postgres and stackpr from Phase 1 ...

  web:
    build:
      context: ./web
      dockerfile: Dockerfile
    environment:
      PUBLIC_API_URL: http://localhost:8080
    ports:
      - "3000:3000"
    depends_on:
      - stackpr
```

**Project structure for Phase 2:**
```
astro-pr-stacked/
├── cmd/stackpr/         # Go CLI entrypoint
├── internal/
│   ├── engine/          # Core cascade logic
│   ├── github/          # API client wrapper
│   ├── db/              # PostgreSQL queries
│   └── server/          # HTTP handlers (webhook + REST API)
├── web/                 # Astro project
│   ├── src/
│   │   ├── components/  # React stack graph component
│   │   └── pages/       # index.astro
│   ├── Dockerfile
│   └── dist/
├── docker-compose.yml
├── Dockerfile
└── Makefile
```

**UI features available in Phase 2:**
- Stack dependency graph (visual node-link diagram)
- Per-PR status indicators (synced / conflict / pending / merged)
- Manual sync trigger button
- Conflict details and resolution instructions

---

### Phase 2.5 — Full UI Management (CLI Parity)

Extends Phase 2 by making the web UI a full management surface. All write operations previously requiring `docker compose exec stackpr stackpr ...` are now available in the browser. The CLI remains functional and is the recommended path for scripted or automated workflows.

**Goal:** Zero CLI required for interactive day-to-day stack management.

#### New API Endpoints

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/stacks` | Create a new named stack |
| `DELETE` | `/api/stacks/{stackID}` | Delete a stack and all its entries (cascaded) |
| `POST` | `/api/stacks/{stackID}/entries` | Add a PR to a stack (resolves branch name from GitHub) |
| `DELETE` | `/api/stacks/{stackID}/entries/{prNumber}` | Remove a PR entry from a stack |
| `POST` | `/api/stacks/{stackID}/entries/{prNumber}/merged` | Mark PR as merged, retarget child's base, cascade sync |

#### Request / Response Shapes

**`POST /api/stacks`**
```json
// Request
{ "name": "auth-feature", "repo_owner": "myorg", "repo_name": "myrepo" }

// Response 201
{ "id": "uuid", "name": "auth-feature", "repo_owner": "myorg", "repo_name": "myrepo", "created_at": "..." }

// Response 409 — stack name already exists for this repo
{ "error": "stack already exists" }
```

**`POST /api/stacks/{stackID}/entries`**
```json
// Request
{ "pr_number": 42 }

// Response 201 — branch resolved from GitHub API
{ "id": "uuid", "stack_id": "...", "pr_number": 42, "branch_name": "feature/auth-api", "position": 1, "status": "synced" }

// Response 422 — PR not found or branch unresolvable
{ "error": "failed to fetch PR #42 from GitHub: ..." }
```

**`DELETE /api/stacks/{stackID}/entries/{prNumber}`**
```json
// Response 200
{ "ok": true }

// Response 404 — PR not in this stack
{ "error": "stack entry not found" }
```

**`POST /api/stacks/{stackID}/entries/{prNumber}/merged`**
```json
// Response 200
{ "ok": true, "message": "retarget and sync triggered" }
```

**`DELETE /api/stacks/{stackID}`**
```json
// Response 200
{ "ok": true }
```

#### Architectural Changes

**Server holds `*github.Client` directly.** The `Add Entry` and `Mark Merged` handlers need to call `GetPR()` to resolve the head/base branch of a PR — the same call the CLI makes in `stackAddCmd` and `stackMergedCmd`. Rather than routing this through `Engine`, the `Server` struct was extended to hold a `*github.Client` field alongside the existing `*engine.Engine`:

```go
type Server struct {
    eng           *engine.Engine
    gh            *github.Client   // added in Phase 2.5
    webhookSecret string
    mux           *http.ServeMux
}
```

The `New()` constructor signature updated accordingly: `New(eng *engine.Engine, gh *github.Client, webhookSecret string)`.

**CORS updated** to allow `DELETE` in addition to `GET`, `POST`, `OPTIONS`.

**`ErrEntryNotFound` sentinel** added to `db/queries.go` so `RemoveStackEntry` can signal "not found" vs. a real DB error, enabling the handler to return HTTP 404 vs. 500 correctly.

#### New UI Components

| Component | Purpose |
|---|---|
| `CreateStackModal.jsx` | Modal overlay with Stack Name, Repo Owner, Repo Name fields. Calls `POST /api/stacks`. |
| `AddPRForm.jsx` | Inline form embedded inside a stack card. Accepts a PR number, calls `POST .../entries`. |
| `PRNode.jsx` (modified) | Adds **Remove** and **Mark Merged** action buttons per PR row. |
| `StackGraph.jsx` (modified) | Adds **Add PR** toggle and **Delete Stack** button in the stack card header. |
| `StackList.jsx` (modified) | Adds **New Stack** primary button in toolbar; replaces CLI hint in empty state with an actionable button. |

Destructive actions (Remove, Mark Merged, Delete Stack) use `window.confirm()` for Phase 2.5. A styled confirmation dialog system is planned for a future phase.

#### UI Features Added in Phase 2.5

- Create a new stack from the browser (replaces `stackpr init` + `stackpr stack create`)
- Add a PR to a stack by number (replaces `stackpr stack add`)
- Remove a PR entry from a stack (replaces `stackpr stack remove`)
- Mark a PR as merged with automatic child retarget + cascade sync (replaces `stackpr stack merged`)
- Delete an entire stack (new — no CLI equivalent beyond scripting)

---

### Phase 3 — Cloud Hosting (GCP or AWS)

The same Docker image from Phase 2 is deployed to a managed cloud platform. Both GCP and AWS paths are supported.

#### Option A: GCP

| Component | GCP Service |
|---|---|
| Application | Cloud Run (containerized Go binary) |
| Database | Cloud SQL for PostgreSQL |
| Container registry | Artifact Registry |
| CI/CD | Cloud Build |
| Secrets | Secret Manager |
| TLS | Google-managed SSL certificate |

```bash
# Build and push image
gcloud builds submit --tag gcr.io/PROJECT_ID/stackpr

# Deploy to Cloud Run
gcloud run deploy stackpr \
  --image gcr.io/PROJECT_ID/stackpr \
  --set-secrets DATABASE_URL=stackpr-db-url:latest \
  --set-secrets GITHUB_TOKEN=github-token:latest \
  --set-secrets WEBHOOK_SECRET=webhook-secret:latest \
  --allow-unauthenticated \
  --port 8080
```

#### Option B: AWS

| Component | AWS Service |
|---|---|
| Application | ECS Fargate (or App Runner) |
| Database | RDS for PostgreSQL |
| Container registry | ECR |
| CI/CD | CodePipeline or GitHub Actions |
| Secrets | AWS Secrets Manager |
| TLS | ACM + Application Load Balancer |

```bash
# Push image to ECR
aws ecr get-login-password | docker login --username AWS --password-stdin $ECR_URI
docker build -t stackpr .
docker tag stackpr:latest $ECR_URI/stackpr:latest
docker push $ECR_URI/stackpr:latest

# Deploy via ECS task definition (DATABASE_URL, GITHUB_TOKEN, WEBHOOK_SECRET injected from Secrets Manager)
```

**GitHub Webhook Configuration (Phase 3):**
- Payload URL: `https://your-service-url/webhook`
- Content type: `application/json`
- Secret: must match `WEBHOOK_SECRET` in cloud secrets
- Events: **Pull requests** only

---

### 4.1 Configuration (`~/.stackpr/config.yaml`)

```yaml
github:
  token: ghp_xxxxxxxxxxxx
  webhook_secret: s3cr3t

database:
  url: postgres://user:password@host:5432/stackpr

server:
  port: 8080

defaults:
  conflict_notify: true      # Post comment on conflicted PR
  auto_retarget: true        # Retarget child PR's base after parent merges
```

---

## 5. Conclusion

StackPR addresses a real and recurring pain point for developers who move faster than their review cycle. By codifying the PR chain as a first-class data structure and automating cascade merge propagation, it eliminates the most tedious and error-prone part of the stacked PR workflow.

**Key design decisions:**

| Decision | Choice | Reason |
|---|---|---|
| Sync strategy | Merge (not rebase) | GitHub API supports merge natively; rebase requires force-push which disrupts reviewers |
| Storage | PostgreSQL | Shared team tool requires concurrent access, network-accessible DB, and cloud-managed hosting compatibility |
| Runtime | Go single binary | Zero dependency distribution; fast startup; strong concurrency for webhook handling |
| Conflict behavior | Stop cascade, notify | Silently skipping a conflict would corrupt the downstream chain |
| UI framework | Astro + React | Astro for fast static shell; React islands only where interactivity is needed |

**Roadmap:**

| Phase | Scope | Environment |
|---|---|---|
| 1 | Go CLI + PostgreSQL: stack management, manual sync, status, webhook server | Local Docker |
| 2 | Astro + React web UI: visual stack graph, live status, manual controls | Local Docker |
| 2.5 | Full UI management: create stacks, add/remove PRs, mark merged, delete stacks — CLI parity in the browser | Local Docker |
| 3 | Production deployment with managed DB, TLS, secrets management, CI/CD | GCP or AWS |
