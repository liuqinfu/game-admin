# 游戏代理管理系统设计方案

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** 设计一个支持无限层级代理、按游戏维度配置结算与佣金规则的游戏代理管理系统框架，并输出系统架构图与Markdown文档。

**Architecture:** 采用领域驱动的分层设计，拆分为代理网络、游戏管理、结算佣金、资金台账、风控审计、运营后台等核心域；系统层采用管理后台 + API 网关 + 领域服务 + 异步结算任务 + 数据存储/缓存/消息总线的模式，兼顾高并发充值结算与复杂代理关系查询。

**Tech Stack:** Markdown 文档、Mermaid 架构图、领域建模、事件驱动结算。

---

### Task 1: 领域建模与业务能力设计

**Objective:** 明确代理、游戏、邀请关系、佣金、结算、比例配置、审计等核心业务对象与关键流程。

### Task 2: 技术架构与模块划分设计

**Objective:** 设计系统分层、核心服务边界、存储/缓存/消息机制，以及扩展能力。

### Task 3: 架构图与文档整合

**Objective:** 将业务架构与技术架构整合为一份可落地的 Markdown 设计文档，包含 Mermaid 架构图、核心流程、数据模型建议与演进建议。
