好的，根据您提供的系统架构分析以及我们之前的讨论，这是一份结合了前端实现的中文版项目 README：

---

# 在线广告投放管理平台

## 项目简介

本项目是一个基础的在线广告投放与管理平台，旨在提供一个核心的广告投放、跟踪、数据报告以及内容审核的功能。平台主要面向广告主（用于投放广告）和平台管理员（用于内容审核）。当前版本已实现用户注册登录、广告创意及活动申请/审核、简单的模拟充值及财务查询、发票申请、广告投放获取及点击跟踪、以及基础效果报告功能。

**核心特性:**

*   用户注册、登录、认证 (基于 JWT)
*   普通用户 (广告主) 和管理员角色的区分
*   广告创意 (Advertisement) 的提交、查看、以及管理员审核
*   广告活动 (Ad Campaign) 的申请、查看、取消、以及管理员审核
*   基于时间（开始/结束日期）的广告活动状态判断
*   广告主账户余额查询、模拟充值、充值历史查看
*   广告主发票申请、发票历史及详情查看
*   广告投放接口，随机获取可用广告
*   广告展示 (Impression) 和点击 (Click) 事件跟踪
*   广告活动及创意效果报告（展示、点击、CTR），支持按日期和活动筛选
*   管理员查看待审核的广告创意和广告活动列表

## 技术栈

本项目采用前后端分离的架构，主要技术栈如下：

*   **后端 (Backend):**
    *   语言: Go
    *   Web 框架/库: Go 标准库 `net/http` (使用 Go 1.22+ 的 Mux 特性)
    *   数据库: MySQL
    *   认证: JWT (JSON Web Tokens)
    *   数据交换格式: JSON
    *   依赖管理: Go Modules
    *   CORS: `github.com/rs/cors`
*   **前端 (Frontend):**
    *   框架: Vue 3 (Composition API, `<script setup>`)
    *   路由: Vue Router
    *   HTTP 客户端: Axios
    *   样式: CSS (scoped CSS)
    *   状态管理: 自定义 `useAuth` 等组合式函数
*   **数据库 (Database):**
    *   MySQL

## 系统架构设计

本系统采用经典的分层 Web 应用架构，旨在解耦各部分职责，提升可维护性。

**1. 核心组件与分层:**

*   **前端表示层 (Frontend Presentation Layer):**
    *   **职责:** 用户与系统交互的界面。负责渲染页面、处理用户输入、通过调用后端 API 获取/提交数据。包含路由管理、状态管理（如用户登录状态）。
    *   **实现:** 基于 Vue 3 的单页应用 (SPA)。包含多个 `.vue` 组件（如 `App.vue` 中的导航，以及各功能页面组件），使用 Vue Router 进行页面导航，Axios 与后端进行 API 调用。`useAuth` 等组合式函数管理前端用户认证状态。
*   **后端表示层 (Backend Presentation Layer - API Endpoints & Handlers):**
    *   **职责:** 接收来自客户端（前端、广告位）的 HTTP 请求，解析请求，进行基础输入验证。协调调用业务逻辑层处理请求。将结果格式化为 JSON 响应或执行重定向。
    *   **实现:** Go 的 `net/http` 包。`main.go` 配置路由 (Mux)，将请求导向不同的 Handler 函数（位于 `internal/handlers`）。`internal/webutil` 提供统一的 JSON 响应和错误处理。
*   **中间件 (Middleware):**
    *   **职责:** 处理跨越多 Handler 的通用逻辑，如认证、授权、CORS 处理等。
    *   **实现:** Go 函数包装 `http.Handler`。`internal/middleware` 包含 `AuthMiddleware` (验证 JWT, 注入用户信息) 和 `AdminMiddleware` (检查管理员权限)。`rs/cors` 处理跨域请求。
