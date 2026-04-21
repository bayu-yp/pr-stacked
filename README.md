# StackPR

**Manage stacked pull requests on GitHub — automatic cascade sync, conflict detection, and base retargeting.**

![Go](https://img.shields.io/badge/go-1.22+-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/badge/license-MIT-green)

---

## Background — The Problem

Modern dev teams often have 3–5 PRs in flight simultaneously, each building on the last. The pattern — known as **stacked pull requests** — chains branches together:

```
dev (main branch)
 └─ feature/auth-model       → PR #1
     └─ feature/auth-api     → PR #2
         └─ feature/auth-ui  → PR #3
```

This unblocks fast-moving developers, but introduces serious maintenance friction:

- **Cascading manual updates** — when `dev` gets a hotfix or PR #1 receives review feedback, every downstream branch must be manually rebased/merged. In a 3-deep stack, that's three manual operations, each prone to error.
- **Conflict surface** — each merge step may produce conflicts that cascade further downstream.
- **No native GitHub tooling** — GitHub has no concept of stacked PRs. There is no cascade UI, no automated sync, no notification when a parent-child sync is needed.
- **Cognitive overhead** — developers must track which branch depends on which, what the current sync state is, and which PRs are blocked.

---

## Solution

**StackPR** is a Go CLI tool that:

- Tracks PR relationships as ordered, named stacks
- Monitors GitHub via webhooks for push and merge events
- Automatically cascades merge updates down the stack
- Detects conflicts and posts a resolution comment directly on the blocked PR
- Retargets a child PR's base when its parent is merged
- Provides a clear view of each stack's health: `synced`, `conflict`, `pending`, `merged`
- Supports manual sync via CLI
- Keeps a full audit log of every sync operation

---

## How It Works

```
┌─────────────────────────────────────────────────────────────┐
│                       StackPR Binary                        │
│                                                             │
│  ┌─────────────┐    ┌──────────────┐                        │
│  │ CLI (Cobra) │    │ Webhook HTTP │                        │
│  └──────┬──────┘    └──────┬───────┘                        │
│         └──────────────────┘                                │
│                      |                                      │
│          ┌───────────▼──────────────────┐                   │
│          │          Core Engine         │                   │
│          │  - CascadeSync               │                   │
│          │  - RetargetBase              │                   │
│          │  - Conflict detector         │                   │
│          └──────────┬───────────────────┘                   │
│                     │                                       │
│             ┌───────┴────────┐                              │
│             ▼                ▼                              │
│         ┌──────────┐  ┌───────────┐                         │
│         │PostgreSQL│  │ GitHub API│                         │
│         └──────────┘  └───────────┘                         │
└─────────────────────────────────────────────────────────────┘
```

**Webhook event handling:**

| Event | Action | Behaviour |
|---|---|---|
| `pull_request` | `synchronize` | Commits pushed to a tracked PR → cascade sync to all children |
| `pull_request` | `closed` (merged) | PR merged → retarget child's base, cascade sync downstream |
| `pull_request` | `closed` (not merged) | PR closed without merge → mark stack as broken, notify |

**Cascade sync** walks the stack from the updated PR downward, calling the GitHub update-branch API on each child. If a child has a conflict (`mergeable_state == "dirty"`), it posts a resolution comment on the PR and halts — it does not silently skip, which would corrupt the downstream chain.

**Base retargeting** — when PR #1 (targeting `dev`) is merged, StackPR immediately retargets PR #2's base from `feature/auth-model` to `dev`, making it the new root, then cascades sync downward.

---

## Features

- Named PR stacks (`stackpr stack create auth-feature`)
- Automatic cascade sync on push/merge via GitHub webhooks
- Conflict detection with actionable PR comments
- Base retargeting when a parent PR is merged
- Stack health status per entry: `synced` / `conflict` / `pending` / `merged`
- Manual sync via CLI for any stack
- Audit log (`sync_events` table) of every sync operation
- Web UI at `http://localhost:3000` — visual stack graph, per-stack sync button, conflict notes

---

## Current State & Roadmap

| Phase | Scope | Status |
|---|---|---|
| **1** | Go CLI + PostgreSQL: stack management, manual sync, status, webhook server | Done |
| **2** | Astro + React web UI: visual stack graph, live status, manual controls | **Current** |
| **3** | Cloud hosting with managed DB, TLS, secrets management, CI/CD (GCP or AWS) | Planned |

---

## Prerequisites

- **Go 1.22+**
- **Docker + Docker Compose** (for the recommended setup)
- **GitHub Personal Access Token** with `repo` and `write:discussion` scopes
- **PostgreSQL** (provided via Docker, or bring your own)

---

## Quick Start

### Option A: Docker (recommended)

```bash
cp env.example .env
# Edit .env — fill in GITHUB_TOKEN, WEBHOOK_SECRET, and Postgres values
make docker-up
stackpr init
```

This starts three services: PostgreSQL, the StackPR server, and the web UI.

- **Web UI**: `http://localhost:3000`
- **API / Webhook server**: `http://localhost:8080`

Point your GitHub webhook at `http://your-server:8080/webhook`.

### Option B: Local binary

```bash
cp env.example .env
# Edit .env — fill in GITHUB_TOKEN, DATABASE_URL, and WEBHOOK_SECRET
make build
./stackpr init
make run
```

`make run` builds the binary and starts the webhook server on port `8080`.

---

## Configuration

`stackpr init` writes a config file to `~/.stackpr/config.yaml`:

```yaml
github:
  token: ""           # or set GITHUB_TOKEN env var
  webhook_secret: ""  # or set WEBHOOK_SECRET env var

database:
  url: "postgres://stackpr:stackpr@localhost:5432/stackpr"  # or set DATABASE_URL env var

server:
  port: 8080

defaults:
  repo_owner: "your-org"
  repo_name: "your-repo"
  conflict_notify: true   # post a comment on conflicted PRs
  auto_retarget: true     # retarget child PR's base after parent merges
```

**Environment variables** — set these in your `.env` file (copied from `env.example`):

| Variable | Config key | Description |
|---|---|---|
| `GITHUB_TOKEN` | `github.token` | GitHub Personal Access Token (`repo` + `write:discussion` scopes) |
| `WEBHOOK_SECRET` | `github.webhook_secret` | Secret used to verify webhook payloads |
| `DATABASE_URL` | `database.url` | PostgreSQL connection string |
| `POSTGRES_DB` | — | Database name for the Docker Compose postgres service |
| `POSTGRES_USER` | — | Postgres user for the Docker Compose postgres service |
| `POSTGRES_PASSWORD` | — | Postgres password for the Docker Compose postgres service |

The first three also override values in `~/.stackpr/config.yaml` when set.

---

## CLI Reference

### `stackpr init`

Initialize StackPR. Creates `~/.stackpr/config.yaml` and prompts for repo owner and name.

```bash
stackpr init
```

---

### `stackpr stack create <name>`

Create a new named stack for the configured repo.

```bash
stackpr stack create auth-feature
```

---

### `stackpr stack add <pr-number> --stack <name>`

Add a PR to a stack. The PR is appended at the end (highest position). Fetches the PR's head branch from GitHub automatically.

```bash
stackpr stack add 12 --stack auth-feature
# or short flag:
stackpr stack add 12 -s auth-feature
```

| Flag | Short | Required | Description |
|---|---|---|---|
| `--stack` | `-s` | Yes | Name of the target stack |

---

### `stackpr stack remove <pr-number> --stack <name>`

Remove a PR from a stack.

```bash
stackpr stack remove 12 --stack auth-feature
```

| Flag | Short | Required | Description |
|---|---|---|---|
| `--stack` | `-s` | Yes | Name of the stack |

---

### `stackpr stack list`

List all stacks for the configured repo.

```bash
stackpr stack list
```

Output:
```
Name           ID                                    Created
----           --                                    -------
auth-feature   3f2c1a4b-...                          2024-03-15 10:22:00
```

---

### `stackpr stack status [stack-name]`

Show the health of a specific stack, or all stacks if no name is given.

```bash
stackpr stack status
stackpr stack status auth-feature
```

Output:
```
Stack: auth-feature (myorg/myrepo)
  Position  PR    Branch                 Status    Last Synced
  --------  --    ------                 ------    -----------
  1         #10   feature/auth-model     synced    2024-03-15 10:30:00
  2         #11   feature/auth-api       synced    2024-03-15 10:30:05
  3         #12   feature/auth-ui        conflict  2024-03-15 10:30:10
```

---

### `stackpr stack sync [stack-name]`

Manually trigger a cascade sync for a specific stack, or all stacks if no name is given.

```bash
stackpr stack sync
stackpr stack sync auth-feature
```

---

### `stackpr stack merged <pr-number> --stack <name>`

Notify StackPR that a PR was merged. Retargets the child PR's base and cascades sync. This is handled automatically via webhook when `stackpr serve` is running; use this command for manual workflows.

```bash
stackpr stack merged 10 --stack auth-feature
```

| Flag | Short | Required | Description |
|---|---|---|---|
| `--stack` | `-s` | Yes | Name of the stack |

---

### `stackpr serve [--port <port>]`

Start the webhook server. Listens for GitHub pull request events and triggers cascade sync automatically.

```bash
stackpr serve
stackpr serve --port 9090
```

| Flag | Default | Description |
|---|---|---|
| `--port` | `8080` | Port to listen on |

---

## Web UI

When running via Docker Compose, the web UI is available at **`http://localhost:3000`**.

- Stacks are grouped by repository and displayed as a vertical chain
- Each PR shows its branch name, current status, and last-synced timestamp
- Color-coded status badges: `synced` (green), `conflict` (red), `pending` (yellow), `merged` (gray)
- **Sync** button per stack triggers a manual cascade sync
- Conflict note with step-by-step local resolution instructions appears when a PR is in conflict state

The UI reads from the REST API exposed by `stackpr serve` (see [REST API](#rest-api) below).

For local frontend development without Docker:

```bash
make web-install   # install npm deps
make web-dev       # start Astro dev server on port 3000
```

---

## Example Workflow

```bash
# 1. Initialize StackPR
stackpr init
#    > GitHub repo owner: myorg
#    > GitHub repo name: myrepo

# 2. Create a named stack
stackpr stack create auth-feature

# 3. Open PRs on GitHub, then add them to the stack in order
stackpr stack add 10 --stack auth-feature   # PR #10: feature/auth-model → dev
stackpr stack add 11 --stack auth-feature   # PR #11: feature/auth-api   → feature/auth-model
stackpr stack add 12 --stack auth-feature   # PR #12: feature/auth-ui    → feature/auth-api

# 4. Check the stack health
stackpr stack status auth-feature

# 5. Start the webhook server — GitHub will auto-sync from now on
stackpr serve &

# 6. Push new commits to PR #10 on GitHub
#    → StackPR receives the webhook, cascades sync to PR #11, then PR #12

# 7. Check status again
stackpr stack status auth-feature

# 8. A reviewer approves and merges PR #10 via GitHub UI
#    → StackPR retargets PR #11's base from feature/auth-model → dev
#    → Cascades sync to PR #12

# 9. Or trigger manually if not using webhooks
stackpr stack merged 10 --stack auth-feature
```

---

## Webhook Setup

1. In your GitHub repo, go to **Settings → Webhooks → Add webhook**
2. Set **Payload URL** to: `http://your-server:8080/webhook`
3. Set **Content type** to: `application/json`
4. Set **Secret** to match the value in `WEBHOOK_SECRET`
5. Under **Which events**, select **Let me select individual events** → check **Pull requests**
6. Click **Add webhook**

Webhook payloads are verified using HMAC-SHA256 (`X-Hub-Signature-256`) against `WEBHOOK_SECRET`.

---

## REST API

`stackpr serve` exposes a REST API used by the web UI and available for scripting:

| Endpoint | Method | Purpose |
|---|---|---|
| `/healthz` | GET | Liveness probe — returns `"ok"` |
| `/webhook` | POST | GitHub webhook receiver (HMAC-verified) |
| `/api/stacks` | GET | List all stacks |
| `/api/stacks/{stackID}` | GET | Get a single stack |
| `/api/stacks/{stackID}/entries` | GET | Get ordered entries for a stack |
| `/api/stacks/{stackID}/sync` | POST | Trigger a manual cascade sync |
| `/api/stacks/{stackID}/events` | GET | Get last 20 sync events |

All API responses are JSON. CORS is open (`*`) for local development.

---

## Make Commands

| Target | Description |
|---|---|
| `make build` | Compile the `stackpr` binary |
| `make run` | Build and run the webhook server locally on port 8080 |
| `make docker-up` | Start all three services (postgres, stackpr, web) via Docker Compose |
| `make docker-down` | Stop containers and remove the named volume |
| `make migrate` | Run database migrations (they also run automatically on startup) |
| `make tidy` | Tidy and verify the Go module |
| `make lint` | Run `go vet ./...` |
| `make test` | Run all unit tests |
| `make web-install` | Install npm dependencies in `web/` |
| `make web-build` | Build the Astro frontend to `web/dist/` |
| `make web-dev` | Start Astro dev server on port 3000 |
| `make help` | Print available targets |

---

## Database Schema

Three tables store all StackPR state:

**`stacks`** — A named PR chain scoped to a repo.

**`stack_entries`** — The ordered list of PRs in a stack, with per-PR status (`synced`, `conflict`, `pending`, `merged`) and a `last_synced` timestamp.

**`sync_events`** — Append-only audit log of every cascade sync: which PR triggered it, start/finish time, and outcome (`success`, `partial`, `failed`).

Migrations run automatically on startup via the embedded SQL in `migrations/001_init.sql`.

---

## Contributing

Pull requests are welcome. For significant changes, please open an issue first to discuss what you'd like to change.

1. Fork the repo
2. Create a feature branch (`git checkout -b feature/my-change`)
3. Commit your changes
4. Push to your fork and open a PR

---

## License

MIT
