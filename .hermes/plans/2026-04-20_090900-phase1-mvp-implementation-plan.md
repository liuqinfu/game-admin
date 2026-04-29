# 第一阶段 MVP 实施计划

> For Hermes: 采用总控 + 并行子 agent 执行；实现时遵循 TDD，先测试后实现；完成后由总控统一验收。

目标：基于需求文档完成第一阶段可运行的最小可用版本，后端使用 Go，前端使用 React，优先打通“代理/邀请码/用户绑定/游戏/规则/充值订单/异步结算/账户台账/审计”的核心闭环。

架构：采用“前后端分离 + 模块化单体后端 + React 管理台”的实现方式。后端优先提供 REST API、领域模块、数据库迁移、异步事件与幂等结算；前端优先提供管理后台 MVP 页面。由于当前仓库仅有需求文档，无现成代码，需要从 0 到 1 建立项目骨架，并优先交付阶段一主链路。

技术栈：
- 后端：Go 1.22+, Gin, GORM, SQLite(开发)/MySQL 兼容建模, Redis/MQ 接口预留, Testify
- 前端：React + Vite + TypeScript + Ant Design + React Router + TanStack Query
- 工具：OpenAPI 风格 REST, Makefile, Docker Compose（可选后续补）

---

## 当前上下文与假设

1. 当前工作区仅包含需求/设计文档，没有任何现成后端或前端代码。
2. 第一阶段需要的是“可运行 MVP”，不是完整生产级平台。
3. 为了在短周期交付，开发环境的数据层可先用 SQLite 落地，但数据模型和 repository 设计需兼容 MySQL 8。
4. MQ/Redis 可先做接口抽象与内存事件总线，保留替换点；充值结算流程必须保留异步语义与幂等能力。
5. 后端优先覆盖以下 API：代理、邀请码、用户注册绑定、游戏、规则、充值回调、佣金明细、台账查询。
6. 前端优先实现 admin-web MVP，agent-portal 可以先只留占位或复用只读查询，不作为首批强制交付目标。

## 总控任务拆解

### 可并行主任务

A. 后端工程骨架与基础设施
B. 核心领域模型与数据库迁移
C. 代理/邀请码/用户绑定/游戏/规则 API
D. 充值订单接入 + 结算 + 台账主链路
E. React 管理后台 MVP
F. 集成测试/端到端验收脚本

其中：
- A 与 E 可并行
- B 可在 A 完成基础骨架后并行推进
- C 与 D 在 B 基础上可部分并行
- F 需在 C/D/E 基本完成后执行

## 交付范围（第一阶段）

必须实现：
1. 后端项目初始化与分层结构
2. 数据模型与自动迁移
3. 代理管理：创建、审核、状态变更、列表/详情
4. 邀请码管理：创建、列表、状态校验
5. 用户注册绑定：注册并绑定邀请码，记录绑定历史
6. 游戏管理：创建、更新、上下架、代理授权
7. 规则管理：创建、查询、发布
8. 充值回调：验签（MVP 可用静态签名策略）、幂等、订单落库
9. 异步结算：按有限层生成佣金记录与台账流水
10. 管理后台：代理、邀请码、游戏、规则、订单/佣金/台账查询基础页面
11. 测试：后端单元/集成测试，至少覆盖注册绑定、重复回调幂等、规则快照、结算入账

暂缓：
- 完整 RBAC
- Kafka/RabbitMQ 实际接入
- Redis 强依赖缓存
- 代理门户完善
- 多租户、多品牌、提现、复杂分润、周期结算单

## 建议目录结构

- `backend/`
  - `cmd/server/main.go`
  - `internal/app/`
  - `internal/domain/`
  - `internal/repository/`
  - `internal/service/`
  - `internal/http/`
  - `internal/settlement/`
  - `internal/events/`
  - `internal/test/`
  - `migrations/`（或 GORM AutoMigrate 初始化）
  - `go.mod`
- `frontend/`
  - `src/main.tsx`
  - `src/app/router.tsx`
  - `src/layouts/AdminLayout.tsx`
  - `src/pages/agents/*`
  - `src/pages/invite-codes/*`
  - `src/pages/games/*`
  - `src/pages/rules/*`
  - `src/pages/orders/*`
  - `src/pages/commissions/*`
  - `src/pages/ledger/*`
  - `src/lib/api.ts`
  - `package.json`
- `docs/`
  - `phase1-architecture.md`
  - `api-contracts.md`
- `Makefile`
- `README.md`

## 分任务实施步骤

### 任务 1：建立项目骨架
目标：初始化 backend/frontend 两个应用与基础 README。
文件：
- Create: `backend/go.mod`
- Create: `backend/cmd/server/main.go`
- Create: `backend/internal/...`
- Create: `frontend/package.json`
- Create: `frontend/vite.config.ts`
- Create: `frontend/src/main.tsx`
- Create: `README.md`

验证：
- `cd backend && go test ./...`
- `cd backend && go run ./cmd/server`
- `cd frontend && npm install && npm run build`

### 任务 2：建立核心领域模型与数据库迁移
目标：把阶段一关键表落成代码模型和迁移。
文件：
- Create: `backend/internal/domain/models/*.go`
- Create/Modify: `backend/internal/app/db.go`
- Create: `backend/internal/app/migrate.go`
- Test: `backend/internal/domain/models/models_test.go`

