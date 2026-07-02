# Pinchy

A Docker-based stack for agentic programming — a foundation to build on, a place to experiment, and the ability to run multiple projects at once.

## Components

| Service | Purpose |
|---------|---------|
| **t3code** | Web-based IDE and workspace manager |
| **opencode** | AI coding assistant server |
| **litellm** | LLM proxy for model routing |
| **dind** | Docker-in-Docker for isolated builds |

## Setup

1. Copy config files:
   ```bash
   cp config/.env.t3code-example config/.env.t3code
   cp config/.env.litellm-example config/.env.litellm
   ```
2. Edit both `.env` files with your credentials
3. Add SSH key to `config/.ssh/id_rsa` (permissions: `chmod 600`)
4. Run: `docker compose up -d`
5. Get t3code pairing key from logs:
   ```bash
   docker compose logs pinchy-t3code
   ```
   Look for: `pairingUrl: http://localhost:3773/pair#token=XXXXXXXXXX`

## Usage

Access via t3code: http://127.0.0.1:3773

## Roadmap

- [ ] Override bash tool — environment per workspace
- [ ] Chrome MCP integration
- [ ] Automatic session creation via Jira/GitHub issues
