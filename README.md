<p align="center">
  <a href="https://github.com/sailboxhq/sailbox">
    <img src="apps/web/public/favicon.svg" width="80" alt="Sailbox" />
  </a>
</p>

<h3 align="center">Self-hosted PaaS, powered by Kubernetes</h3>

<p align="center">
  Deploy apps, databases, and cron jobs to your own servers —<br/>
  with real Kubernetes under the hood, not Docker wrappers.
</p>

<p align="center">
  <a href="https://github.com/sailboxhq/sailbox/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-AGPL--3.0-blue" alt="License" /></a>
  <a href="https://github.com/sailboxhq/sailbox/releases"><img src="https://img.shields.io/github/v/release/sailboxhq/sailbox?color=863bff" alt="Release" /></a>
  <a href="https://github.com/sailboxhq/sailbox/stargazers"><img src="https://img.shields.io/github/stars/sailboxhq/sailbox?style=flat&color=863bff" alt="Stars" /></a>
  <a href="https://github.com/sailboxhq/sailbox/pulls"><img src="https://img.shields.io/github/issues-pr/sailboxhq/sailbox?color=863bff" alt="PRs" /></a>
</p>

<p align="center">
  <a href="https://github.com/sailboxhq/sailbox">Documentation</a> ·
  <a href="https://github.com/sailboxhq/sailbox/issues">Report Bug</a> ·
  <a href="https://github.com/sailboxhq/sailbox/discussions">Discussions</a> ·
  <a href="README_CN.md">中文</a>
</p>

<p align="center"><img src=".github/screenshot.png" width="800" alt="Sailbox Dashboard" /></p>

---

## Why we built this

We've been running self-hosted services for years. We used Coolify, Dokploy, CapRover — they're great projects and the teams behind them have done incredible work making self-hosting accessible. We're grateful they exist.

But as our workloads grew, we kept hitting the same ceiling. Under the hood, they rely on Docker Compose or Docker Swarm. That works well for many use cases — but when we needed real health checks, graceful rolling updates, autoscaling, or the ability to grow from one node to three, we found ourselves fighting the abstraction instead of shipping.

We kept thinking: Kubernetes already solves all of this. It has Deployments with health probes. It has HPA for autoscaling. It has CronJobs, StatefulSets, Ingress. But traditional Kubernetes is heavy — it demands multiple nodes, complex setup, and a dedicated ops team. That's overkill for most of us.

Then we found **[K3s](https://k3s.io)** — a CNCF-certified Kubernetes distribution packed into a single binary under 100 MB. It runs on a $5 VPS, a Raspberry Pi, or a bare-metal server with just 2 GB of RAM. Same Kubernetes API, same ecosystem, a fraction of the footprint.

So we built Sailbox on top of it.

You get all the power of Kubernetes — rolling updates, autoscaling, health probes, CronJobs — without any of the setup complexity. One `curl` command and you're running. Start with a single node, add workers when you're ready. And because everything is a real Kubernetes object, `kubectl` still works — your workloads are portable from day one.

## Quick start

```bash
curl -sSL https://get.sailbox.dev | sh
```

Opens at `http://your-server-ip:3000`. That's it.

**Upgrade:**

```bash
docker compose -f /opt/sailbox/docker-compose.yml pull && docker compose -f /opt/sailbox/docker-compose.yml up -d
```

> **Requirements:** Linux (x86_64 / arm64), 2 CPU, 2 GB RAM minimum. Runs on any VPS, bare metal, or Raspberry Pi.

## Features

#### Applications
- Git push to deploy (GitHub App) or Docker image
- In-cluster builds via **Kaniko** — no Docker socket needed
- Rolling deploys, one-click rollback, cancel in-flight builds
- Custom domains with automatic TLS
- Environment variables, secrets, persistent volumes
- Health checks (liveness & readiness probes)
- Horizontal autoscaling (Kubernetes HPA)
- Web terminal into running containers

#### Databases
- PostgreSQL · MySQL · MariaDB · Redis · MongoDB
- Connection strings, external access via NodePort
- Automated S3 backups with schedule and retention
- Version management and health probes

#### Cron Jobs
- Native Kubernetes CronJobs
- Manual trigger, run history, real-time logs

#### Cluster
- Node overview with topology visualization
- Helm releases and DaemonSets
- Traefik ingress configuration editor
- Alert rules — CPU, memory, disk, node, pod events
- Auto-cleanup of evicted and failed pods

#### Team & Security
- Roles: Owner · Admin · Member
- Project-level permissions (admin / viewer)
- Two-factor authentication (TOTP)
- Team invitations via email

#### Notifications
- Email (SMTP) · Slack · Discord · Telegram
- Auto-fire on alert with per-channel toggle

#### Developer Experience
- Real-time log streaming
- `Cmd+K` global search
- Dark / light theme
- REST API

## Comparison

|  | Sailbox | Coolify | Dokploy |
|---|:---:|:---:|:---:|
| **Orchestrator** | Kubernetes (K3s) | Docker Compose | Docker Swarm |
| **In-cluster builds** (no Docker socket) | Kaniko | — | — |
| **Rolling updates** | Native K8s | Custom | Custom |
| **Autoscaling** (HPA) | Yes | — | — |
| **Health probes** (liveness / readiness) | Yes | — | — |
| **Helm releases** management | Yes | — | — |
| **Node topology** view | Yes | — | — |
| **CronJobs** | K8s native | Custom | Custom |
| **kubectl / Helm** compatible | Yes | — | — |
| **Two-factor auth** (TOTP) | Yes | — | — |
| **RBAC** with project-level perms | Yes | Limited | Limited |
| **Alert rules** (CPU/Mem/Node/Pod) | Yes | — | Basic |
| **Database S3 backup** | Yes | Yes | Yes |
| **Docker Compose** support | — | Yes | Yes |
| **One-click templates** | — | Yes | Yes |

> Sailbox doesn't wrap Kubernetes — it **is** Kubernetes.<br/>
> Your workloads run the same way they would on any K8s cluster, and everything you learn here applies everywhere else.

## Contributing

Contributions are welcome. Please see [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request.

## License

Sailbox is open-source under [AGPL-3.0](LICENSE) with [attribution terms](NOTICE). The "Powered by Sailbox" notice must remain visible in derivative works. For a commercial license, [contact us](mailto:hello@sailbox.dev).

---

<p align="center">
  <a href="mailto:hello@sailbox.dev">Contact</a> ·
  <a href="https://github.com/sponsors/sailboxhq">Sponsor</a> ·
  <a href="https://github.com/sailboxhq/sailbox">Documentation</a> ·
  <a href="https://github.com/sailboxhq/sailbox/discussions">Community</a>
</p>