验证：
- `cd backend && go test ./...`
- 启动服务时自动迁移成功

### 任务 3：代理、邀请码、用户绑定 API
目标：打通代理创建/审核/邀请码生成/用户注册绑定。
文件：
- Create: `backend/internal/service/agent_service.go`
- Create: `backend/internal/service/player_service.go`
- Create: `backend/internal/http/agent_handler.go`
- Create: `backend/internal/http/player_handler.go`
- Test: `backend/internal/http/agent_handler_test.go`
- Test: `backend/internal/http/player_handler_test.go`

关键验证：
- 冻结代理无法生成邀请码
- 无效邀请码无法注册
- 重复绑定不会生成重复记录

### 任务 4：游戏、规则管理 API
目标：实现游戏 CRUD、授权、规则创建/发布/查询。
文件：
- Create: `backend/internal/service/game_service.go`
- Create: `backend/internal/service/rule_service.go`
- Create: `backend/internal/http/game_handler.go`
- Create: `backend/internal/http/rule_handler.go`
- Test: `backend/internal/http/game_handler_test.go`
- Test: `backend/internal/http/rule_handler_test.go`

关键验证：
- 已下架游戏不可新增推广授权
- 规则冲突时按优先级/生效时间/版本策略唯一命中（MVP 至少实现查询排序口径）

### 任务 5：充值回调、异步结算、规则快照、台账
目标：打通充值入库 -> 事件发布 -> 佣金计算 -> 台账入账。
文件：
- Create: `backend/internal/service/recharge_service.go`
- Create: `backend/internal/settlement/processor.go`
- Create: `backend/internal/service/account_service.go`
- Create: `backend/internal/http/recharge_handler.go`
- Create: `backend/internal/http/query_handler.go`
- Test: `backend/internal/settlement/processor_test.go`
- Test: `backend/internal/http/recharge_handler_test.go`

关键验证：
- 同一渠道订单号重复回调仅成功一次
- 历史订单结算依赖规则快照
- 每层生成佣金记录并对应台账流水

### 任务 6：React 管理后台 MVP
目标：搭建管理台并对接后端基础 API。
文件：
- Create: `frontend/src/app/router.tsx`
- Create: `frontend/src/layouts/AdminLayout.tsx`
- Create: `frontend/src/pages/agents/index.tsx`
- Create: `frontend/src/pages/invite-codes/index.tsx`
- Create: `frontend/src/pages/games/index.tsx`
- Create: `frontend/src/pages/rules/index.tsx`
- Create: `frontend/src/pages/orders/index.tsx`
- Create: `frontend/src/pages/commissions/index.tsx`
- Create: `frontend/src/pages/ledger/index.tsx`
- Create: `frontend/src/lib/api.ts`

关键验证：
- 能查看代理/邀请码/游戏/规则/订单/佣金/台账列表
- 能创建代理、邀请码、游戏、规则
- 页面可构建通过

### 任务 7：集成联调与验收
目标：验证从代理创建到充值入账的完整闭环。
文件：
- Create: `backend/internal/test/phase1_flow_test.go`
- Create: `docs/phase1-architecture.md`
- Create: `docs/api-contracts.md`
- Modify: `README.md`

验收用例：
1. 创建代理并审核启用
2. 生成邀请码
3. 用户携邀请码注册绑定
4. 创建游戏与规则
5. 提交充值回调
6. 查询佣金记录与台账流水
7. 重复回调验证幂等

## 子 agent 并行分工建议

并行组 1：后端基础骨架 + 数据模型
并行组 2：后端业务 API（代理/邀请码/绑定/游戏/规则）
并行组 3：结算链路（充值/快照/佣金/台账/幂等）
并行组 4：前端管理台 MVP

## 风险与设计取舍

1. 当前仓库无代码，真正完成“第一阶段全部目标”工作量较大，本轮优先产出可运行 MVP。
2. MQ/Redis 若强行真实接入会抬高启动成本，因此先以内存事件总线 + 清晰接口抽象保证后续演进。
3. RBAC 先采用简化占位方案：请求头模拟角色或固定后台入口，后续再补全账号/角色/权限点。
4. 审计日志先覆盖关键操作写库，先不做完整检索/导出能力。
5. 数据库先用 SQLite 提升落地速度，但 SQL 字段和索引命名按 MySQL 习惯设计。

## 总控执行策略

1. 先初始化项目骨架。
2. 再并行委派：
   - 子 agent A：后端基础与领域模型
   - 子 agent B：后端业务 API
   - 子 agent C：充值结算链路
   - 子 agent D：前端管理台
3. 总控合并后亲自运行：
   - `go test ./...`
   - `npm run build`
   - 关键端到端验收
4. 若子 agent 失败，总控直接修复或重新分派。

## 完成定义

满足以下条件才算第一阶段目标达成（MVP 口径）：
- 后端可启动
- 前端可构建
- 核心接口可用
- 充值到佣金台账闭环可跑通
- 幂等与规则快照有测试覆盖
- 总控完成统一验收并输出剩余差距
