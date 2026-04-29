# Game Admin

`game-admin` 是一套单仓多进程的代理运营后台，当前已经落地为：

- `frontend` 管理台
- `gateway-service`
- 15 个 HTTP 服务
- 2 个 worker
- `MySQL + Redis + RabbitMQ` 基础设施

这不是“单体 demo”，也还不是“完全自治微服务终态”。

当前真实状态：
- 已具备网关转发、服务拆分、Prometheus `/metrics`、`/healthz` `/readyz`、DB outbox + MQ/DB fallback worker、服务级启动/迁移/验活/发布脚本
- 已完成主要同步调用治理，服务间同步调用统一经 `internal/contract/*` + `internal/syncclient`
- 仍未完成独立数据库自治，服务依然共享同一套 DB/schema/model

相关文档：
- [微服务架构说明](docs/当前系统架构.md)
- [部署与验活 Runbook](docs/微服务部署运行手册.md)
- [游戏系统集成接口](docs/游戏系统集成接口.md)

## 1. 仓库结构

```text
.
├── backend/
│   ├── cmd/                         # gateway / service / worker / migrate / compose render 入口
│   ├── internal/
│   │   ├── contract/               # 服务间同步 contract
│   │   ├── eventbus/               # outbox / envelope / delivery / retry / dead-letter
│   │   ├── gateway/                # gateway proxy / ready probe / metrics
│   │   ├── http/                   # HTTP transport / route registration / middleware
│   │   ├── metrics/                # Prometheus 指标注册
│   │   ├── services/               # 业务域服务
│   │   ├── syncclient/             # 统一 HTTP client 治理
│   │   ├── telemetry/              # tracing / OTLP exporter
│   │   └── worker/                 # notification / data-platform-sync runtime
├── frontend/                       # React + Vite 管理台
├── scripts/                        # 启动 / 迁移 / 运维 / 发布 / 回滚脚本
├── deploy/                         # release/runtime/observability 产物
├── docs/                           # 架构、runbook、交付文档
├── docker-compose.yml              # 本地基础联调
└── docker-compose.microservices.yml
```

## 2. 当前架构

### 2.1 HTTP 服务

| 服务 | 端口 | 职责 |
|---|---:|---|
| `gateway-service` | 8080 | 统一入口、按路径转发、上游 readiness 探测 |
| `identity-service` | 8081 | 登录、当前登录态、RBAC |
| `tenant-service` | 8082 | 租户、品牌、平台配置 |
| `agent-service` | 8083 | 代理、邀请码、玩家、绑定 |
| `relation-service` | 8084 | 代理祖先链、后代链、团队统计 |
| `game-service` | 8085 | 游戏目录、代理游戏授权 |
| `rule-service` | 8086 | 佣金规则、快照 |
| `activity-service` | 8087 | 活动奖励 |
| `recharge-service` | 8088 | 充值订单、OpenAPI、回调 |
| `settlement-service` | 8089 | 佣金、结算单、重算 |
| `account-service` | 8090 | 代理账户、台账 |
| `withdrawal-service` | 8091 | 提现申请、审核、打款 |
| `risk-service` | 8092 | 风控案件、风险情报 |
| `report-service` | 8093 | 报表、数据平台指标 |
| `audit-service` | 8094 | 审计查询 |

### 2.2 Worker

| 服务 | 端口 | 职责 |
|---|---:|---|
| `notification-service` | 8095 | 通知事件消费 |
| `data-platform-sync-service` | 8096 | 数仓同步事件消费 |

### 2.3 基础设施

| 依赖 | 端口 | 说明 |
|---|---:|---|
| `mysql` | 3306 | 当前统一 OLTP 主库 |
| `redis` | 6379 | 锁、缓存、幂等辅助 |
| `rabbitmq` | 5672 / 15672 | MQ 分发、本地管理台 |

### 2.4 总体拓扑

```text
frontend(:5173)
      |
      v
gateway(:8080)
      |
      +--> identity(:8081)
      +--> tenant(:8082)
      +--> agent(:8083)
      +--> relation(:8084)
      +--> game(:8085)
      +--> rule(:8086)
      +--> activity(:8087)
      +--> recharge(:8088)
      +--> settlement(:8089)
      +--> account(:8090)
      +--> withdrawal(:8091)
      +--> risk(:8092)
      +--> report(:8093)
      +--> audit(:8094)

notification-worker(:8095)
data-platform-sync-worker(:8096)

all services
   +--> MySQL
   +--> Redis
   +--> RabbitMQ
```

## 3. 当前已完成能力

### 3.1 网关治理

- `gateway /healthz` 只表示进程存活
- `gateway /readyz` 会逐个主动探测全部 HTTP 上游的 `/readyz`
- 探测透传 `X-Request-ID` / `X-Trace-ID`
- 任一关键上游失败时返回 `503`，并携带依赖明细

### 3.2 Prometheus / 日志 / tracing

- gateway、HTTP 服务、worker 统一暴露 Prometheus `/metrics`
- 已覆盖 HTTP 请求总数、请求耗时、ready 状态、worker 轮询/投递/锁获取指标
- 入站支持 `traceparent`
- 下游同步调用会透传 `requestID` / `traceID`
- OTLP exporter 已接入，默认可关闭

### 3.3 事件治理

- DB outbox
- 统一事件 envelope
- worker 原子抢占 queued delivery
- 指数退避重试
- dead-letter
- MQ 开启时走 RabbitMQ，关闭时可回退 DB fallback consume

### 3.4 同步调用治理

- 统一入口：`backend/internal/syncclient`
- 统一边界：`backend/internal/contract/*`
- `risk-service` 已通过 contract 调 `report/account/withdrawal`
- `withdrawal-service` 已通过 contract 调 `account/risk`
- `recalculation-service` 已通过 contract 调 `settlement`
- `backend/internal/architecture/sync_boundary_test.go` 会阻止新增未登记的跨服务直接 import

