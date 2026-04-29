# 第三阶段第一里程碑：平台上下文 + 租户隔离基础版 Implementation Plan

> For Hermes: Use subagent-driven-development skill to implement this plan task-by-task.

Goal: 在不大规模重构现有单体架构的前提下，为系统建立第 3 阶段的平台化地基：统一 tenant/brand 上下文、在关键运营/财务页面与接口上引入租户隔离、补齐越权校验与审计支撑。

Architecture: 采用“现有单体增量平台化”的方式推进。后端先在已有 tenant/brand、RBAC、audit、withdrawal、report、risk、settlement 基础上补充 scope 校验和 tenant-aware 查询；前端在现有 AdminLayout 与各业务页之上增加全局平台上下文（activeTenantId/activeBrandId）并透传到支持的查询。第一里程碑只做平台化基础，不做活动中心、智能风控评分模型、数仓链路等扩域开发。

Tech Stack: Go + Gin + GORM backend, React 18 + Vite + Ant Design + TanStack Query frontend, existing RBAC/auth/audit infrastructure.

---

## 1. Current Context / Evidence

后端现状（来自代码巡检）
- `backend/internal/http/router.go`
  - 已存在 `/api/tenants`、`/api/brands` 基础 CRUD
  - `brands` 列表已支持 `tenantID` 查询参数过滤
  - `withdrawalCreatePayload` 仍使用 `tenantCode`
  - `/api` 整体使用 `authMiddleware(deps.DB)` + `requirePermission(...)`
  - `settlement-bills`、`withdrawal-requests`、`report/*`、`risk/*` 已存在
- 已存在 tenant/brand 模型与审计/RBAC 能力，但关键业务接口尚未显式 tenant-aware
- 当前主要风险：
  1. 多数核心数据接口仍是全局查询
  2. 提现流 tenant 关联不够强（`tenantCode`）
  3. 缺少统一 tenant scope 校验辅助函数

前端现状（来自代码巡检）
- `frontend/src/layouts/AdminLayout.tsx` 已有成熟的壳子和权限导航
- 已有 `/tenant-brand`、`/withdrawal-requests`、`/reports`、`/risk` 页面
- 当前页面普遍没有全局 active tenant / brand context
- 当前查询参数中几乎没有跨模块稳定透传 `tenantId` / `brandId`

第一里程碑原则
- DRY: 统一 scope helper，不在每个 handler/page 重复散落逻辑
- YAGNI: 只做 tenant/brand 上下文与隔离，不预先引入复杂租户详情工作台
- TDD: 先补后端越权/过滤测试，再实现；前端以 build + 页面行为验证为主

---

## 2. Milestone Scope

In scope
1. 后端：对以下模块增加 tenant-aware 查询/校验
   - settlement bills
   - withdrawal requests
   - reports
   - risk views
2. 后端：新增统一平台 scope 解析辅助函数
3. 后端：补充平台管理员 / 租户管理员越权测试
4. 前端：新增全局平台上下文（activeTenantId / activeBrandId）
5. 前端：在 AdminLayout 增加 tenant / brand selector
6. 前端：在 `reports` / `withdrawal-requests` / `risk` / `tenant-brand` 页面透传 scope 并展示当前上下文

Out of scope
- 活动奖励中心
- 智能风控评分引擎
- 数仓 ETL / BI
- 微服务拆分
- 全量业务表一次性 tenant_id/brand_id 改造

---

## 3. Proposed Data/Permission Strategy

### 3.1 Scope strategy

平台上下文定义：
- activeTenantId: 当前选中的租户（可空，表示平台全局视角）
- activeBrandId: 当前选中的品牌（可空，表示租户级视角）

角色语义（第一版）
- 平台管理员：可以不带 tenant scope 查看全局，或主动切换到某租户/品牌
- 租户管理员：必须受限于其所属 tenant；若传入其他 tenantID，应返回 403

### 3.2 Backend enforcement strategy

新增统一 helper（建议放在 `backend/internal/http/router.go`，后续再抽离）
- `currentAdminIdentity(c *gin.Context) ...`
- `resolveTenantScope(c *gin.Context, db *gorm.DB) (tenantID *uint64, err error)`
- `enforceTenantScope(c *gin.Context, requestedTenantID *uint64) error`
- `applyTenantScope(query *gorm.DB, tenantID *uint64, column string) *gorm.DB`

