# Game Admin

第一阶段完善版的代理后台原型，包含：
- backend：Go + Gin + Gorm 的后端服务
- frontend：React + Vite + Ant Design 的管理台

当前仓库目标是让团队可以在本地快速拉起一个可演示、可联调、可继续迭代的最小运行基线。

当前仍以 SQLite 单机基线为主，但已经补充了面向 MySQL / Redis / MQ 的轻量配置预留，便于后续逐步演进而不提前引入重型架构改造。

## 第一阶段范围

第一阶段聚焦以下能力：
- 管理台基础框架与页面骨架
- 后端健康检查与基础 REST API
- 最小登录鉴权基线：`/api/auth/login` + Bearer Token
- 代理、邀请码、游戏、佣金规则、订单、佣金、台账、审计等基础管理接口
- SQLite 本地数据存储，适合本地开发和演示

不包含：
- 完整 RBAC 权限体系
- 动态账号体系与密码管理
- 生产级部署编排
- 外部支付/三方系统正式接入

## 目录结构

```text
.
├── backend/                 # Go 服务
│   ├── cmd/server/          # 后端启动入口
│   ├── internal/app/        # 应用启动、数据库初始化
│   ├── internal/config/     # 环境变量配置读取
│   ├── internal/http/       # 路由、鉴权、中间件、接口实现
│   └── data/                # 默认 SQLite 数据目录（运行后生成）
├── frontend/                # React + Vite 管理台
│   ├── src/app/             # 路由与应用入口
│   ├── src/pages/           # 页面模块
│   ├── src/lib/             # API 请求封装
│   └── public/              # 静态资源
├── .env.example             # 本地环境变量示例
└── docker-compose.yml       # 最小本地联调基线
```

## 本地启动前提

建议环境：
- Go 1.25+
- Node.js 18+
- npm 9+

推荐先执行初始化脚本：

```bash
./scripts/init.sh
```

常用参数：
- `--skip-tests`：跳过 `go test ./...`
- `--skip-build`：跳过 `npm run build`
- `--force-npm-install`：即使依赖目录已存在也强制重新执行 `npm install`

脚本会自动完成：
- 从 `.env.example` 复制生成 `.env`（若不存在）
- 创建 `backend/data/` 数据目录
- 在依赖缺失或 `frontend/package-lock.json` 更新时安装前端依赖
- 默认执行 `go test ./...`
- 默认执行 `npm run build`

已知当前代码基线：
- 后端已通过 `go test ./...`
- 前端已通过 `npm run build`

## 环境变量

项目当前默认仍以 SQLite 运行，详见根目录 `.env.example`。同时已经预留 Redis / MQ 配置项，默认关闭，不会影响现有本地运行方式。

后端支持：
- `APP_ENV`：运行环境，默认 `development`
- `HTTP_PORT`：后端监听端口，默认 `8080`
- `DATABASE_DRIVER`：数据库驱动标识，默认 `sqlite`，当前代码仍实际使用 SQLite；后续切换 MySQL 时可复用该配置位
- `DATABASE_DSN`：数据库连接串，SQLite 默认 `data/app.db`；若后续演进 MySQL，可改为标准 MySQL DSN
- `REDIS_ENABLED`：是否启用 Redis 相关基础设施配置，默认 `false`
- `REDIS_ADDR` / `REDIS_PASSWORD` / `REDIS_DB` / `REDIS_PREFIX`：Redis 连接与 key 前缀预留
- `MQ_ENABLED`：是否启用消息队列配置，默认 `false`
- `MQ_DRIVER` / `MQ_URL` / `MQ_TOPIC`：消息队列类型、连接串与默认 topic/queue 名称预留

前端支持：
- `VITE_API_BASE_URL`：前端请求后端 API 的基础地址，默认 `http://localhost:8080/api`

## 最小登录信息

当前第一阶段鉴权为固定演示账号，定义在 `backend/internal/http/router.go`：
- 用户名：`admin`
- 密码：`admin123`
- 登录接口：`POST /api/auth/login`
- 成功返回固定 token：`phase1-admin-token`

说明：
- 前端登录页会调用 `/api/auth/login`
- 登录成功后会自动把 Bearer Token 存到浏览器 localStorage
- 受保护接口需要携带 `Authorization: Bearer phase1-admin-token`

