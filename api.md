**通用约定:**

*   **Base URL:** 假设 API 部署在`http://localhost:8080` (本地测试)。后续路径都是基于此 Base URL。
*   **数据格式:** 所有请求体和响应体都使用 JSON 格式 (`Content-Type: application/json`, `Accept: application/json`)。
*   **认证:**
    *   需要认证的接口，客户端需要在 HTTP 请求头中包含 `Authorization: Bearer <your_jwt_token>`。
    *   `User (JWT)`: 需要普通用户登录获取的 Token。
    *   `Admin (JWT)`: 需要管理员用户登录获取的 Token。
    *   `Public`: 公开接口，无需认证。
*   **标准响应格式 (成功):**
    ```json
    {
        "code": 0,         // 0 表示成功
        "message": "Success", // 成功消息
        "data": { ... }     // 实际返回的数据，可能是对象或数组，也可能为 null
    }
    ```
*   **标准响应格式 (失败):**
    ```json
    {
        "code": <error_code>, // 非 0 的错误码 (例如 400, 401, 403, 404, 500)
        "message": "<error_description>", // 错误描述信息
        "data": null
    }
    ```
*   **金额单位:** 除非特别说明，所有涉及金额的字段（如充值、余额、开票金额）都以 **分** 为单位。
*   **日期格式:** API 请求和响应中的日期字符串通常使用 `YYYY-MM-DD` 格式。

---

### 一、 用户管理 (User Management)

1.  **用户注册 (Register)**
    *   **Purpose:** 创建一个新的用户账户。
    *   **Method:** `POST`
    *   **Path:** `/register`
    *   **Authentication:** `Public`
    *   **Request Body:**
        ```json
        {
            "username": "newUser",   // string, required, unique
            "password": "password123", // string, required
            "email": "user@example.com" // string, required, unique
            // "is_admin": false // boolean, optional (通常注册时默认为 false, 或由特定逻辑控制)
        }
        ```
    *   **Response (Success - 201 Created):**
        ```json
        {
            "code": 0,
            "message": "用户注册成功",
            "data": {
                "user_id": 123 // 新创建用户的 ID
            }
        }
        ```
    *   **Error Responses:** `400 Bad Request` (输入无效，如用户名/邮箱已存在、密码太弱), `500 Internal Server Error`。

2.  **用户登录 (Login)**
    *   **Purpose:** 用户登录以获取认证 Token。
    *   **Method:** `POST`
    *   **Path:** `/login`
    *   **Authentication:** `Public`
    *   **Request Body:**
        ```json
        {
            "username": "newUser",   // string, required
            "password": "password123" // string, required
        }
        ```
    *   **Response (Success - 200 OK):**
        ```json
        {
            "code": 0,
            "message": "登录成功",
            "data": {
                "token": "<jwt_token_string>", // JWT Token
                "user_id": 123,
                "username": "newUser",
                "is_admin": false // 用户是否为管理员
            }
        }
        ```
    *   **Error Responses:** `400 Bad Request` (缺少字段), `401 Unauthorized` (用户名或密码错误), `500 Internal Server Error`。

---

### 二、 广告创意管理 (Advertisements)

1.  **提交广告创意 (Submit Ad)**
    *   **Purpose:** 广告主提交一个新的广告创意等待审核。
    *   **Method:** `POST`
    *   **Path:** `/ads`
    *   **Authentication:** `User (JWT)`
    *   **Request Body:**
        ```json
        {
            "title": "夏季特惠广告", // string, required
            "image_url": "http://example.com/ad_image.jpg", // string, required, URL
            "target_url": "http://advertiser.com/landing_page" // string, required, URL
        }
        ```
    *   **Response (Success - 201 Created):**
        ```json
        {
            "code": 0,
            "message": "广告创意提交成功，等待审核",
            "data": {
                "advertisement_id": 456 // 新创建广告创意的 ID
            }
        }
        ```
    *   **Error Responses:** `400 Bad Request` (输入无效), `401 Unauthorized`, `500 Internal Server Error`。

