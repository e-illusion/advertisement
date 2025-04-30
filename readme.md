### 系统架构设计分析

这个简单的广告系统采用了一个经典的分层 Web 应用架构。其核心目标是解耦不同部分的职责，使得系统更易于开发、维护和扩展。

**1. 核心组件与分层:**

*   **表示层 (Presentation Layer - API Endpoints & Handlers):**
    *   **职责:** 接收来自客户端（广告主前端、管理员前端、广告位）的 HTTP 请求，解析请求参数（路径参数、查询参数、请求体），验证输入（基础格式验证）。调用业务逻辑层（Store 层）处理请求。将处理结果（数据或错误）格式化为 JSON 响应返回给客户端。
    *   **实现:** Go 的 `net/http` 包 (`http.ServeMux`, `http.HandlerFunc`)，以及我们编写的各种 `Handler` 函数（如 `RegisterHandler`, `SubmitAdHandler`, `GetAdHandler`, `AdClickHandler`, `GetAdPerformanceHandler` 等）。`webutil` 提供了统一的 JSON 响应和错误处理。
*   **中间件 (Middleware):**
    *   **职责:** 处理横切关注点（Cross-Cutting Concerns），这些功能需要在多个 Handler 执行前后应用。主要包括：
        *   **认证 (Authentication):** 验证 JWT Token，解析用户信息并注入到请求上下文中 (`middleware.AuthMiddleware`)。
        *   **授权 (Authorization):** 检查用户是否具有特定权限（如管理员权限） (`middleware.AdminMiddleware`)。
        *   **日志记录 (Logging):** （虽然我们主要在 Handler 和 Store 中记录）可以在中间件层面记录所有请求的入口和出口信息。
        *   **CORS 处理:** （如果前端部署在不同域）允许跨域请求。
    *   **实现:** 使用标准的 Go HTTP 中间件模式（函数包装 `http.Handler`）。
*   **业务逻辑/数据访问层 (Business Logic / Data Access Layer - Store):**
    *   **职责:** 封装与数据库交互的所有逻辑。执行 CRUD (Create, Read, Update, Delete) 操作。包含一些简单的业务计算（如计算可开票金额 `GetSuccessfulRechargeTotalInRange`，查询汇总数据 `GetAdPerformanceSummary`）。定义了一个 `Store` 接口，提供了与具体数据库实现的解耦。
    *   **实现:** `internal/store/store.go` 定义了 `Store` 接口和 `DBStore` 结构体（及其方法），使用 `database/sql` 包与 MySQL 数据库交互。
*   **数据模型层 (Data Model Layer):**
    *   **职责:** 定义应用程序中使用的数据结构，这些结构通常映射到数据库表或 API 请求/响应的载荷 (Payload)。
    *   **实现:** `internal/models/models.go` 中定义的各种 Go `struct`（如 `User`, `Advertisement`, `AdCampaign`, `AdEvent`, `InvoiceRequest`, `RechargeTransaction` 等）。
*   **数据存储层 (Data Persistence Layer):**
    *   **职责:** 持久化存储应用程序的所有数据。
    *   **实现:** MySQL 数据库。包含了 `users`, `advertisements`, `ad_campaigns`, `recharge_transactions`, `invoice_requests`, `ad_events` 等表。

**2. 关键交互流程:**

*   **广告投放流程:**
    1.  **广告位 (Client)** 调用 `GET /get-ad`。
    2.  **API Server (Mux)** 将请求路由到 `GetAdHandler`。
    3.  `GetAdHandler` 调用 **Store** (`GetRandomActiveCampaign`, `GetAdvertisementByID`) 获取一个符合条件的广告。
    4.  **Store** 查询 **MySQL** 数据库。
    5.  `GetAdHandler` 调用 **Store** (`LogAdEvent`) 记录一次 **Impression** 事件。
    6.  **Store** 将事件写入 **MySQL** 的 `ad_events` 表。
    7.  `GetAdHandler` 将广告信息（图片 URL、标题、**点击跟踪 URL 的构建信息**）返回给广告位。
*   **广告点击流程:**
    1.  **用户** 点击广告位上构造好的 **点击跟踪 URL** (`GET /ads/click/{cid}/{aid}`).
    2.  **API Server (Mux)** 路由到 `AdClickHandler`。
    3.  `AdClickHandler` 调用 **Store** (`GetCampaignAndAdInfo`) 查询活动和广告信息（包括原始 `target_url`）。
    4.  **Store** 查询 **MySQL**。
    5.  `AdClickHandler` 调用 **Store** (`LogAdEvent`) 记录一次 **Click** 事件。
    6.  **Store** 将事件写入 **MySQL** 的 `ad_events` 表。
    7.  `AdClickHandler` 返回 **HTTP 302 重定向** 响应，`Location` 指向广告的 `target_url`。
    8.  **用户浏览器** 跟随重定向访问广告主的目标页面。
*   **广告主查看效果流程:**
    1.  **广告主前端 (Client)** 发送带 Token 的请求 `GET /my-performance?start_date=...`。
    2.  **API Server (Mux + Auth Middleware)** 验证 Token 并路由到 `GetAdPerformanceHandler`。
    3.  `GetAdPerformanceHandler` 解析查询参数，调用 **Store** (`GetAdPerformanceSummary`)。
    4.  **Store** 执行 **MySQL** 聚合查询 (JOIN `ad_events`, `ad_campaigns`, `advertisements`)。
    5.  `GetAdPerformanceHandler` 计算 CTR，将汇总数据返回给广告主前端。

