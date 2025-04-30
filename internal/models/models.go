package models

import (
	"time"
)

// Advertisement 代表广告数据模型
type Advertisement struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	ImageURL  string `json:"image_url"`
	TargetURL string `json:"target_url"`
	UserID    int    `json:"user_id"` // <-- 新增: 关联的用户 ID
	Status    string `json:"status"`  // <-- 新增: 广告状态 (Pending, Approved, Rejected)
}

// UserCredentials 代表用户登录/注册时使用的凭证结构
type UserCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// --- 新增：User 代表从数据库获取的用户信息 ---
type User struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"` // <-- 新增 Role 字段
	Balance		 int64  	`json:"balance"`
}

// AdCampaign 代表一个广告活动请求或实例
type AdCampaign struct {
	ID             int       `json:"id"`
	AdvertisementID int       `json:"advertisement_id"`
	UserID         int       `json:"user_id"`
	StartDate      time.Time `json:"start_date"` // 使用 time.Time 处理日期
	EndDate        time.Time `json:"end_date"`   // 使用 time.Time 处理日期
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	// 可以选择性地嵌入关联的 Advertisement 信息，如果 API 需要返回
	// Advertisement *Advertisement `json:"advertisement,omitempty"`
}

// --- 新增：RechargeTransaction 代表充值记录 ---
type RechargeTransaction struct {
	ID             int64     `json:"id"`
	UserID         int       `json:"user_id"`
	Amount         int64     `json:"amount"`         // 单位：分
	Status         string    `json:"status"`
	TransactionID  *string   `json:"transaction_id"` // 使用指针，因为可能为 NULL
	PaymentMethod  string    `json:"payment_method"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// --- 新增：RechargeRequest 用于接收充值请求 ---
type RechargeRequest struct {
	Amount float64 `json:"amount"` // 用户输入金额，可能带小数（如 10.50 元）
}

// --- 新增：BalanceResponse 用于返回余额 ---
type BalanceResponse struct {
    Balance float64 `json:"balance"` // 返回给用户的余额，单位是元
    Currency string `json:"currency"` // e.g., "CNY"
}

// --- 用于请求活动的数据结构 ---
// （也可以直接在 Handler 中定义）
type CampaignRequestData struct {
    AdvertisementID int    `json:"advertisement_id"`
    StartDate       string `json:"start_date"` // 接收 "YYYY-MM-DD" 格式字符串
    EndDate         string `json:"end_date"`   // 接收 "YYYY-MM-DD" 格式字符串
}

// --- 用于审核活动的数据结构 ---
type CampaignReviewData struct {
    Status string `json:"status"` // "Approved" or "Rejected"
}

// RechargeHistoryFilters 封装充值历史记录的过滤条件
type RechargeHistoryFilters struct {
	StartDate *time.Time // 使用指针，如果未提供则为 nil
	EndDate   *time.Time
	MinAmount *int64     // 单位：分
	MaxAmount *int64     // 单位：分
	Status    *string    // 使用指针，允许空状态
}


// CampaignFilters 封装广告活动列表的过滤条件
type CampaignFilters struct {
	StartDate *time.Time // 使用指针，如果未提供则为 nil
	EndDate   *time.Time
	Status    *string    // 使用指针，允许空状态
    // AdvertisementID *int // 可选：按关联的广告创意 ID 过滤
}

// CampaignWithAdDetails 用于在列表中返回活动及其关联的广告创意简要信息
type CampaignWithAdDetails struct {
    ID             int       `json:"id"`
    AdvertisementID int       `json:"advertisement_id"`
    UserID         int       `json:"user_id"` // 通常在用户自己的列表里可以省略
    StartDate      time.Time `json:"start_date"`
    EndDate        time.Time `json:"end_date"`
    Status         string    `json:"status"`
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`

    // 关联的广告信息 (可以只包含部分字段)
    AdTitle    string `json:"ad_title"`
    AdImageURL string `json:"ad_image_url"`
    // AdTargetURL string `json:"ad_target_url"` // 根据需要添加
}


// AdEvent 用于记录单个广告事件
type AdEvent struct {
    ID              int64     `json:"id"`
    EventType       string    `json:"event_type"` // "Impression" or "Click"
    AdvertisementID int       `json:"advertisement_id"`
    CampaignID      int       `json:"campaign_id"`
    UserID          int       `json:"user_id"`
    EventTimestamp  time.Time `json:"event_timestamp"`
}

// AdPerformanceFilter 用于查询广告效果的过滤条件
type AdPerformanceFilter struct {
    StartDate *time.Time // 基于 event_timestamp 过滤
    EndDate   *time.Time
    CampaignID *int       // 可选：按特定活动过滤
    // AdvertisementID *int // 可选：按特定创意过滤 (如果需要更细粒度)
}

// AdPerformanceSummary 返回给用户的广告效果汇总数据
type AdPerformanceSummary struct {
    CampaignID      int     `json:"campaign_id"`
    CampaignName    string  `json:"campaign_name"` // 需要 Join 获取
    AdvertisementID int     `json:"advertisement_id"`
    AdTitle         string  `json:"ad_title"`      // 需要 Join 获取
    Impressions     int64   `json:"impressions"`
    Clicks          int64   `json:"clicks"`
    CTR             float64 `json:"ctr"` // Click-Through Rate (%)
}

// InvoiceRequest 对应数据库中的发票请求记录
type InvoiceRequest struct {
	ID                 int64      `json:"id"`
	UserID             int        `json:"user_id"`
	Status             string     `json:"status"`
	InvoicePeriodStart time.Time  `json:"invoice_period_start"` // 使用 time.Time 更灵活
	InvoicePeriodEnd   time.Time  `json:"invoice_period_end"`
	TotalAmount        int64      `json:"total_amount"` // 单位：分
	BillingTitle       string     `json:"billing_title"`
	TaxID              *string    `json:"tax_id"` // 指针，允许为空
	BillingAddress     string     `json:"billing_address"`
	InvoiceNumber      *string    `json:"invoice_number"` // 指针，允许为空
	Notes              *string    `json:"notes"`          // 指针，允许为空
	RequestedAt        time.Time  `json:"requested_at"`
	ProcessedAt        *time.Time `json:"processed_at"` // 指针，允许为空
}

// InvoiceRequestPayload 用户提交发票请求时的请求体结构
type InvoiceRequestPayload struct {
	StartDate      string `json:"start_date"` // "YYYY-MM-DD"
	EndDate        string `json:"end_date"`   // "YYYY-MM-DD"
	BillingTitle   string `json:"billing_title"`
	TaxID          string `json:"tax_id"` // 前端可能传空字符串
	BillingAddress string `json:"billing_address"`
}

// InvoiceRequestFilter 用于过滤发票请求历史 (可选)
type InvoiceRequestFilter struct {
    Status    *string
    StartDate *time.Time // 按请求日期过滤
    EndDate   *time.Time // 按请求日期过滤
}