2.  **获取我的广告创意列表 (Get My Ads)**
    *   **Purpose:** 广告主查看自己提交的所有广告创意及其状态。
    *   **Method:** `GET`
    *   **Path:** `/my-ads`
    *   **Authentication:** `User (JWT)`
    *   **Query Parameters:**
        *   `status` (string, optional): 按状态过滤 (e.g., `Pending`, `Approved`, `Rejected`)。
    *   **Response (Success - 200 OK):**
        ```json
        {
            "code": 0,
            "message": "Success",
            "data": [
                {
                    "id": 456,
                    "user_id": 123,
                    "title": "夏季特惠广告",
                    "image_url": "http://example.com/ad_image.jpg",
                    "target_url": "http://advertiser.com/landing_page",
                    "status": "Pending", // "Pending", "Approved", "Rejected"
                    "review_notes": null, // 管理员审核备注
                    "created_at": "2023-10-27T10:00:00Z",
                    "updated_at": "2023-10-27T10:00:00Z"
                },
                // ... more ads
            ]
        }
        ```
    *   **Error Responses:** `401 Unauthorized`, `500 Internal Server Error`。

3.  **审核广告创意 (Admin Review Ad)**
    *   **Purpose:** 管理员审核广告创意（批准或拒绝）。
    *   **Method:** `PATCH`
    *   **Path:** `/ads/{id}/status`
    *   **Authentication:** `Admin (JWT)`
    *   **Path Parameters:**
        *   `id` (integer, required): 要审核的广告创意 ID。
    *   **Request Body:**
        ```json
        {
            "status": "Approved", // string, required, "Approved" or "Rejected"
            "review_notes": "广告内容合规" // string, optional, 审核备注
        }
        ```
    *   **Response (Success - 200 OK):**
        ```json
        {
            "code": 0,
            "message": "广告创意审核状态已更新",
            "data": null
        }
        ```
    *   **Error Responses:** `400 Bad Request` (无效状态), `401 Unauthorized`, `403 Forbidden` (非管理员), `404 Not Found` (广告不存在), `500 Internal Server Error`。

---

### 三、 广告活动管理 (Campaigns)

1.  **申请创建广告活动 (Request Campaign)**
    *   **Purpose:** 广告主基于已批准的广告创意申请创建一个广告活动。
    *   **Method:** `POST`
    *   **Path:** `/campaigns`
    *   **Authentication:** `User (JWT)`
    *   **Request Body:**
        ```json
        {
            "advertisement_id": 456, // integer, required, 必须是该用户已 Approved 的广告 ID
            "start_date": "2024-09-01", // string, required, YYYY-MM-DD
            "end_date": "2024-09-30", // string, required, YYYY-MM-DD
            "daily_budget": 5000 // integer, optional, 每日预算（单位：分），如果需要预算控制
        }
        ```
    *   **Response (Success - 201 Created):**
        ```json
        {
            "code": 0,
            "message": "广告活动申请提交成功，等待审核",
            "data": {
                "campaign_id": 789 // 新创建广告活动的 ID
            }
        }
        ```
    *   **Error Responses:** `400 Bad Request` (无效输入，如广告未批准、日期错误、广告不属于该用户), `401 Unauthorized`, `404 Not Found` (广告 ID 不存在), `500 Internal Server Error`。

2.  **获取我的广告活动列表 (Get My Campaigns)**
    *   **Purpose:** 广告主查看自己创建的所有广告活动及其状态。
    *   **Method:** `GET`
    *   **Path:** `/my-campaigns`
    *   **Authentication:** `User (JWT)`
    *   **Query Parameters:**
        *   `status` (string, optional): 按状态过滤 (e.g., `Pending`, `Approved`, `Active`, `Paused`, `Completed`, `Cancelled`, `Rejected`)。
    *   **Response (Success - 200 OK):**
        ```json
        {
            "code": 0,
            "message": "Success",
            "data": [
                {
                    "id": 789,
                    "user_id": 123,
                    "advertisement_id": 456,
                    "start_date": "2024-09-01T00:00:00Z",
                    "end_date": "2024-09-30T23:59:59Z",
                    "status": "Pending", // "Pending", "Approved", "Active", ...
                    "review_notes": null,
                    "created_at": "2023-10-27T11:00:00Z",
                    "updated_at": "2023-10-27T11:00:00Z"
                    // "daily_budget": 5000 // 如果有
                },
                // ... more campaigns
            ]
        }
        ```
    *   **Error Responses:** `401 Unauthorized`, `500 Internal Server Error`。

