<p align="center">
  <a href="https://github.com/sailboxhq/sailbox">
    <img src="apps/web/public/favicon.svg" width="80" alt="Sailbox" />
  </a>
</p>

<h3 align="center">自托管 PaaS，以 Kubernetes 为基础</h3>

<p align="center">
  将应用、数据库和定时任务部署到你自己的服务器 —<br/>
  底层是真正的 Kubernetes，而不是 Docker 的包装层。
</p>

<p align="center">
  <a href="https://github.com/sailboxhq/sailbox/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-AGPL--3.0-blue" alt="License" /></a>
  <a href="https://github.com/sailboxhq/sailbox/releases"><img src="https://img.shields.io/github/v/release/sailboxhq/sailbox?color=863bff" alt="Release" /></a>
  <a href="https://github.com/sailboxhq/sailbox/stargazers"><img src="https://img.shields.io/github/stars/sailboxhq/sailbox?style=flat&color=863bff" alt="Stars" /></a>
  <a href="https://github.com/sailboxhq/sailbox/pulls"><img src="https://img.shields.io/github/issues-pr/sailboxhq/sailbox?color=863bff" alt="PRs" /></a>
</p>

<p align="center">
  <a href="https://github.com/sailboxhq/sailbox">文档</a> ·
  <a href="https://github.com/sailboxhq/sailbox/issues">问题反馈</a> ·
  <a href="https://github.com/sailboxhq/sailbox/discussions">社区讨论</a> ·
  <a href="README.md">English</a>
</p>

<p align="center"><img src=".github/screenshot.png" width="800" alt="Sailbox Dashboard" /></p>

---

## 为什么做 Sailbox

我们跑自托管服务好几年了。Coolify、Dokploy、CapRover 都用过 — 它们是很好的项目，背后的团队做了很多了不起的工作，让自托管变得简单。我们很感谢有这些工具存在。

但随着业务增长，我们反复撞上同一个天花板。它们的底层依赖 Docker Compose 或 Docker Swarm，对很多场景来说已经够好了 — 但当我们需要真正的健康检查、平滑的滚动更新、自动扩缩容，或者从一台机器扩到三台的时候，我们发现自己是在跟抽象层较劲，而不是在写业务。

我们一直在想：Kubernetes 早就把这些问题解决了。它有带健康探针的 Deployment，有 HPA 自动扩缩容，有 CronJob，有 StatefulSet，有 Ingress。但传统 Kubernetes 太重了 — 需要多个节点、复杂的配置、专职的运维团队。对大多数人来说，这杀鸡用牛刀了。

后来我们发现了 **[K3s](https://k3s.io)** — 一个 CNCF 认证的 Kubernetes 发行版，打包成不到 100 MB 的单个二进制文件。它能跑在一台 5 美元的 VPS 上、一个树莓派上、或者一台只有 2 GB 内存的裸金属服务器上。API 完全兼容，生态完全通用，资源占用却只是传统 K8s 的零头。

所以我们在它上面做了 Sailbox。

你能用上 Kubernetes 的全部能力 — 滚动更新、自动扩缩容、健康探针、CronJob — 但完全不需要折腾搭建。一条 `curl` 命令就能跑起来。先用一台机器，需要时再加 worker 节点。而且因为所有资源都是真正的 Kubernetes 对象，`kubectl` 随时可用 — 你的工作负载从第一天起就是可迁移的。

## 快速开始

```bash
curl -sSL https://get.sailbox.dev | sh
```

安装完成后打开 `http://<服务器IP>:3000`，开始使用。

**升级：**

```bash
docker compose -f /opt/sailbox/docker-compose.yml pull && docker compose -f /opt/sailbox/docker-compose.yml up -d
```

> **环境要求：** Linux（x86_64 / arm64），最低 2 核 CPU、2 GB 内存。VPS、裸金属、树莓派均可。

## 功能

#### 应用部署
- Git 推送部署（GitHub App 集成）或 Docker 镜像
- 集群内构建，使用 **Kaniko** — 不需要挂载 Docker Socket
- 滚动部署、一键回滚、取消进行中的构建
- 自定义域名，自动签发 TLS 证书
- 环境变量、密钥、持久化存储
- 健康检查（存活探针 & 就绪探针）
- 水平自动扩缩容（Kubernetes HPA）
- Web 终端直连运行中的容器

#### 数据库
- PostgreSQL · MySQL · MariaDB · Redis · MongoDB
- 连接信息、NodePort 外部访问
- 自动 S3 备份，支持自定义计划和保留策略
- 版本管理与健康探测

#### 定时任务
- 原生 Kubernetes CronJob
- 手动触发、运行历史、实时日志

#### 集群管理
- 节点概览与拓扑可视化
- Helm releases 和 DaemonSet
- Traefik 入口配置编辑器
- 告警规则 — CPU、内存、磁盘、节点状态、Pod 事件
- 自动清理驱逐和失败的 Pod

#### 团队与安全
- 角色：拥有者 · 管理员 · 成员
- 项目级权限（管理 / 只读）
- 双因素认证（TOTP）
- 邮件邀请加入团队

#### 通知
- 邮件（SMTP）· Slack · Discord · Telegram
- 告警自动触发，每个通道独立开关

#### 开发体验
- 实时日志流
- `Cmd+K` 全局搜索
- 深色 / 浅色主题
- REST API

## 对比

|  | Sailbox | Coolify | Dokploy |
|---|:---:|:---:|:---:|
| **编排引擎** | Kubernetes (K3s) | Docker Compose | Docker Swarm |
| **集群内构建**（无需 Docker Socket） | Kaniko | — | — |
| **滚动更新** | K8s 原生 | 自定义 | 自定义 |
| **自动扩缩容**（HPA） | Yes | — | — |
| **健康探针**（存活 / 就绪） | Yes | — | — |
| **Helm releases** 管理 | Yes | — | — |
| **节点拓扑**视图 | Yes | — | — |
| **定时任务** | K8s CronJob | 自定义 | 自定义 |
| **kubectl / Helm** 兼容 | Yes | — | — |
| **双因素认证**（TOTP） | Yes | — | — |
| **RBAC** + 项目级权限 | Yes | 有限 | 有限 |
| **告警规则**（CPU/内存/节点/Pod） | Yes | — | 基础 |
| **数据库 S3 备份** | Yes | Yes | Yes |
| **Docker Compose** 支持 | — | Yes | Yes |
| **一键模板** | — | Yes | Yes |

> Sailbox 不是在 Kubernetes 外面包了一层 — 它**就是** Kubernetes。<br/>
> 你的工作负载和在任何 K8s 集群上运行的方式完全一样，在这里学到的一切，在别处同样适用。

## 参与贡献

欢迎贡献。请在提交 PR 前阅读 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 许可证

Sailbox 基于 [AGPL-3.0](LICENSE) 开源，附带[署名条款](NOTICE)。所有衍生作品的 UI 中须保留 "Powered by Sailbox" 标识。如需移除，请[联系我们](mailto:hello@sailbox.dev)获取商业许可。

---

<p align="center">
  <a href="mailto:hello@sailbox.dev">联系我们</a> ·
  <a href="https://github.com/sponsors/sailboxhq">赞助</a> ·
  <a href="https://github.com/sailboxhq/sailbox">文档</a> ·
  <a href="https://github.com/sailboxhq/sailbox/discussions">社区</a>
</p>