示例：

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}'
```

健康检查：

```bash
curl http://localhost:8080/healthz
```

## 后端启动方式

如未完成本地准备，可先执行：

```bash
./scripts/init.sh
```

后端启动入口：`backend/cmd/server/main.go`

1) 安装依赖并启动

```bash
cd backend
go run ./cmd/server
```

默认行为：
- 监听 `:8080`
- 自动创建/使用 SQLite 数据库 `backend/data/app.db`
- 启动时自动执行数据库迁移

2) 自定义端口或数据库路径

```bash
cd backend
APP_ENV=development HTTP_PORT=8080 DATABASE_DSN=data/app.db go run ./cmd/server
```

3) 启动后验证

```bash
curl http://localhost:8080/healthz
```

## 前端启动方式

前端默认端口来自 `frontend/vite.config.ts`：`5173`

1) 安装依赖

```bash
cd frontend
npm install
```

2) 启动开发服务器

```bash
cd frontend
VITE_API_BASE_URL=http://localhost:8080/api npm run dev
```

3) 浏览器访问

```text
http://localhost:5173
```

如果不额外设置 `VITE_API_BASE_URL`，当前代码默认也会请求：

```text
http://localhost:8080/api
```

## 一键本地联调（docker compose）

仓库提供了一个最小 `docker-compose.yml`，用于快速本地联调，不做生产化设计。

说明：
- 当前 backend 仍只连接 SQLite
- compose 中已补充 `redis` 与 `rabbitmq` 服务，作为后续缓存、异步任务、事件分发演进的本地依赖基线
- backend 默认通过环境变量保留这些依赖地址，但默认 `REDIS_ENABLED=false`、`MQ_ENABLED=false`，因此不会改变现有业务逻辑

启动：

```bash
docker compose up --build
```

访问：
- 前端：http://localhost:5173
- 后端：http://localhost:8080
- 健康检查：http://localhost:8080/healthz
- Redis：localhost:6379
- RabbitMQ AMQP：localhost:5672
- RabbitMQ 管理台：http://localhost:15672

停止：

```bash
docker compose down
```

说明：
- compose 会挂载源码目录，适合作为最小开发联调基线
- SQLite 数据文件默认落在容器内 `/app/backend/data/app.db`
- Redis / RabbitMQ 当前未挂载持久化 volume，保持轻量
- 如需保留数据，可继续补 volume；第一阶段先保持简单

## 部署脚本

仓库新增 `scripts/deploy.sh`，用于在本机生成一份可运行的部署产物目录，不直接做远程发布。

适用场景：
- 本地/测试机打包 backend 二进制与 frontend 静态资源
- 在不引入额外 CI/CD 编排的前提下，统一部署产物结构
- 为后续接入 rsync、scp、制品仓库或容器化发布提供基础出口

脚本行为：
- 自动读取根目录 `.env`（若存在）
- 生成 `backend/.env` 与 `frontend/.env.production`
- 构建后端二进制：`backend/bin/server`
- 构建前端静态资源：`frontend/dist`
- 输出部署目录：`deploy/release`
- 生成启动脚本：`deploy/release/run-backend.sh`、`deploy/release/run-frontend.sh`
- 默认在执行结束后清理工作区中的 `frontend/.env.production`，避免遗留生产构建配置

使用方式：

```bash
cp .env.example .env
chmod +x scripts/deploy.sh
./scripts/deploy.sh
```

常用参数：

```bash
./scripts/deploy.sh --api-base-url https://example.com/api
./scripts/deploy.sh --skip-backend
./scripts/deploy.sh --skip-frontend
./scripts/deploy.sh --keep-frontend-env
```

参数说明：
- `--api-base-url URL`：覆盖 `PUBLIC_API_BASE_URL`，并写入前端构建时使用的 `VITE_API_BASE_URL`
- `--skip-backend`：跳过后端编译，复用已有 `backend/bin/server`
- `--skip-frontend`：跳过前端构建，复用已有 `frontend/dist`
- `--keep-frontend-env`：构建完成后保留工作区中的 `frontend/.env.production`

产物结构示例：

```text
deploy/
├── release/
│   ├── backend/
│   │   ├── server
│   │   └── .env
│   ├── frontend/
│   │   ├── dist/
│   │   └── .env.production
│   ├── run-backend.sh
│   ├── run-frontend.sh
│   ├── .env.example
│   └── README.md
└── runtime/
    ├── backend/data/
    └── logs/
