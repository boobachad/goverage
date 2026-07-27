# Goverage

An AI agent governance gateway that enforces token budgets, ABAC policies, identity verification, kill-switches, and lifecycle management on LLM calls — with a real-time React dashboard.

## Architecture

```
agents/ (Python)  →  gateway/ (Go/Fiber)  →  OpenRouter (LLM upstream)
                          ↕
                  Redis (token leases)
                  Postgres (audit logs)
                  dashboard/ (React + Vite)
```

**Components:**
- **gateway** — Go/Fiber reverse proxy. Enforces ABAC policy, token budgets (two-phase lease via Redis), Ed25519 identity verification, kill-switch, and lifecycle state. Streams telemetry over WebSocket.
- **agents** — Python demo agents. Each registers a DID keypair with the gateway, then fires LLM calls through it.
- **dashboard** — React + Vite + Zustand UI. Live agent status, audit log, budget controls, policy editor, halt/resume controls.
- **docker** — Postgres + Redis via Docker Compose.

---

## Prerequisites

- Go 1.21+
- Python 3.10+
- Node.js 18+ / Bun
- Postgres 15+ and Redis 7+ (either native or via Docker Compose)

---

## Setup

### 1. Clone the repo

```bash
git clone --recurse-submodules <your-repo-url>
```

This pulls `area12/agent-governance-toolkit` which the gateway and agents depend on.

### 2. Configure env files

```bash
# Gateway
cp gateway/.env.example gateway/.env

# Agents
cp agents/.env.example agents/.env
```

Fill in the required values (see `.env.example` comments).

### 2. Start infrastructure

**Option A — Docker Compose:**

```bash
cd docker
docker compose up -d
```

**Option B — local services:**

Ensure Postgres and Redis are already running on your machine:

```bash
# create db and fill via init.sql
psql -U postgres -c "CREATE DATABASE goverage;"
psql -U postgres -d goverage -f docker/init.sql

# start redis
redis-server
```

Both options expose the same ports (`5432`, `6379`) and the `gateway/.env` defaults work for either.

### 3. Run the gateway

```bash
cd gateway
go run .
```

Gateway starts on the port defined in `gateway/.env` (default `3000`).

### 4. Run demo agents

```bash
cd agents
python -m venv .venv && source .venv/bin/activate
pip install openai httpx requests python-dotenv
python deploy_agents.py
```

Two agents (`agent-deleter`, `agent-reader`) run concurrently. `agent-deleter` is blocked by policy; `agent-reader` is allowed.

### 5. Run the dashboard

```bash
cd dashboard
bun install   # or: npm install
bun run dev   # or: npm run dev
```

Dashboard runs at `http://localhost:5173`. It connects to the gateway at `http://localhost:3000`.

---

## Testing the system

### 1. Run the demo scenario

```bash
cd agents
python deploy_agents.py
```

Two agents run concurrently through the gateway:
- `agent-reader` — allowed by policy, upstream returns a tool call
- `agent-deleter` — blocked by ABAC, receives HTTP 403

### 2. Use the dashboard (`http://localhost:5173`)

The dashboard has three tabs:

**Active Agents tab**
- Lists all registered agents with their DID, status, token cap, tokens used, and remaining budget
- **Fleet budget** — click Edit next to the total to set a global token cap across all agents
- **Per-agent budget** — click Edit in the Cap column of any agent row to set that agent's individual limit
- **Halt / Lift Halt** — immediately blocks or unblocks an agent's LLM access via kill-switch
- **Suspend / Quarantine** — lifecycle state transitions for an active agent
- **Activate** — resumes a suspended or quarantined agent
- **Refresh** — manually re-polls agent status (also auto-polls every 5 seconds)

**Policy Editor tab**
- Displays the current `policy.yaml` loaded in the gateway
- Edit the YAML directly in the browser and click **Save** to hot-reload the policy without restarting the gateway

**Telemetry tab**
- Left panel: combined feed of live WebSocket events and persisted audit log entries (click any row to inspect)
- Right panel: full detail of the selected log — request payload, LLM response, tool invocations
- **Verify Hash Chain** — runs cryptographic verification of the in-memory audit ledger; shows each block's hash, previous hash, agent, action, and decision. Flags tampering if the chain is broken
- **Refresh History** — re-fetches the last 1000 entries from Postgres

---

## Policy format

Edit `gateway/policy.yaml` or POST to `/api/policy` at runtime:

```yaml
rules:
  - action: "data.read"
    effect: "allow"
    conditions:
      agent_id: "agent-reader"
```

`effect` can be `allow` or `deny`. Unmatched actions default to deny.

---

## Project structure

```
check2/
├── gateway/          # Go gateway (main.go, routes/, middleware/, audit/)
├── agents/           # Python agent demo (deploy_agents.py)
├── dashboard/        # React + Vite frontend
├── docker/           # docker-compose.yml + init.sql
└── area12/           # Local copy of agent-governance-toolkit (Go + Python SDKs)
```