*   **业务逻辑/数据访问层 (Business Logic / Data Access Layer - Store):**
    *   **职责:** 封装与数据库交互的所有逻辑。执行 CRUD 操作，包含部分简单的业务计算和数据聚合（如计算总充值、查询效果汇总）。定义接口实现解耦。
    *   **实现:** `internal/store/store.go` 定义了 `Store` 接口和 `DBStore` 结构体（使用 `database/sql` 与 MySQL 交互）。
*   **数据模型层 (Data Model Layer):**
    *   **职责:** 定义应用程序中使用的结构化数据，通常对应数据库表或 API 数据的结构。
    *   **实现:** `internal/models/models.go` 中定义的各种 Go `struct`（`User`, `Advertisement`, `AdCampaign`, `AdEvent`, `InvoiceRequest`, `RechargeTransaction`）。
*   **数据存储层 (Data Persistence Layer):**
    *   **职责:** 持久化存储应用程序的所有数据。
    *   **实现:** MySQL 数据库。包含 `users`, `advertisements`, `ad_campaigns`, `recharge_transactions`, `invoice_requests`, `ad_events` 等表。

**2. 关键交互流程示例:**

*   **广告主查看广告效果流程:**
    1.  **广告主前端 (Vue App)** 用户访问“广告效果”页面。
    2.  **前端** 通过 Axios 发送带 JWT Token 的 `GET /my-performance` 请求到后端 API。
    3.  **后端 API Server (Mux)** 接收请求，经过 `AuthMiddleware` 验证 Token 并解析用户 ID。
    4.  **Mux** 将请求路由到 `GetAdPerformanceHandler`。
    5.  `GetAdPerformanceHandler` 从请求上下文中获取用户 ID，解析查询参数（如日期范围、活动 ID）。
    6.  `GetAdPerformanceHandler` 调用 **Store** 接口的 `GetAdPerformanceSummary(userID, params...)` 方法。
    7.  **DBStore** 实现执行 SQL 聚合查询，关联 `ad_events`, `ad_campaigns`, `advertisements` 表，按用户 ID、活动 ID、广告 ID 过滤和分组，计算展示、点击总数。
    8.  **MySQL** 返回查询结果到 **DBStore**。
    9.  **DBStore** 将结果返回给 `GetAdPerformanceHandler`。
    10. `GetAdPerformanceHandler` 计算每个条目的 CTR，并将数据格式化为 JSON 响应。
    11. **后端 API Server** 将 JSON 响应返回给 **前端**。
    12. **前端** 接收 JSON 数据，更新界面，展示广告效果列表。

*   **广告投放流程（简化）：**
    1.  **广告位 (外部网站/App)** 发送 `GET /get-ad` 请求。
    2.  **后端 API Server (Mux)** 路由到 `GetAdHandler` (此接口无需认证)。
    3.  `GetAdHandler` 调用 **Store** 接口的 `GetRandomActiveCampaign()` 方法。
    4.  **DBStore** 查询 **MySQL**，从当前 `Active` 状态且在有效期内的活动中随机选一个活动 ID。
    5.  **DBStore** 再根据活动 ID 获取关联的广告创意信息。
    6.  **DBStore** 将广告信息返回给 `GetAdHandler`。
    7.  `GetAdHandler` 调用 **Store** 接口的 `LogAdEvent` 方法，记录一条 `Impression` 事件，包含选中的活动 ID、广告 ID 及时间戳。
    8.  **DBStore** 将事件数据插入 **MySQL** 的 `ad_events` 表。
    9.  `GetAdHandler` 将广告创意信息（包含构建点击跟踪 URL 所需的 ID）格式化为 JSON 响应返回给 **广告位**。

**3. 架构图:**