3.  **获取我的广告活动详情 (Get My Campaign Details)**
    *   **Purpose:** 广告主查看单个广告活动的详细信息。
    *   **Method:** `GET`
    *   **Path:** `/my-campaigns/{id}`
    *   **Authentication:** `User (JWT)`
    *   **Path Parameters:**
        *   `id` (integer, required): 要查看的广告活动 ID。
    *   **Response (Success - 200 OK):** (返回单个活动对象，结构同上列表中的元素)
    *   **Error Responses:** `401 Unauthorized`, `404 Not Found` (活动不存在或不属于该用户), `500 Internal Server Error`。

4.  **取消广告活动 (Cancel Campaign)**
    *   **Purpose:** 广告主取消一个尚未结束的广告活动。
    *   **Method:** `PATCH`
    *   **Path:** `/my-campaigns/{id}/cancel`
    *   **Authentication:** `User (JWT)`
    *   **Path Parameters:**
        *   `id` (integer, required): 要取消的广告活动 ID。
    *   **Request Body:** None
    *   **Response (Success - 200 OK):**
        ```json
        {
            "code": 0,
            "message": "广告活动已取消",
            "data": null
        }
        ```
    *   **Error Responses:** `400 Bad Request` (活动状态不允许取消，如已结束或已取消), `401 Unauthorized`, `404 Not Found` (活动不存在或不属于该用户), `500 Internal Server Error`。

5.  **审核广告活动 (Admin Review Campaign)**
    *   **Purpose:** 管理员审核广告活动申请（批准或拒绝）。
    *   **Method:** `PATCH`
    *   **Path:** `/campaigns/{id}/status`
    *   **Authentication:** `Admin (JWT)`
    *   **Path Parameters:**
        *   `id` (integer, required): 要审核的广告活动 ID。
    *   **Request Body:**
        ```json
        {
            "status": "Approved", // string, required, "Approved" or "Rejected"
            "review_notes": "活动设置合理" // string, optional, 审核备注
        }
        ```
    *   **Response (Success - 200 OK):**
        ```json
        {
            "code": 0,
            "message": "广告活动审核状态已更新",
            "data": null
        }
        ```
    *   **Error Responses:** `400 Bad Request` (无效状态), `401 Unauthorized`, `403 Forbidden`, `404 Not Found`, `500 Internal Server Error`。

---

### 四、 计费与财务 (Billing & Finance)

1.  **账户充值 (Recharge)**
    *   **Purpose:** 用户为账户充值（模拟，实际需对接支付网关）。
    *   **Method:** `POST`
    *   **Path:** `/recharge`
    *   **Authentication:** `User (JWT)`
    *   **Request Body:**
        ```json
        {
            "amount": 10000, // integer, required, 充值金额（单位：分）
            "payment_method": "Simulated" // string, optional, 支付方式
        }
        ```
    *   **Response (Success - 200 OK):** (模拟成功)
        ```json
        {
            "code": 0,
            "message": "充值成功",
            "data": {
                "transaction_id": "txn_123abc", // 交易 ID
                "new_balance": 15000 // 充值后的新余额（单位：分）
            }
        }
        ```
    *   **Error Responses:** `400 Bad Request` (金额无效), `401 Unauthorized`, `500 Internal Server Error` (模拟处理失败)。

2.  **查询账户余额 (Get Balance)**
    *   **Purpose:** 用户查询当前账户余额。
    *   **Method:** `GET`
    *   **Path:** `/balance`
    *   **Authentication:** `User (JWT)`
    *   **Response (Success - 200 OK):**
        ```json
        {
            "code": 0,
            "message": "Success",
            "data": {
                "balance": 15000 // 当前余额（单位：分）
            }
        }
        ```
    *   **Error Responses:** `401 Unauthorized`, `500 Internal Server Error`。

3.  **获取充值记录 (Get Recharge History)**
    *   **Purpose:** 用户查询自己的充值历史记录。
    *   **Method:** `GET`
    *   **Path:** `/recharges`
    *   **Authentication:** `User (JWT)`
    *   **Query Parameters:**
        *   `status` (string, optional): 按状态过滤 (e.g., `Pending`, `Success`, `Failed`)。
        *   `start_date` (string, optional): `YYYY-MM-DD`，按创建日期过滤。
        *   `end_date` (string, optional): `YYYY-MM-DD`，按创建日期过滤。
    *   **Response (Success - 200 OK):**
        ```json
        {
            "code": 0,
            "message": "Success",
            "data": [
                {
                    "id": 1,
                    "user_id": 123,
                    "amount": 10000,
                    "status": "Success",
                    "transaction_id": "txn_123abc",
                    "payment_method": "Simulated",
                    "created_at": "2023-10-27T12:00:00Z",
                    "updated_at": "2023-10-27T12:01:00Z"
                },
                // ... more transactions
            ]
        }
        ```
    *   **Error Responses:** `401 Unauthorized`, `500 Internal Server Error`。