### 3.5 补偿链

- `withdrawal payout` 成功链路会触发 `risk release`
- 若后续本地事务失败，会调用 `restore-pending-cases` 补偿恢复
- `restore` 分支会在同一事务内补冻结账本、回写案件状态并发布 `risk.case.restored`

## 4. 当前未完成项

- 仍未完成独立数据库自治
- 服务仍共享同一套 DB/schema/model
- 读模型仍未完全事件投影化
- 仍缺远端制品仓库、审批、灰度发布、自动化回滚
- 规则平台终态、智能风控终态、数据中台成品层仍未完成

一句话：

```text
[同步调用治理] = 已收口
[补偿链/事件链] = 已收口
[独立存储自治] = 未完成
```

## 5. 环境要求

- Go `1.25+`
- Node.js `18+`
- npm `9+`
- Docker / Docker Compose

推荐在 macOS / Linux 下执行脚本。

## 6. 快速启动

### 6.1 推荐方式：完整微服务拓扑

```bash
cd /Users/leon/project/game-admin
./scripts/run-microservices.sh up
```

迁移：

```bash
./scripts/migrate-microservices.sh
```

批量验活：

```bash
./scripts/microservice-ops.sh health
./scripts/microservice-ops.sh ready
./scripts/microservice-ops.sh metrics gateway-service notification-service
```

访问地址：

- Frontend: `http://localhost:5173`
- Gateway: `http://localhost:8080`
- RabbitMQ UI: `http://localhost:15672`
- Jaeger: `http://localhost:16686`
- Grafana: `http://localhost:3000`

### 6.2 直接用 compose

```bash
docker compose -f docker-compose.microservices.yml up --build
```

### 6.3 只启动部分服务

```bash
./scripts/run-microservices.sh up identity-service tenant-service audit-service
./scripts/migrate-microservices.sh identity-service tenant-service audit-service
```

## 7. 常用命令

### 7.1 后端验证

```bash
cd /Users/leon/project/game-admin/backend
GOCACHE=/Users/leon/project/game-admin/.cache/go-build GIN_MODE=release go test ./...
GOCACHE=/Users/leon/project/game-admin/.cache/go-build go vet ./...
```

### 7.2 前端开发

```bash
cd /Users/leon/project/game-admin/frontend
npm install
npm run dev
```

### 7.3 单服务 migrate

```bash
cd /Users/leon/project/game-admin/backend
go run ./cmd/migrate-service identity-service
go run ./cmd/migrate-service tenant-service
go run ./cmd/migrate-service audit-service
```

### 7.4 服务级发布产物

```bash
cd /Users/leon/project/game-admin
./scripts/release-microservices.sh gateway-service identity-service tenant-service notification-service
./scripts/promote-release.sh gateway-service
./scripts/rollback-release.sh gateway-service
```

## 8. 健康检查与指标

### 8.1 通用约定

- `/healthz`：进程存活
- `/readyz`：依赖可用、可以接流量
- `/metrics`：Prometheus exposition

### 8.2 示例

```bash
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/readyz
curl http://127.0.0.1:8080/metrics

curl http://127.0.0.1:8081/readyz
curl http://127.0.0.1:8095/readyz
curl http://127.0.0.1:8095/metrics
```

## 9. 同步调用边界

```text
[gateway]
   -> [HTTP services]        allowed

[risk-service]
   -> [report/account/withdrawal contract] allowed

[withdrawal-service]
   -> [account/risk contract] allowed

[recalculation-service]
   -> [settlement contract] allowed

[all services]
   -> [shared DB/schema]     legacy boundary
```

禁止新增：

- 未经登记的 `internal/services/*` 跨包直接 import
- 绕过 `internal/contract/*` 的新增同步耦合
- 把 `requestID` / `traceID` 当 metrics label

## 10. 关键脚本

| 脚本 | 作用 |
|---|---|
| `scripts/run-microservices.sh` | 启动微服务拓扑 |
| `scripts/migrate-microservices.sh` | 批量迁移 |
| `scripts/microservice-ops.sh` | 健康检查、ready、metrics、start/stop |
| `scripts/release-microservices.sh` | 生成服务级发布产物 |
| `scripts/promote-release.sh` | 本地运行时晋级 |
| `scripts/rollback-release.sh` | 本地运行时回滚 |
| `scripts/observability.sh` | 启动/停止/检查 Prometheus + Jaeger + Grafana |

## 11. 关键代码入口

- Gateway: `backend/cmd/gateway-service`
- HTTP 路由层：`backend/internal/http`
- 服务定义与端口：`backend/internal/servicedef/catalog.go`
- 路由元数据：`backend/internal/routing/metadata.go`
- Contract：`backend/internal/contract`
- Sync client：`backend/internal/syncclient/http.go`
- Eventbus：`backend/internal/eventbus`
- Worker runtime：`backend/internal/worker/runtime.go`
- Metrics：`backend/internal/metrics`
- Telemetry：`backend/internal/telemetry`

## 12. 说明

- 旧的 `backend/cmd/server` 和 `docker-compose.yml` 仍保留，适合最小本地联调；当前主线架构以 `docker-compose.microservices.yml` 和 `scripts/*microservices*.sh` 为准
- 当前项目已经不适合再用“第一阶段 SQLite 原型”来描述
- 如果你的目标是“继续做治理/架构演进”，请先看：
  - [docs/当前系统架构.md](docs/当前系统架构.md)
  - [docs/微服务部署运行手册.md](docs/微服务部署运行手册.md)
  - [docs/游戏系统集成接口.md](docs/游戏系统集成接口.md)