第一版优先策略：
- 若现有表有明确 tenant 归属字段，则直接按 tenant 字段过滤
- 若无直接 tenant_id，但可从 agent / brand / tenantCode 推导，则通过 join/lookup 做校验
- 对无法安全判定 tenant 归属的接口，第一版先只支持平台管理员或显式返回限制错误，避免假隔离

### 3.3 Frontend context strategy

新增平台上下文模块：
- `frontend/src/platform-scope/PlatformScopeProvider.tsx`
- `frontend/src/platform-scope/usePlatformScope.ts`

状态：
- `activeTenantId?: number`
- `activeBrandId?: number`
- `setActiveTenantId`
- `setActiveBrandId`
- 当 tenant 切换时，若 brand 不属于新 tenant，则自动清空 activeBrandId

UI：
- 在 `AdminLayout.tsx` 顶部工具栏增加 tenant / brand Select
- tenant 变化时动态刷新 brand 列表
- 在目标页面 hero/toolbars 显示当前 scope badge / summary

---

## 4. Bite-Sized Task Plan

### Task 1: 盘点后端关键接口的 tenant 归属路径

Objective: 明确 settlement / withdrawal / reports / risk 各自可以通过哪个字段或关联实现 tenant scope 校验。

Files:
- Inspect: `backend/internal/http/router.go`
- Inspect: `backend/internal/domain/model/models.go`
- Test: `backend/internal/http/router_test.go`

Step 1: 列出以下接口的 tenant 归属来源
- `/api/settlement-bills`
- `/api/withdrawal-requests`
- `/api/report/*`
- `/api/risk/*`

Step 2: 为每个接口标注
- 直接 tenant 字段
- 间接 agent -> tenant 映射
- tenantCode
- 暂时不可安全限定

Step 3: 将结果整理成实现注释或开发 checklist（不写用户文档）

Verification:
- 实现前，开发者能明确每个接口使用哪条 tenant 归属链路，不靠猜。

### Task 2: 为后端新增统一 tenant scope helper 的失败测试

Objective: 先写出租户管理员越权失败、平台管理员放行的测试骨架。

Files:
- Modify: `backend/internal/http/router_test.go`

Step 1: 新增失败测试
- `TestPhase3TenantScopedSettlementBillAccess`
- `TestPhase3TenantScopedWithdrawalAccess`
- `TestPhase3TenantScopedReportAccess`
- `TestPhase3TenantScopedRiskAccess`

Step 2: 每个测试至少覆盖
- 平台管理员跨 tenant 成功
- 租户管理员访问本 tenant 成功
- 租户管理员访问其他 tenant 返回 403

Step 3: 运行单测确认先失败

Run:
- `go test ./internal/http/... -run Phase3TenantScoped -v`

Expected:
- FAIL，提示当前 handler 未做 tenant scope enforcement 或结果不符合预期

### Task 3: 实现后端 tenant scope helper

Objective: 提供统一 scope 解析与 query 应用能力，避免在各 handler 重复解析 tenantID。

Files:
- Modify: `backend/internal/http/router.go`
- Test: `backend/internal/http/router_test.go`

Step 1: 在 router.go 增加 helper
- 解析 query 中 `tenantID`
- 从登录身份中拿到角色/可访问 tenant 信息（按当前 auth 数据结构最小实现）
- 平台管理员允许空 scope
- 租户管理员默认锁定到其所属 tenant

Step 2: 封装 query helper
- `applyTenantScope(...)`
- `requireTenantScopedResource(...)` 或等价辅助函数

Step 3: 运行 scope 相关测试

Run:
- `go test ./internal/http/... -run Phase3TenantScoped -v`

Expected:
- 仍可能部分 FAIL，但 helper 层测试通过或接口更接近通过

### Task 4: 对 settlement bills 增加 tenant-aware 列表/操作校验

Objective: 让 settlement bill 先成为 tenant-aware 的财务域样板。

Files:
- Modify: `backend/internal/http/router.go`
- Test: `backend/internal/http/router_test.go`