**3. 技术栈:**

*   **后端语言:** Go
*   **Web 框架/库:** Go 标准库 `net/http`
*   **数据库:** MySQL
*   **认证:** JWT (JSON Web Tokens)
*   **数据交换格式:** JSON

**4. 架构优点:**

*   **清晰的分层:** 各层职责分明，易于理解和维护。
*   **模块化:** 不同功能（用户、广告、活动、计费、事件）相对独立。
*   **可测试性:** `Store` 接口的存在使得 Handler 层可以进行单元测试（通过 Mock Store）。
*   **简单直接:** 对于当前规模的功能，架构不复杂，易于快速开发和部署。
*   **无状态 API:** 使用 JWT 使得 API 服务本身是无状态的，便于水平扩展。

**5. 架构局限与潜在改进点:**

*   **广告选择策略简单:** `GetRandomActiveCampaign` 中的 `ORDER BY RAND()` 在数据量大时性能低下，且无法实现复杂的定向、竞价或预算控制。
*   **事件处理同步:** Impression 和 Click 事件是同步写入数据库的。在高并发下，这可能成为瓶颈，并且影响主请求（尤其是 `GetAdHandler`）的响应时间。可以考虑使用消息队列（如 Kafka, RabbitMQ）异步处理事件。
*   **性能报告简单:** `GetAdPerformanceSummary` 直接在主数据库上进行聚合查询。对于大数据量和复杂报表，可能需要引入数据仓库或专门的分析数据库 (OLAP)。
*   **缺乏缓存:** 对于常用数据（如活动信息、用户信息）没有使用缓存（如 Redis, Memcached），可能导致数据库压力增大。
*   **单点数据库:** 只有一个 MySQL 实例，存在单点故障风险，且扩展性有限。可以考虑读写分离、分库分表等策略。
*   **配置管理:** 数据库连接信息、JWT 密钥等硬编码或简单处理，实际项目中应使用配置文件或环境变量管理。
*   **安全性:** 需要更全面的安全考虑，如输入验证（防 SQL 注入、XSS）、更强的密码策略、API 限流、日志审计等。
*   **部署:** 未涉及容器化 (Docker)、编排 (Kubernetes) 和 CI/CD 流程。

### 架构设计图

下面是一个表示该广告系统架构的简化图：

```mermaid
graph TD
    subgraph Clients
        C1[广告主前端 (Web App)]
        C2[管理员前端 (Web App)]
        C3[广告位 (网站/App)]
    end

    subgraph "API Server (Go Application)"
        A1[HTTP Mux / Router]
        A2[Middleware (Auth, Admin)]
        A3[Handlers (API Endpoints)]
        A4[Store Interface]
        A5[DBStore (MySQL Implementation)]
        A6[Models (Data Structures)]
    end

    subgraph "Data Persistence"
        DB[(MySQL Database)]
    end

    %% Client Interactions
    C1 -- "1. 管理请求 (带JWT)" --> A1
    C2 -- "2. 管理请求 (带JWT)" --> A1
    C3 -- "3. 获取广告请求 (GET /get-ad)" --> A1
    subgraph "User Click Flow"
        direction LR
        U1[User Browser] -- "4. 点击广告链接" --> A1_Click(GET /ads/click/...)
        A1_Click -- "6. HTTP 302 Redirect" --> U1
        U1 -- "7. 访问目标页" --> AdvertiserSite[Advertiser Target Site]
    end
    A1_Click --> A1

    %% API Server Internal Flow
    A1 -- "路由请求" --> A2
    A2 -- "处理中间件 (认证/授权)" --> A3
    A3 -- "调用业务逻辑" --> A4
    A4 -- "通过具体实现" --> A5
    A5 -- "SQL 查询/执行" --> DB
    DB -- "返回数据" --> A5
    A5 -- "返回处理结果" --> A3
    A3 -- "使用数据模型" --> A6
    A3 -- "格式化响应" --> A1
    A1 -- "HTTP 响应 (JSON / Redirect)" --> Clients
    A1 -- "HTTP 响应 (JSON / Redirect)" --> U1

    %% Ad Click Handler specifically interacts with DBStore to log event and get target URL
    %% Note: Mermaid doesn't easily show the Handler calling Store *and* Store calling DB,
    %% so the A3->A4->A5->DB flow represents the general pattern.

    %% Implicit data flow for Click Handler redirect
    %% AdClickHandler (in A3) gets target_url via A4/A5/DB, then sends redirect via A1

    %% Log Event Flow (simplified) - Handlers like GetAdHandler & AdClickHandler trigger this
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
```

**图例说明:**

*   **Clients:** 代表与系统交互的不同用户端或系统。
*   **API Server (Go Application):** 系统的核心后端服务。
    *   **Mux/Router:** 接收 HTTP 请求并分发给对应的 Handler。
    *   **Middleware:** 处理认证、授权等通用逻辑。
    *   **Handlers:** 处理具体的业务请求。
    *   **Store Interface:** 定义数据访问操作的契约。
    *   **DBStore:** `Store` 接口的具体数据库实现。
    *   **Models:** 定义数据结构。
*   **Data Persistence:** 数据库存储。
*   **箭头:** 表示请求流或数据流的方向。
    *   实线箭头表示主要的请求/响应流程。
    *   用户点击流程单独展示了重定向步骤。
    *   日志事件流也示意性地标出。

这个分析和架构图应该能很好地总结我们所构建的简单广告系统。