4.  **申请开具发票 (Request Invoice)**
    *   **Purpose:** 用户针对指定时间段内的成功充值记录申请开具发票。
    *   **Method:** `POST`
    *   **Path:** `/invoices/request`
    *   **Authentication:** `User (JWT)`
    *   **Request Body:**
        ```json
        {
            "start_date": "2024-08-01", // string, required, 开票周期开始日期 YYYY-MM-DD
            "end_date": "2024-08-31",   // string, required, 开票周期结束日期 YYYY-MM-DD
            "billing_title": "客户公司名称", // string, required, 发票抬头
            "tax_id": "1234567890ABCDEF", // string, optional, 税号
            "billing_address": "详细邮寄地址或电子邮箱" // string, required
        }
        ```
    *   **Response (Success - 201 Created):**
        ```json
        {
            "code": 0,
            "message": "发票请求已提交成功",
            "data": {
                "invoice_request_id": 101 // 发票请求记录的 ID
            }
        }
        ```
    *   **Error Responses:** `400 Bad Request` (输入无效，日期错误，该时段无可开票金额), `401 Unauthorized`, `500 Internal Server Error` (计算金额或创建记录失败)。

5.  **获取我的发票请求历史 (Get My Invoices)**
    *   **Purpose:** 用户查看自己提交的发票请求记录。
    *   **Method:** `GET`
    *   **Path:** `/invoices`
    *   **Authentication:** `User (JWT)`
    *   **Query Parameters:**
        *   `status` (string, optional): 按状态过滤 (e.g., `Pending`, `Processing`, `Completed`, `Failed`)。
        *   `start_date` (string, optional): `YYYY-MM-DD`，按请求日期过滤。
        *   `end_date` (string, optional): `YYYY-MM-DD`，按请求日期过滤。
    *   **Response (Success - 200 OK):**
        ```json
        {
            "code": 0,
            "message": "Success",
            "data": [
                {
                    "id": 101,
                    "user_id": 123,
                    "status": "Pending",
                    "invoice_period_start": "2024-08-01T00:00:00Z",
                    "invoice_period_end": "2024-08-31T23:59:59Z",
                    "total_amount": 15050, // 开票总额（单位：分）
                    "billing_title": "客户公司名称",
                    "tax_id": "1234567890ABCDEF",
                    "billing_address": "详细邮寄地址或电子邮箱",
                    "invoice_number": null, // （模拟）发票号
                    "notes": null, // 处理备注
                    "requested_at": "2023-10-27T14:00:00Z",
                    "processed_at": null // 处理完成时间
                },
                // ... more invoice requests
            ]
        }
        ```
    *   **Error Responses:** `401 Unauthorized`, `500 Internal Server Error`。

6.  **获取我的发票请求详情 (Get My Invoice Details)**
    *   **Purpose:** 用户查看单个发票请求的详细信息。
    *   **Method:** `GET`
    *   **Path:** `/invoices/{id}`
    *   **Authentication:** `User (JWT)`
    *   **Path Parameters:**
        *   `id` (integer, required): 要查看的发票请求 ID。
    *   **Response (Success - 200 OK):** (返回单个发票请求对象，结构同上列表中的元素)
    *   **Error Responses:** `401 Unauthorized`, `404 Not Found` (请求不存在或不属于该用户), `500 Internal Server Error`。

7.  **更新发票请求状态 (Admin Update Invoice Status)** - 可选实现
    *   **Purpose:** 管理员更新发票请求的处理状态（模拟开票）。
    *   **Method:** `PATCH`
    *   **Path:** `/admin/invoices/{id}/status`
    *   **Authentication:** `Admin (JWT)`
    *   **Path Parameters:**
        *   `id` (integer, required): 要更新的发票请求 ID。
    *   **Request Body:**
        ```json
        {
            "status": "Completed", // string, required, 新状态 (e.g., "Processing", "Completed", "Failed")
            "invoice_number": "INV-2024-001", // string, optional, 发票号码
            "notes": "已开具并邮寄" // string, optional, 处理备注
        }
        ```
    *   **Response (Success - 200 OK):**
        ```json
        {
            "code": 0,
            "message": "发票请求状态已更新",
            "data": null
        }
        ```
    *   **Error Responses:** `400 Bad Request` (无效状态), `401 Unauthorized`, `403 Forbidden`, `404 Not Found`, `500 Internal Server Error`。