```

部署脚本默认约定：
- 后端默认以 `APP_ENV=production` 打包
- SQLite 默认运行文件路径为 `../runtime/backend/data/app.db`
- 前端构建时使用 `PUBLIC_API_BASE_URL` 写入 `VITE_API_BASE_URL`
- `run-frontend.sh` 优先使用 `python3` 启静态文件服务，若不可用则回退 `npx serve`
- 跳过构建时会校验复用产物是否已存在，避免静默生成不完整发布包

本地验证部署产物：

```bash
./deploy/release/run-backend.sh
./deploy/release/run-frontend.sh 4173
```

访问地址：
- 前端：http://localhost:4173
- 后端健康检查：http://localhost:8080/healthz

## 基础设施演进基线建议

为了避免当前阶段过度设计，本仓库仅做“配置先行、实现后补”的轻量落地：

1. 数据库演进
- 当前默认：SQLite，本地演示与快速联调最省成本
- 已预留：`DATABASE_DRIVER` + `DATABASE_DSN`
- 后续建议：当出现并发写入、事务一致性、审计追踪或运维要求时，再切到 MySQL，并在 `internal/app` 内补充按 driver 分支打开数据库

2. Redis 演进
- 当前状态：未接入代码路径
- 已预留：开关、地址、密码、库号、key 前缀
- 后续适用场景：登录态黑名单、验证码/短 TTL 数据、列表缓存、幂等键、分布式锁

3. MQ 演进
- 当前状态：未接入代码路径
- 已预留：开关、驱动、连接串、默认 topic
- 后续适用场景：订单后置处理、佣金异步结算、审计事件投递、站内通知/运营事件

4. 当前边界
- 这次改动不引入新的运行时依赖到业务代码
- 不修改现有 SQLite 启动逻辑
- 不增加复杂部署编排，仅补充本地环境基线与文档约束

## 基础设施说明

第一阶段的基础设施基线与演进建议已整理到：


文档重点说明：
- 当前已落地的基础设施基线：backend + frontend + SQLite
- 已预留但默认关闭的 Redis / RabbitMQ 能力
- `docker-compose.yml` 提供的本地联调依赖范围
- 第一阶段内推荐的近期开演进路径（先保持 SQLite 基线，再按需要引入 MySQL / Redis / MQ）

## 测试与构建命令

后端：

```bash
cd backend
go test ./...
go build ./cmd/server
```

前端：

```bash
cd frontend
npm run build
npm run preview
```

## 推荐本地启动顺序

1) 初始化项目

```bash
./scripts/init.sh
```

2) 启动后端

```bash
cd backend
go run ./cmd/server
```

3) 启动前端

```bash
cd frontend
VITE_API_BASE_URL=http://localhost:8080/api npm run dev
```

4) 浏览器访问并登录
- 打开 `http://localhost:5173`
- 使用 `admin / admin123` 登录

## 已知限制

- 当前登录凭证与 token 为代码内固定值，仅适合第一阶段演示
- 前端未配置 Vite 代理，默认直接访问 `VITE_API_BASE_URL`
- 默认数据库为 SQLite，本地开发方便，但不代表生产方案；`DATABASE_DRIVER` 目前主要作为演进占位配置
- Redis / MQ 目前仅完成配置与 compose 预留，业务代码尚未实际消费这些依赖
- `frontend/` 当前已存在构建产物与 `node_modules`，提交前可按团队规范决定是否保留
- docker compose 为最小联调用途，未做生产级镜像裁剪、健康依赖编排、数据持久化治理

## 常见问题

1. 前端登录后接口仍 401
- 确认后端已启动在 `http://localhost:8080`
- 确认登录返回 token 成功
- 确认浏览器 localStorage 中已写入 token

2. 前端打不开数据
- 确认 `VITE_API_BASE_URL` 指向 `http://localhost:8080/api`
- 确认后端接口可通过 curl 访问

3. 后端数据库文件未生成
- 首次启动后端才会创建 `backend/data/app.db`
- 如目录权限异常，请确认 `backend/data/` 可写