```mermaid
graph TD
    subgraph Clients
        C1[Advertiser Front-end<br>(Vue SPA)]
        C2[Admin Front-end<br>(Vue Components)]
        C3[Ad Slot<br>(Website/App)]
    end

    subgraph "Backend API Server (Go Application)"
        A1[HTTP Mux / Router<br>(main.go)]
        A2[Middleware<br>(Auth, Admin, CORS)]
        A3[Handlers<br>(internal/handlers)]
        A4[Store Interface<br>(internal/store)]
        A5[DBStore Implementation<br>(internal/store)]
        A6[Models<br>(internal/models)]
        A7[Utils<br>(internal/webutil)]
    end

    subgraph "Data Persistence"
        DB[(MySQL Database)]
    end

    %% Client Interactions
    C1 -- "HTTP Request (with JWT)" --> A1
    C2 -- "HTTP Request (with JWT)" --> A1
    C3 -- "HTTP Request (GET /get-ad)" --> A1
    subgraph "User Click Flow"
        direction LR
        U1[End User Browser] -- "Click Ad Link<br>(GET /ads/click/...)" --> A1_Click(Ad Click Handler)
        A1_Click -- "HTTP 302 Redirect" --> U1
        U1 -- "Visit Target Page" --> AdvertiserSite[Advertiser Target Site]
    end
    A1_Click --> A1

    %% API Server Internal Flow
    A1 -- "Route Request" --> A2
    A2 -- "Process Middleware" --> A3
    A3 -- "Invoke Business Logic" --> A4
    A4 -- "Via Implementation" --> A5
    A5 -- "SQL Query/Execution" --> DB
    DB -- "Return Data" --> A5
    A5 -- "Return Processed Result" --> A3
    A3 -- "Use Data Models" --> A6
    A3 -- "Use Utils" --> A7
    A7 --> A3
    A3 -- "Format Response" --> A1
    A1 -- "HTTP Response (JSON / Redirect)" --> Clients
    A1 -- "HTTP Response (JSON / Redirect)" --> U1

    %% Log Event Flow
    A3 -- "Log Impression/Click" --> A4
    A4 --> A5
    A5 -- "INSERT INTO ad_events" --> DB

    style DB fill:#f9f,stroke:#333,stroke-width:2px
    style A1 fill:#bbf,stroke:#333,stroke-width:1px
    style A2 fill:#bbf,stroke:#333,stroke-width:1px
    style A3 fill:#bbf,stroke:#333,stroke-width:1px
    style A4 fill:#ccf,stroke:#333,stroke-width:1px,stroke-dasharray: 5 5
    style A5 fill:#bbf,stroke:#333,stroke-width:1px
    style A6 fill:#ddf,stroke:#333,stroke-width:1px
    style A7 fill:#eef,stroke:#333,stroke-width:1px
```

**图例说明:**

*   **Clients:** 代表与系统后端 API 交互的不同客户端应用。
*   **Backend API Server:** 系统的 Go 后端服务。
    *   **Mux/Router:** 请求路由器。
    *   **Middleware:** 中间件处理层。
    *   **Handlers:** 具体 API 端点处理逻辑。
    *   **Store Interface:** 数据访问接口层。
    *   **DBStore:** Store 接口的 MySQL 具体实现。
    *   **Models:** 数据结构定义。
    *   **Utils:** 通用工具函数（如 Web 响应格式化）。
*   **Data Persistence:** 数据库存储。
*   **箭头:** 表示请求流或数据流的方向。实线箭头表示主要的请求/响应，虚线箭头表示接口定义，虚线框表示接口的实现。

**4. 架构优点 (已实现):**

*   **清晰的分层:** 前后端分离，后端各层职责分明，易于理解和维护。
*   **模块化:** 不同功能（用户、广告、活动、财务、事件）相对独立。
*   **数据访问抽象:** `Store` 接口提供了一定的数据库实现解耦。
*   **无状态 API:** 使用 JWT 使后端服务无状态，便于水平扩展。
*   **简单直接:** 对于当前规模，架构不复杂，易于快速开发和部署。

**5. 架构局限与未来改进方向 (当前版本不足):**