Step 1: 在 settlement bill 列表查询中接入 tenant scope
Step 2: 在 confirm/export/create 等关键操作中校验目标 bill 是否属于当前 tenant scope
Step 3: 审计日志补充 tenant 维度（若当前结构允许）
Step 4: 跑 settlement + phase3 scope 测试

Run:
- `go test ./internal/http/... -run 'Settlement|Phase3TenantScoped' -v`

Expected:
- PASS

### Task 5: 对 withdrawal requests 增加 tenant-aware 列表/审核校验

Objective: 提现域成为第二个 tenant-aware 样板。

Files:
- Modify: `backend/internal/http/router.go`
- Test: `backend/internal/http/router_test.go`

Step 1: 列表查询接入 tenant scope
Step 2: create/review 时校验 payload 或目标 request 的 tenant 归属
Step 3: 若仅有 `tenantCode`，先做 tenantCode -> tenant 归属校验，不一次性大改 schema
Step 4: 跑 withdrawal + scope 测试

Run:
- `go test ./internal/http/... -run 'Withdrawal|Phase3TenantScoped' -v`

Expected:
- PASS

### Task 6: 对 reports/risk 增加 tenant-aware 过滤

Objective: 把运营查询域接入平台上下文。

Files:
- Modify: `backend/internal/http/router.go`
- Test: `backend/internal/http/router_test.go`

Step 1: 报表接口支持 tenant scope 限定
Step 2: risk 列表/视图支持 tenant scope 限定
Step 3: 补充越权测试
Step 4: 运行相关测试

Run:
- `go test ./internal/http/... -run 'Report|Risk|Phase3TenantScoped' -v`

Expected:
- PASS

### Task 7: 跑后端全量测试并做回归修复

Objective: 确保第一里程碑的 tenant 改造不破坏 phase-1/2 已完成能力。

Files:
- Modify if needed: `backend/internal/http/router.go`
- Modify if needed: `backend/internal/http/router_test.go`

Step 1: 运行后端全量测试
Run:
- `go test ./...`

Step 2: 若失败，按最小修复原则回归
Step 3: 再次运行直到通过

Expected:
- 所有后端测试通过

### Task 8: 为前端新增平台上下文 provider 与 hook

Objective: 建立统一 active tenant / active brand 状态。

Files:
- Create: `frontend/src/platform-scope/PlatformScopeProvider.tsx`
- Create: `frontend/src/platform-scope/usePlatformScope.ts`
- Modify: `frontend/src/main.tsx`

Step 1: 创建 provider/context
Step 2: 提供 activeTenantId / activeBrandId 读写 API
Step 3: 在 `main.tsx` 包裹应用

Verification:
- `npm run build`
- 预期：build 通过

### Task 9: 在 AdminLayout 增加 tenant / brand selector

Objective: 让平台上下文可被用户操作。

Files:
- Modify: `frontend/src/layouts/AdminLayout.tsx`
- Modify: `frontend/src/lib/api.ts`
- Modify: `frontend/src/i18n/messages.ts`

Step 1: 调用现有 tenant/brand 列表接口
Step 2: 在 layout 顶部工具栏增加两个 Select
Step 3: tenant 变化时刷新/清空 brand
Step 4: 增加 i18n 文案

Verification:
- `npm run build`
- 手动确认：切换 tenant 后 brand 列表正确收敛

### Task 10: 在 reports / withdrawal / risk / tenant-brand 页面透传并展示 scope

Objective: 让关键 phase-3 页面感知平台上下文。

Files:
- Modify: `frontend/src/pages/reports/index.tsx`
- Modify: `frontend/src/pages/withdrawal-requests/index.tsx`
- Modify: `frontend/src/pages/risk/index.tsx`
- Modify: `frontend/src/pages/tenant-brand/index.tsx`
- Modify: `frontend/src/lib/api.ts`
- Modify: `frontend/src/i18n/messages.ts`

Step 1: 在 query key 中加入 tenantId/brandId
Step 2: 在支持的 API 请求中透传 tenantID/brandID
Step 3: hero/card 中展示当前 scope summary
Step 4: 页面无 scope 数据时显示合理空态/提示