---

### 五、 广告投放与效果 (Ad Serving & Performance)

1.  **获取广告 (Get Ad)**
    *   **Purpose:** 广告位（如网站、App）调用以获取一个要展示的广告。
    *   **Method:** `GET`
    *   **Path:** `/get-ad`
    *   **Authentication:** `Public`
    *   **Response (Success - 200 OK, 有广告):**
        ```json
        {
            "code": 0,
            "message": "Success",
            "data": {
                "campaign_id": 789,        // 活动 ID (用于构建点击链接)
                "advertisement_id": 456,   // 广告创意 ID (用于构建点击链接)
                "title": "夏季特惠广告",
                "image_url": "http://example.com/ad_image.jpg",
                "target_url": "http://advertiser.com/landing_page" // 原始目标 URL (前端不用这个做点击链接)
            }
        }
        ```
    *   **Response (Success - 200 OK, 无广告):**
        ```json
        {
            "code": 0, // or a specific code like 1
            "message": "没有可用的广告",
            "data": null // or {}
        }
        ```
    *   **Notes:**
        *   此接口调用会记录一次 **Impression** 事件。
        *   前端需要使用返回的 `campaign_id` 和 `advertisement_id` 构建点击跟踪 URL，例如 `http://your-ad-server.com/ads/click/789/456`。
    *   **Error Responses:** `500 Internal Server Error` (选择广告或记录 Impression 时出错)。

2.  **广告点击跟踪 (Track Ad Click)**
    *   **Purpose:** 用户点击广告后访问此链接，用于记录点击事件并重定向到目标页。
    *   **Method:** `GET`
    *   **Path:** `/ads/click/{campaign_id}/{advertisement_id}`
    *   **Authentication:** `Public`
    *   **Path Parameters:**
        *   `campaign_id` (integer, required): 被点击广告的活动 ID。
        *   `advertisement_id` (integer, required): 被点击广告的创意 ID。
    *   **Response (Success):** **HTTP 302 Found**
        *   `Location` Header: `http://advertiser.com/landing_page` (广告的原始 `target_url`)。
    *   **Notes:**
        *   此接口调用会记录一次 **Click** 事件。
        *   浏览器会自动跟随 302 重定向到 `Location` 指定的 URL。
    *   **Error Responses:** `400 Bad Request` (ID 无效), `404 Not Found` (活动或广告不存在/不匹配), `500 Internal Server Error` (记录 Click 或获取 `target_url` 失败)。

3.  **获取我的广告效果数据 (Get My Performance)**
    *   **Purpose:** 广告主查询其广告活动的效果数据（展示、点击、CTR）。
    *   **Method:** `GET`
    *   **Path:** `/my-performance`
    *   **Authentication:** `User (JWT)`
    *   **Query Parameters:**
        *   `start_date` (string, optional): `YYYY-MM-DD`，按事件时间过滤。
        *   `end_date` (string, optional): `YYYY-MM-DD`，按事件时间过滤。
        *   `campaign_id` (integer, optional): 按特定广告活动过滤。
        *   `advertisement_id` (integer, optional): 按特定广告创意过滤（如果需要）。
    *   **Response (Success - 200 OK):**
        ```json
        {
            "code": 0,
            "message": "Success",
            "data": [
                {
                    "campaign_id": 789,
                    "campaign_name": "我的九月活动", // 需要 Join 查询获取
                    "advertisement_id": 456,
                    "ad_title": "夏季特惠广告", // 需要 Join 查询获取
                    "impressions": 10500, // 展示次数
                    "clicks": 210, // 点击次数
                    "ctr": 2.00 // 点击率 (%)，例如 (clicks / impressions) * 100
                },
                // ... performance summary for other campaign/ad combinations
            ]
        }
        ```
    *   **Notes:**
        *   如果没有指定日期范围，可能默认查询最近 7 天或 30 天。
        *   CTR (Click-Through Rate) 由后端计算。
    *   **Error Responses:** `400 Bad Request` (日期范围错误), `401 Unauthorized`, `500 Internal Server Error` (查询聚合数据失败)。

---

这份文档提供了该广告系统所有核心接口的详细说明，涵盖了用户管理、广告管理、活动管理、计费财务以及广告投放与效果跟踪等功能。