*   **广告选择策略:** `ORDER BY RAND()` 在大数据量时性能问题；无法支持复杂的广告策略（如竞价、定向）。
*   **事件处理同步:** Impression 和 Click 事件同步写入数据库，在高并发下可能成为瓶颈。
*   **性能报告:** 直接在主数据库进行聚合查询，大数据量时性能堪忧，且报表功能基础。
*   **缺乏缓存:** 未使用缓存减轻数据库压力。
*   **单点数据库:** 只有一个 MySQL 实例，存在风险和扩展限制。
*   **配置管理:** 敏感配置（如数据库连接串、JWT 密钥）管理方式不够完善。
*   **安全性:** 需要更全面的安全审计和加固（如输入验证、API 限流）。
*   **未实现的功能:** 预算控制、复杂的定位、发票的管理员处理、用户密码修改/找回等功能尚未实现。

## 快速开始

（此部分为概览，详细步骤请参考具体代码目录下的说明，如后端 `README.md`，前端 `README.md` 等）

1.  **环境准备:** 安装 Go 环境, Node.js 环境 (含 npm/yarn), MySQL 数据库。
2.  **克隆项目:** `git clone <项目仓库地址>`
3.  **数据库设置:**
    *   创建 MySQL 数据库。
    *   执行数据库迁移脚本（如果提供）。
    *   配置后端代码中的数据库连接信息。
4.  **后端启动:**
    *   进入后端代码目录。
    *   运行 `go run main.go`。
5.  **前端启动:**
    *   进入前端代码目录。
    *   安装依赖: `npm install` 或 `yarn install`。
    *   启动开发服务器: `npm run dev` 或 `yarn dev`。
6.  访问前端地址 (通常是 `http://localhost:5173`) 开始使用。

## API 端点概览

以下是后端已实现的部分主要 API 端点：

*   **公开接口:**
    *   `POST /register`: 用户注册
    *   `POST /login`: 用户登录
    *   `GET /get-ad`: 获取随机广告用于展示 (记录 Impression)
    *   `GET /ads/click/{campaign_id}/{advertisement_id}`: 广告点击跟踪并重定向 (记录 Click)
*   **需要认证（广告主）接口:**
    *   `POST /ads`: 提交广告创意
    *   `GET /my-ads`: 查看我的广告创意列表
    *   `POST /campaigns`: 申请广告活动
    *   `GET /my-campaigns`: 查看我的广告活动列表
    *   `GET /my-campaigns/{id}`: 查看我的广告活动详情
    *   `PATCH /my-campaigns/{id}/cancel`: 取消我的广告活动
    *   `POST /recharge`: 模拟充值
    *   `GET /balance`: 查询我的账户余额
    *   `GET /recharges`: 查看我的充值历史
    *   `POST /invoices/request`: 申请发票
    *   `GET /invoices`: 查看我的发票申请历史
    *   `GET /invoices/{id}`: 查看我的发票申请详情
    *   `GET /my-performance`: 查看我的广告效果报告
*   **需要管理员认证接口:**
    *   `GET /admin/ads/pending`: 查看待审核广告创意列表
    *   `GET /admin/campaigns/pending`: 查看待审核广告活动列表
    *   `PATCH /ads/{id}/status`: 审核广告创意（更新状态）
    *   `PATCH /campaigns/{id}/status`: 审核广告活动（更新状态）

## 未来改进方向

*   实现预算设置、消耗和扣费逻辑。
*   引入更复杂的广告定向能力。
*   优化广告投放策略，支持更高级的算法。
*   将同步的事件记录改为异步处理（如使用消息队列）。
*   优化效果报告的存储和查询，可能引入专门的分析层。
*   增加缓存机制提升性能。
*   完善管理员后台功能，包括用户管理、更全面的平台数据统计和发票处理。
*   集成真实的支付网关。
*   实现用户密码修改、重置等账户管理功能。
*   编写自动化测试（单元测试、集成测试）。

## 许可证

（待定，可选择 MIT 或其他开源许可证）

---