Verification:
- `npm run build`
- 手动检查：页面刷新、切换 scope 时 query 触发重新获取

### Task 11: 跑前端 build 并做最小回归修复

Objective: 确保平台上下文接入不破坏现有前端。

Files:
- Modify if needed: touched frontend files above

Run:
- `npm run build`

Expected:
- PASS

### Task 12: 统一验收第一里程碑

Objective: 以 phase-3 第一里程碑目标做总控验收。

Files:
- No required code changes

Checklist:
- 后端 tenant scope 越权测试通过
- 后端全量测试通过
- 前端 build 通过
- AdminLayout 可切 tenant/brand
- 关键页面透传并展示 scope
- 平台管理员与租户管理员行为符合预期

---

## 5. Files Likely to Change

Backend
- `backend/internal/http/router.go`
- `backend/internal/http/router_test.go`
- 可能辅助查看：`backend/internal/domain/model/models.go`

Frontend
- `frontend/src/main.tsx`
- `frontend/src/layouts/AdminLayout.tsx`
- `frontend/src/lib/api.ts`
- `frontend/src/i18n/messages.ts`
- `frontend/src/pages/reports/index.tsx`
- `frontend/src/pages/withdrawal-requests/index.tsx`
- `frontend/src/pages/risk/index.tsx`
- `frontend/src/pages/tenant-brand/index.tsx`
- `frontend/src/platform-scope/PlatformScopeProvider.tsx`
- `frontend/src/platform-scope/usePlatformScope.ts`

---

## 6. Parallel Work Packages

可并行包 A：后端 tenant scope 基础
- Task 1-7
- 负责人：backend subagent

可并行包 B：前端平台上下文基础
- Task 8-11
- 负责人：frontend subagent

可并行包 C：总控终审与验收
- Task 12
- 负责人：planner/controller（我）

依赖关系
- A 与 B 可以并行启动
- B 中具体 API 透传以 A 暴露/确认的 query 参数为准
- 最终由总控统一跑测试/build 并验收

---

## 7. Risks / Tradeoffs / Open Questions

Risks
1. 当前后端不少表未显式 tenant_id，tenant scope 可能需要通过 agent/tenantCode 间接推导
2. auth 身份模型若没有明确 tenant 归属字段，需先用最小兼容方式补充测试身份
3. 若试图一次性把所有业务表 tenant 化，会扩大范围并拖慢第一里程碑

Tradeoffs
- 本里程碑优先“查询与关键财务操作 tenant-aware”，而不是一次性做全量 schema 重构
- brand scope 第一版主要做 UI/context 与可选透传，tenant scope 才是强约束主线

Open Questions
1. 当前租户管理员身份在后端 auth payload/DB 中的 tenant 归属字段是什么？若没有，需要最小补法
2. 哪些报表已经能通过现有数据安全地映射到 tenant？哪些需要先限制功能
3. 是否先把 withdrawal 的 `tenantCode` 保留并做映射校验，还是在本里程碑直接升级为 `tenantID`

建议答案
- 第一里程碑先保留 `tenantCode`，做映射与约束，避免 schema 扩动过大
- 先保证 tenant scope 正确，再考虑 brand 级强隔离与 schema 正规化

---

## 8. Verification Summary

Backend verification commands
- `cd /Users/leon/project/game-admin/backend && go test ./internal/http/... -run Phase3TenantScoped -v`
- `cd /Users/leon/project/game-admin/backend && go test ./...`

Frontend verification commands
- `cd /Users/leon/project/game-admin/frontend && npm run build`

Manual verification
- 登录平台管理员，能切换 tenant/brand 并查看对应数据
- 登录租户管理员，默认锁定本 tenant，尝试访问其他 tenant 返回拒绝或不可见
- 切换 tenant 后，reports / withdrawals / risk 页面数据刷新

---

## 9. Recommended Execution Handoff

按用户偏好的 controller/planner 模式执行：
1. 先由总控明确本 plan 的并行包和验收点
2. 并行委派 backend/frontend subagent 实现
3. 子 agent 负责具体实现和测试
4. 总控统一复查代码质量、再跑 `go test ./...` 与 `npm run build`
5. 若任一子任务失败，总控重新分配或亲自修复
