package store

import (
	"context" // Store 方法应该接受 context
	"database/sql"
	"errors" // 用于自定义错误类型
	"fmt"    // 用于错误包装
	"log"    // 临时用于记录
	"strings"
	"time"

	// 需要导入 models 包
	"advertisement/internal/models" // 替换 "your_module_name"
	// 导入 mysql 驱动可能需要用于错误类型断言 (可选，如果需要特定错误处理)
	// "github.com/go-sql-driver/mysql"
)

// --- 自定义错误类型 ---
// 使用自定义错误类型或哨兵错误 (sentinel errors) 可以让上层调用者更容易地判断错误原因
var (
	ErrNotFound      = errors.New("store: resource not found")
	ErrDuplicateUser = errors.New("store: username already exists")
	// 可以添加更多自定义错误...
)


// --- Store 接口定义了所有数据库操作 ---
type Store interface {
	// 用户相关
	CreateUser(ctx context.Context, username string, passwordHash string) error
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)

	// 广告相关
	CreateAdvertisement(ctx context.Context, ad *models.Advertisement) (int64, error)
	GetAdvertisementsByUserID(ctx context.Context, userID int) ([]models.Advertisement, error)
	// GetRandomApprovedAd(ctx context.Context) (*models.Advertisement, error)
	UpdateAdvertisementStatus(ctx context.Context, adID int, status string) error // <-- 新增接口方法
    GetAdvertisementByID(ctx context.Context, adID int) (*models.Advertisement, error) // <-- (可选但有用) 增加一个按ID获取广告的方法，供更新前检查

	// --- 新增广告活动相关方法 ---
	CreateAdCampaign(ctx context.Context, campaign *models.AdCampaign) (int64, error)
	UpdateAdCampaignStatus(ctx context.Context, campaignID int, status string) error
	GetAdCampaignByID(ctx context.Context, campaignID int) (*models.AdCampaign, error)
	// GetAdCampaignsByUserID(ctx context.Context, userID int) ([]models.AdCampaign, error) // 可选
	// GetPendingAdCampaigns(ctx) ([]models.AdCampaign, error) // 可选
  
	// --- 替换 GetRandomApprovedAd ---
	GetRandomActiveCampaignAd(ctx context.Context) (*models.Advertisement, error) // 返回活动广告的创意信息

	// --- 新增充值和余额相关方法 ---
	// CreateRechargeTransaction 在数据库中创建一条新的充值记录 (初始状态 Pending)
    // 返回新创建记录的 ID
	CreateRechargeTransaction(ctx context.Context, userID int, amountInCents int64, paymentMethod string) (int64, error)

    // ProcessSuccessfulRecharge 原子性地增加用户余额并更新充值记录状态为 Success
    // 这个方法必须在数据库事务中执行所需操作
    ProcessSuccessfulRecharge(ctx context.Context, userID int, amountInCents int64, rechargeRecordID int64, simulatedTxID string) error

    // UpdateRechargeTransactionStatus 更新特定充值记录的状态和模拟交易ID (例如更新为 Failed)
    UpdateRechargeTransactionStatus(ctx context.Context, rechargeRecordID int64, status string, simulatedTxID string) error

    // GetUserBalance 获取指定用户的余额 (单位：分)
	GetUserBalance(ctx context.Context, userID int) (int64, error)

	GetUserRechargeHistory(ctx context.Context, userID int, filters models.RechargeHistoryFilters) ([]models.RechargeTransaction, error)

 	// --- 广告活动管理 ---
    // GetAdCampaignsByUserID 获取指定用户的广告活动列表，支持过滤，并包含广告创意信息
    GetAdCampaignsByUserID(ctx context.Context, userID int, filters models.CampaignFilters) ([]models.CampaignWithAdDetails, error)

    // GetAdCampaignByIDAndUser 获取用户拥有的单个广告活动的详细信息 (包含广告创意信息)
    GetAdCampaignByIDAndUser(ctx context.Context, campaignID int, userID int) (*models.CampaignWithAdDetails, error)

    // UpdateAdCampaignStatusByUser 用户更新自己广告活动的状态 (例如取消)
    // 返回错误如果活动不属于该用户，或状态不允许更改
    UpdateAdCampaignStatusByUser(ctx context.Context, campaignID int, userID int, newStatus string) error

    // GetAdCampaignByID(ctx context.Context, campaignID int) (*models.AdCampaign, error) // 可能仍然需要给管理员用
    // UpdateAdCampaignStatus(ctx context.Context, campaignID int, status string) error // 管理员更新状态的方法


	 // --- 广告事件与效果 ---
    // LogAdEvent 记录一个广告事件 (Impression 或 Click)
    LogAdEvent(ctx context.Context, event models.AdEvent) error

    // GetAdPerformanceSummary 查询广告效果汇总数据
    GetAdPerformanceSummary(ctx context.Context, userID int, filters models.AdPerformanceFilter) ([]models.AdPerformanceSummary, error)
	GetRandomActiveCampaign(ctx context.Context) (*models.AdCampaign, error)

    // --- 发票相关方法 ---
    // GetSuccessfulRechargeTotalInRange 计算指定用户在日期范围内成功充值的总额（分）
    GetSuccessfulRechargeTotalInRange(ctx context.Context, userID int, startDate, endDate time.Time) (int64, error)

    // CreateInvoiceRequest 创建一个新的发票请求记录
    CreateInvoiceRequest(ctx context.Context, req models.InvoiceRequest) (int64, error)

    // GetInvoiceRequestsByUserID 获取用户的发票请求历史，支持过滤
    GetInvoiceRequestsByUserID(ctx context.Context, userID int, filters models.InvoiceRequestFilter) ([]models.InvoiceRequest, error)

    // GetInvoiceRequestByIDAndUser 获取用户拥有的单个发票请求详情
    GetInvoiceRequestByIDAndUser(ctx context.Context, invoiceID int64, userID int) (*models.InvoiceRequest, error)

    // --- (管理员功能，可选) ---
    // UpdateInvoiceRequestStatus 更新发票请求的状态和可选的发票号/备注 (需要权限控制)
    // UpdateInvoiceRequestStatus(ctx context.Context, invoiceID int64, status string, invoiceNumber *string, notes *string, processedAt *time.Time) error
}


// --- DBStore 是 Store 接口的数据库实现 ---
type DBStore struct {
	db *sql.DB // 持有数据库连接池
}

// NewDBStore 创建一个新的 DBStore 实例
func NewDBStore(db *sql.DB) *DBStore {
	return &DBStore{db: db}
}

// --- 实现 Store 接口的方法 ---

// CreateUser 在数据库中创建一个新用户
func (s *DBStore) CreateUser(ctx context.Context, username string, passwordHash string) error {
	query := "INSERT INTO users (username, password_hash) VALUES (?, ?)"
	_, err := s.db.ExecContext(ctx, query, username, passwordHash)
	if err != nil {
		// 检查是否是唯一约束冲突错误 (用户名重复)
		// 注意：这种检查可能依赖于具体的数据库驱动错误实现
		// 一个更通用的方法是检查 SQLState 或错误消息字符串
		// 这里用字符串包含作为示例，实际中可能需要更健壮的方法
		if strings.Contains(err.Error(), "Duplicate entry") { // 针对 MySQL 的示例
			return ErrDuplicateUser // 返回自定义错误
		}
		// 对于其他错误，包装一下以提供更多上下文
		return fmt.Errorf("store: failed to create user: %w", err)
	}
	return nil
}

// GetUserByUsername 从数据库中按用户名查找用户
func (s *DBStore) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	user := &models.User{} // 创建一个 User 结构体指针用于接收数据
	query := "SELECT id, username, password_hash, role FROM users WHERE username = ?"
	err := s.db.QueryRowContext(ctx, query, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// 如果没有找到用户，返回我们自定义的 ErrNotFound
			return nil, ErrNotFound
		}
		// 对于其他数据库错误
		return nil, fmt.Errorf("store: failed to get user by username %s: %w", username, err)
	}
	return user, nil
}

// CreateAdvertisement 在数据库中创建一个新广告
func (s *DBStore) CreateAdvertisement(ctx context.Context, ad *models.Advertisement) (int64, error) {
	query := `
		INSERT INTO advertisements (title, image_url, target_url, user_id, status)
		VALUES (?, ?, ?, ?, ?)
	`
	result, err := s.db.ExecContext(ctx, query,
		ad.Title,
		ad.ImageURL,
		ad.TargetURL,
		ad.UserID, // 需要确保调用前 ad.UserID 已设置
		ad.Status, // 需要确保调用前 ad.Status 已设置 (例如 'Pending')
	)
	if err != nil {
		// 可以检查外键约束错误等
		return 0, fmt.Errorf("store: failed to create advertisement: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		// 即使获取 ID 失败，记录已插入，但仍需报告错误
		return 0, fmt.Errorf("store: failed to get last insert ID for advertisement: %w", err)
	}
	return id, nil
}

// GetAdvertisementsByUserID 获取指定用户的所有广告
func (s *DBStore) GetAdvertisementsByUserID(ctx context.Context, userID int) ([]models.Advertisement, error) {
	query := `
		SELECT id, title, image_url, target_url, user_id, status
		FROM advertisements
		WHERE user_id = ?
		ORDER BY id DESC
	`
	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("store: failed to query advertisements for user %d: %w", userID, err)
	}
	defer rows.Close() // 非常重要！

	var ads []models.Advertisement
	for rows.Next() {
		var ad models.Advertisement
		// 注意 Scan 的参数要和 SELECT 的列对应，包括 user_id
		err := rows.Scan(&ad.ID, &ad.Title, &ad.ImageURL, &ad.TargetURL, &ad.UserID, &ad.Status)
		if err != nil {
			// 单行扫描失败，记录日志并返回错误
			log.Printf("store: failed to scan advertisement row: %v", err)
			return nil, fmt.Errorf("store: failed to process advertisement list: %w", err)
		}
		ads = append(ads, ad)
	}

	// 检查遍历过程中的错误
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("store: error iterating advertisement rows: %w", err)
	}

	// 如果没有广告，返回空的切片和 nil 错误
	return ads, nil
}

// // GetRandomApprovedAd 获取一个随机的已批准广告
// // 注意：简单的 LIMIT 1 并不是真正的随机，但与之前行为一致。
// // 真正的随机可以使用 ORDER BY RAND() (MySQL) 或 TABLESAMPLE (PostgreSQL) 等，但性能较低。
// func (s *DBStore) GetRandomApprovedAd(ctx context.Context) (*models.Advertisement, error) {
// 	ad := &models.Advertisement{}
// 	query := `
// 		SELECT id, title, image_url, target_url, user_id, status
// 		FROM advertisements
// 		WHERE status = 'Approved'
// 		LIMIT 1
// 	`
// 	// 这里仍然使用 QueryRowContext，因为它期望最多一行
// 	err := s.db.QueryRowContext(ctx, query).Scan(&ad.ID, &ad.Title, &ad.ImageURL, &ad.TargetURL, &ad.UserID, &ad.Status)
// 	if err != nil {
// 		if errors.Is(err, sql.ErrNoRows) {
// 			// 没有找到已批准的广告
// 			return nil, ErrNotFound
// 		}
// 		return nil, fmt.Errorf("store: failed to get approved ad: %w", err)
// 	}
// 	return ad, nil
// }

// ... DBStore struct, NewDBStore, CreateUser, GetUserByUsername, CreateAdvertisement, GetAdvertisementsByUserID, GetRandomApprovedAd ...

// (可选) GetAdvertisementByID 根据 ID 获取广告信息
func (s *DBStore) GetAdvertisementByID(ctx context.Context, adID int) (*models.Advertisement, error) {
    ad := &models.Advertisement{}
    query := `SELECT id, title, image_url, target_url, user_id, status FROM advertisements WHERE id = ?`
    err := s.db.QueryRowContext(ctx, query, adID).Scan(&ad.ID, &ad.Title, &ad.ImageURL, &ad.TargetURL, &ad.UserID, &ad.Status)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, ErrNotFound // 使用自定义的未找到错误
        }
        return nil, fmt.Errorf("store: failed to get advertisement by id %d: %w", adID, err)
    }
    return ad, nil
}


// UpdateAdvertisementStatus 更新指定广告的状态
func (s *DBStore) UpdateAdvertisementStatus(ctx context.Context, adID int, status string) error {
	query := "UPDATE advertisements SET status = ? WHERE id = ?"
	result, err := s.db.ExecContext(ctx, query, status, adID)
	if err != nil {
		return fmt.Errorf("store: failed to update status for advertisement %d: %w", adID, err)
	}

	// 检查是否有行受到影响，如果没有，说明该 ID 的广告不存在
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		// 即使更新成功，获取影响行数失败也应记录，但不一定需要返回错误给用户
        log.Printf("store: failed to get rows affected after updating ad %d: %v", adID, err)
        // return fmt.Errorf("store: failed to check rows affected for advertisement %d: %w", adID, err)
        // 决定是否返回错误，通常不影响主要逻辑可以不返回
        return nil // 假设更新操作本身成功即可
	}

	if rowsAffected == 0 {
        // 没有行被更新，意味着广告 ID 不存在
		return ErrNotFound // 返回未找到错误
	}

	log.Printf("store: 广告 %d 状态更新为 %s", adID, status)
	return nil
}

func (s *DBStore) CreateAdCampaign(ctx context.Context, campaign *models.AdCampaign) (int64, error) {
    query := `
        INSERT INTO ad_campaigns (advertisement_id, user_id, start_date, end_date, status)
        VALUES (?, ?, ?, ?, ?)
    `
    result, err := s.db.ExecContext(ctx, query,
        campaign.AdvertisementID,
        campaign.UserID,
        campaign.StartDate, // time.Time 会被驱动正确处理
        campaign.EndDate,
        campaign.Status,    // 应为 'Pending'
    )
    if err != nil {
        // 检查外键错误等
        return 0, fmt.Errorf("store: failed to create ad campaign: %w", err)
    }
    id, err := result.LastInsertId()
    if err != nil {
        return 0, fmt.Errorf("store: failed to get last insert ID for ad campaign: %w", err)
    }
    log.Printf("store: 创建广告活动成功, ID: %d", id)
    return id, nil
}

func (s *DBStore) GetAdCampaignByID(ctx context.Context, campaignID int) (*models.AdCampaign, error) {
    campaign := &models.AdCampaign{}
    query := `
        SELECT id, advertisement_id, user_id, start_date, end_date, status, created_at, updated_at
        FROM ad_campaigns
        WHERE id = ?
    `
    err := s.db.QueryRowContext(ctx, query, campaignID).Scan(
        &campaign.ID,
        &campaign.AdvertisementID,
        &campaign.UserID,
        &campaign.StartDate,
        &campaign.EndDate,
        &campaign.Status,
        &campaign.CreatedAt,
        &campaign.UpdatedAt,
    )
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, ErrNotFound
        }
        return nil, fmt.Errorf("store: failed to get ad campaign by id %d: %w", campaignID, err)
    }
    return campaign, nil
}


func (s *DBStore) UpdateAdCampaignStatus(ctx context.Context, campaignID int, status string) error {
    query := "UPDATE ad_campaigns SET status = ? WHERE id = ?"
    result, err := s.db.ExecContext(ctx, query, status, campaignID)
    if err != nil {
        return fmt.Errorf("store: failed to update status for ad campaign %d: %w", campaignID, err)
    }
    rowsAffected, err := result.RowsAffected()
    if err != nil {
        log.Printf("store: failed to get rows affected after updating campaign %d: %v", campaignID, err)
        return nil // 通常不阻塞主逻辑
    }
    if rowsAffected == 0 {
        return ErrNotFound // ID 不存在
    }
    log.Printf("store: 广告活动 %d 状态更新为 %s", campaignID, status)
    return nil
}


// --- 实现新的广告获取逻辑 ---
func (s *DBStore) GetRandomActiveCampaignAd(ctx context.Context) (*models.Advertisement, error) {
    ad := &models.Advertisement{}
    // 查询状态为 'Approved' 且当前日期在活动有效期内的活动，
    // 并关联 advertisements 表获取广告创意信息。
    // 使用 CURDATE() 或 NOW()::DATE (取决于数据库) 获取当前日期。
    // 使用 ORDER BY RAND() LIMIT 1 获取随机一条。
    query := `
        SELECT
            adv.id, adv.title, adv.image_url, adv.target_url, adv.user_id, adv.status
        FROM ad_campaigns camp
        JOIN advertisements adv ON camp.advertisement_id = adv.id
        WHERE
            camp.status = 'Approved'
            AND CURDATE() >= camp.start_date  -- MySQL 使用 CURDATE()
            AND CURDATE() <= camp.end_date
        ORDER BY RAND()
        LIMIT 1
    `
    // 注意：如果你的 start_date/end_date 是 DATETIME/TIMESTAMP，比较时应使用 NOW()

    err := s.db.QueryRowContext(ctx, query).Scan(
        &ad.ID,
        &ad.Title,
        &ad.ImageURL,
        &ad.TargetURL,
        &ad.UserID, // 这将是广告创作者的 ID，不一定是活动请求者的 ID（虽然通常是同一个）
        &ad.Status, // 这将是广告创意的状态 ('Approved')
    )

    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            // 没有找到当前可用的活动广告
            return nil, ErrNotFound
        }
        // 其他数据库错误
        return nil, fmt.Errorf("store: failed to get active campaign ad: %w", err)
    }
    return ad, nil
}

// --- 实现充值和余额方法 ---

func (s *DBStore) CreateRechargeTransaction(ctx context.Context, userID int, amountInCents int64, paymentMethod string) (int64, error) {
    query := `
        INSERT INTO recharge_transactions (user_id, amount, status, payment_method)
        VALUES (?, ?, 'Pending', ?)
    `
    result, err := s.db.ExecContext(ctx, query, userID, amountInCents, paymentMethod)
    if err != nil {
        // 考虑外键约束等错误
        return 0, fmt.Errorf("store: failed to create recharge transaction record: %w", err)
    }
    id, err := result.LastInsertId()
    if err != nil {
        return 0, fmt.Errorf("store: failed to get last insert ID for recharge transaction: %w", err)
    }
    log.Printf("store: 创建充值记录成功, ID: %d, UserID: %d, Amount: %d分", id, userID, amountInCents)
    return id, nil
}


// ProcessSuccessfulRecharge **原子性地**更新余额和交易状态
func (s *DBStore) ProcessSuccessfulRecharge(ctx context.Context, userID int, amountInCents int64, rechargeRecordID int64, simulatedTxID string) error {
    // 1. 开始数据库事务
    tx, err := s.db.BeginTx(ctx, nil) // 使用默认隔离级别
    if err != nil {
        return fmt.Errorf("store: failed to begin transaction for recharge: %w", err)
    }
    // 使用 defer 配合 Rollback，确保事务在出错时回滚
    // 如果后面 Commit 成功，Rollback() 调用是无操作的 (no-op)
    defer tx.Rollback()

    // 2. 更新用户余额 (在事务中)
    updateBalanceQuery := "UPDATE users SET balance = balance + ? WHERE id = ?"
    _, err = tx.ExecContext(ctx, updateBalanceQuery, amountInCents, userID)
    if err != nil {
        return fmt.Errorf("store: failed to update user %d balance in transaction: %w", userID, err)
    }
    log.Printf("store: [TX] 用户 %d 余额增加 %d 分", userID, amountInCents)

    // 3. 更新充值记录状态为 Success (在事务中)
    updateTxQuery := `
        UPDATE recharge_transactions
        SET status = 'Success', transaction_id = ?, updated_at = ?
        WHERE id = ? AND status = 'Pending' -- 确保只更新 Pending 状态的记录
    `
    result, err := tx.ExecContext(ctx, updateTxQuery, simulatedTxID, time.Now(), rechargeRecordID)
    if err != nil {
        return fmt.Errorf("store: failed to update recharge transaction %d status in transaction: %w", rechargeRecordID, err)
    }

    // 检查更新充值记录是否真的影响了一行，防止重复处理或处理错误 ID
    rowsAffected, _ := result.RowsAffected() // 忽略获取影响行数的错误，主要依赖前面的错误检查
    if rowsAffected == 0 {
         // 可能是记录不存在，或者状态不是 Pending
        return fmt.Errorf("store: recharge transaction %d not found or not in Pending state during update", rechargeRecordID)
    }

    log.Printf("store: [TX] 充值记录 %d 状态更新为 Success, TxID: %s", rechargeRecordID, simulatedTxID)

    // 4. 提交事务
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("store: failed to commit recharge transaction: %w", err)
    }

    log.Printf("store: 充值事务成功提交 (UserID: %d, Amount: %d分, RecordID: %d)", userID, amountInCents, rechargeRecordID)
    return nil // 事务成功
}

func (s *DBStore) UpdateRechargeTransactionStatus(ctx context.Context, rechargeRecordID int64, status string, simulatedTxID string) error {
    query := `
        UPDATE recharge_transactions
        SET status = ?, transaction_id = ?, updated_at = ?
        WHERE id = ?
    `
    // 注意：如果 simulatedTxID 为空字符串，数据库会存储空字符串而不是 NULL。
    // 如果需要存 NULL，需要传递 sql.NullString 或 *string(nil)
    var txIDArg interface{} // 使用 interface{} 以便可以传递 nil
    if simulatedTxID != "" {
        txIDArg = simulatedTxID
    } else {
        txIDArg = nil // 传递 nil 会被驱动理解为 NULL
    }

    result, err := s.db.ExecContext(ctx, query, status, txIDArg, time.Now(), rechargeRecordID)
    if err != nil {
        return fmt.Errorf("store: failed to update status for recharge transaction %d: %w", rechargeRecordID, err)
    }
    rowsAffected, _ := result.RowsAffected()
    if rowsAffected == 0 {
        return ErrNotFound // 记录不存在
    }
    log.Printf("store: 充值记录 %d 状态更新为 %s (TxID: %v)", rechargeRecordID, status, txIDArg)
    return nil
}


func (s *DBStore) GetUserBalance(ctx context.Context, userID int) (int64, error) {
    var balance int64
    query := "SELECT balance FROM users WHERE id = ?"
    err := s.db.QueryRowContext(ctx, query, userID).Scan(&balance)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            // 理论上用户应该存在，否则无法通过认证，返回 ErrNotFound 或 0 都可以
            return 0, ErrNotFound // 或者 log.Printf 并返回 0, nil
        }
        return 0, fmt.Errorf("store: failed to get balance for user %d: %w", userID, err)
    }
    return balance, nil
}

func (s *DBStore) GetUserRechargeHistory(ctx context.Context, userID int, filters models.RechargeHistoryFilters) ([]models.RechargeTransaction, error) {
	// 基础查询语句
	baseQuery := `
        SELECT id, user_id, amount, status, transaction_id, payment_method, created_at, updated_at
        FROM recharge_transactions
    `
	// 条件子句和参数列表
	conditions := []string{"user_id = ?"} // 始终按用户 ID 过滤
	args := []interface{}{userID}         // 参数列表，第一个总是 user_id

	// 根据过滤器动态添加条件和参数
	if filters.Status != nil {
		conditions = append(conditions, "status = ?")
		args = append(args, *filters.Status)
	}
	if filters.StartDate != nil {
		// 使用 >= 比较 timestamp
		conditions = append(conditions, "created_at >= ?")
		args = append(args, *filters.StartDate) // time.Time 会被驱动正确处理
	}
	if filters.EndDate != nil {
		// 使用 <= 比较 timestamp
		conditions = append(conditions, "created_at <= ?")
		args = append(args, *filters.EndDate) // time.Time 会被驱动正确处理
	}
	if filters.MinAmount != nil {
		conditions = append(conditions, "amount >= ?")
		args = append(args, *filters.MinAmount) // int64
	}
	if filters.MaxAmount != nil {
		conditions = append(conditions, "amount <= ?")
		args = append(args, *filters.MaxAmount) // int64
	}

	// 组合最终的查询语句
	finalQuery := baseQuery + " WHERE " + strings.Join(conditions, " AND ") + " ORDER BY created_at DESC"

	log.Printf("Executing filtered recharge history query: %s with args: %v", finalQuery, args) // 调试日志

	// 执行查询
	rows, err := s.db.QueryContext(ctx, finalQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: failed to query filtered recharge history for user %d: %w", userID, err)
	}
	defer rows.Close()

	// 处理结果
	var history []models.RechargeTransaction
	for rows.Next() {
		var tx models.RechargeTransaction
		var nullableTxID sql.NullString // 用于接收可能为 NULL 的 transaction_id
		err := rows.Scan(
			&tx.ID,
			&tx.UserID,
			&tx.Amount, // 读取的是分 (int64)
			&tx.Status,
			&nullableTxID, // Scan 到 nullable 类型
			&tx.PaymentMethod,
			&tx.CreatedAt,
			&tx.UpdatedAt,
		)
		if err != nil {
			log.Printf("store: failed to scan filtered recharge transaction row: %v", err)
			// 在循环中遇到扫描错误，通常表明数据有问题或结构不匹配，最好返回错误
			return nil, fmt.Errorf("store: error processing filtered recharge history row: %w", err)
		}

		// 处理 nullable transaction_id
		if nullableTxID.Valid {
			tx.TransactionID = &nullableTxID.String // 如果数据库值不是 NULL，将其赋给指针
		} else {
			tx.TransactionID = nil // 否则确保模型中的指针为 nil
		}

		history = append(history, tx) // 将成功扫描的记录添加到结果列表
	} // 结束 rows.Next() 循环

	// 检查循环结束后是否有错误发生（例如数据库连接中断）
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("store: error iterating over filtered recharge history rows: %w", err)
	}

	// 如果没有错误，返回查询到的历史记录
	return history, nil
}

func (s *DBStore) GetAdCampaignsByUserID(ctx context.Context, userID int, filters models.CampaignFilters) ([]models.CampaignWithAdDetails, error) {
    // 基础查询语句，JOIN advertisements 表
    baseQuery := `
        SELECT
            camp.id, camp.advertisement_id, camp.user_id, camp.start_date, camp.end_date,
            camp.status, camp.created_at, camp.updated_at,
            adv.title AS ad_title, adv.image_url AS ad_image_url
        FROM ad_campaigns camp
        JOIN advertisements adv ON camp.advertisement_id = adv.id
    `
    // 条件子句和参数列表
    conditions := []string{"camp.user_id = ?"} // 始终按用户 ID 过滤
    args := []interface{}{userID}

    // 根据过滤器动态添加条件和参数 (注意表别名 camp)
    if filters.Status != nil {
        conditions = append(conditions, "camp.status = ?")
        args = append(args, *filters.Status)
    }
    if filters.StartDate != nil {
        conditions = append(conditions, "camp.start_date >= ?") // 活动本身的开始日期
        args = append(args, *filters.StartDate)
    }
    if filters.EndDate != nil {
        conditions = append(conditions, "camp.end_date <= ?")   // 活动本身的结束日期
        args = append(args, *filters.EndDate)
    }
    // if filters.AdvertisementID != nil {
    //     conditions = append(conditions, "camp.advertisement_id = ?")
    //     args = append(args, *filters.AdvertisementID)
    // }

    // 组合最终的查询语句
    finalQuery := baseQuery + " WHERE " + strings.Join(conditions, " AND ") + " ORDER BY camp.created_at DESC"

    log.Printf("Executing filtered user campaigns query: %s with args: %v", finalQuery, args)

    rows, err := s.db.QueryContext(ctx, finalQuery, args...)
    if err != nil {
        return nil, fmt.Errorf("store: failed to query campaigns for user %d: %w", userID, err)
    }
    defer rows.Close()

    var campaigns []models.CampaignWithAdDetails
    for rows.Next() {
        var camp models.CampaignWithAdDetails
        err := rows.Scan(
            &camp.ID, &camp.AdvertisementID, &camp.UserID, &camp.StartDate, &camp.EndDate,
            &camp.Status, &camp.CreatedAt, &camp.UpdatedAt,
            &camp.AdTitle, &camp.AdImageURL, // Scan 广告信息
        )
        if err != nil {
            log.Printf("store: failed to scan campaign row for user %d: %v", userID, err)
            return nil, fmt.Errorf("store: error processing campaign row: %w", err)
        }
        campaigns = append(campaigns, camp)
    }

    if err = rows.Err(); err != nil {
        return nil, fmt.Errorf("store: error iterating over campaign rows for user %d: %w", userID, err)
    }

    return campaigns, nil
}

func (s *DBStore) GetAdCampaignByIDAndUser(ctx context.Context, campaignID int, userID int) (*models.CampaignWithAdDetails, error) {
	query := `
        SELECT
            camp.id, camp.advertisement_id, camp.user_id, camp.start_date, camp.end_date,
            camp.status, camp.created_at, camp.updated_at,
            adv.title AS ad_title, adv.image_url AS ad_image_url
        FROM ad_campaigns camp
        JOIN advertisements adv ON camp.advertisement_id = adv.id
        WHERE camp.id = ? AND camp.user_id = ? -- Filter by both campaign ID and user ID
    `
	log.Printf("Executing get campaign by ID and user query: ID=%d, UserID=%d", campaignID, userID)

	var camp models.CampaignWithAdDetails
	err := s.db.QueryRowContext(ctx, query, campaignID, userID).Scan(
		&camp.ID, &camp.AdvertisementID, &camp.UserID, &camp.StartDate, &camp.EndDate,
		&camp.Status, &camp.CreatedAt, &camp.UpdatedAt,
		&camp.AdTitle, &camp.AdImageURL,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// 没有找到匹配的记录，可能是 ID 不存在，或者不属于该用户
			log.Printf("Campaign not found or access denied: ID=%d, UserID=%d", campaignID, userID)
			return nil, ErrNotFound // 使用通用未找到错误
		}
		// 其他数据库错误
		log.Printf("Error querying campaign by ID %d for user %d: %v", campaignID, userID, err)
		return nil, fmt.Errorf("store: failed to get campaign %d for user %d: %w", campaignID, userID, err)
	}

	return &camp, nil
}

func (s *DBStore) UpdateAdCampaignStatusByUser(ctx context.Context, campaignID int, userID int, newStatus string) error {
	// 1. (可选但推荐) 先检查活动是否存在且属于该用户，以及当前状态是否允许修改
    // 这样可以提供更具体的错误信息，但会增加一次查询
	/*
	currentCamp, err := s.GetAdCampaignByIDAndUser(ctx, campaignID, userID)
	if err != nil {
		return err // 会返回 ErrNotFound 或其他数据库错误
	}
	// 在这里添加状态转换逻辑检查，例如只能从 Pending/Approved -> Cancelled
	if currentCamp.Status != "Pending" && currentCamp.Status != "Approved" {
        return fmt.Errorf("store: cannot change status from '%s' to '%s'", currentCamp.Status, newStatus) // 或者定义一个更具体的错误类型
	}
    // 如果是取消操作，还可以检查活动是否已经开始
    // if newStatus == "Cancelled" && time.Now().After(currentCamp.StartDate) {
    //    return fmt.Errorf("store: cannot cancel a campaign that has already started")
    // }
	*/

	// 2. 执行更新，直接在 WHERE 子句中包含 userID 和 campaignID
	query := "UPDATE ad_campaigns SET status = ?, updated_at = ? WHERE id = ? AND user_id = ?"
    // (可选) 增加状态检查: "UPDATE ... WHERE id = ? AND user_id = ? AND status IN ('Pending', 'Approved')"
    // 如果希望严格限制只能从特定状态更新，可以像上面这样在 WHERE 中加入状态检查

	result, err := s.db.ExecContext(ctx, query, newStatus, time.Now(), campaignID, userID)
	if err != nil {
		log.Printf("Error updating campaign %d status by user %d: %v", campaignID, userID, err)
		return fmt.Errorf("store: failed to update status for campaign %d by user %d: %w", campaignID, userID, err)
	}

	// 3. 检查是否真的更新了行
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("store: failed to get rows affected after updating campaign %d by user %d: %v", campaignID, userID, err)
		// 即使获取影响行数失败，也可能已经更新成功，所以不一定返回错误
		return nil
	}

	if rowsAffected == 0 {
		// 没有行被更新，原因可能是：
		// 1. 活动 ID 不存在
		// 2. 活动不属于该用户
        // 3. (如果加了状态检查) 活动当前状态不允许被更新
        // 由于无法区分具体原因，返回 ErrNotFound 比较通用
		log.Printf("No rows affected when user %d tried to update campaign %d to status %s", userID, campaignID, newStatus)
		return ErrNotFound // 或者一个更具体的权限错误
	}

	log.Printf("User %d successfully updated campaign %d status to %s", userID, campaignID, newStatus)
	return nil
}

// --- 实现广告事件与效果方法 ---

func (s *DBStore) LogAdEvent(ctx context.Context, event models.AdEvent) error {
    query := `
        INSERT INTO ad_events (event_type, advertisement_id, campaign_id, user_id, event_timestamp)
        VALUES (?, ?, ?, ?, ?)
    `
    _, err := s.db.ExecContext(ctx, query,
        event.EventType,
        event.AdvertisementID,
        event.CampaignID,
        event.UserID,
        event.EventTimestamp, // 应该设为 time.Now() 或从调用者传入
    )
    if err != nil {
        log.Printf("Error logging ad event (%s) for user %d, campaign %d, ad %d: %v",
            event.EventType, event.UserID, event.CampaignID, event.AdvertisementID, err)
        return fmt.Errorf("store: failed to log ad event: %w", err)
    }
    // 不需要返回 ID，成功即可
    // log.Printf("Logged ad event: %s for campaign %d", event.EventType, event.CampaignID) // DEBUG Log
    return nil
}


func (s *DBStore) GetAdPerformanceSummary(ctx context.Context, userID int, filters models.AdPerformanceFilter) ([]models.AdPerformanceSummary, error) {
    // 基础聚合查询，JOIN campaigns 和 advertisements 获取名称
    baseQuery := `
        SELECT
            evt.campaign_id,
            camp.name AS campaign_name,
            evt.advertisement_id,
            adv.title AS ad_title,
            SUM(CASE WHEN evt.event_type = 'Impression' THEN 1 ELSE 0 END) AS impressions,
            SUM(CASE WHEN evt.event_type = 'Click' THEN 1 ELSE 0 END) AS clicks
        FROM ad_events evt
        JOIN ad_campaigns camp ON evt.campaign_id = camp.id
        JOIN advertisements adv ON evt.advertisement_id = adv.id
    `
    conditions := []string{"evt.user_id = ?"} // 必须按用户过滤
    args := []interface{}{userID}

    // 添加过滤条件
    if filters.StartDate != nil {
        conditions = append(conditions, "evt.event_timestamp >= ?")
        args = append(args, *filters.StartDate)
    }
    if filters.EndDate != nil {
        endOfDay := filters.EndDate.AddDate(0, 0, 1).Add(-1 * time.Nanosecond) // 包含当天
        conditions = append(conditions, "evt.event_timestamp <= ?")
        args = append(args, endOfDay)
    }
    if filters.CampaignID != nil {
        conditions = append(conditions, "evt.campaign_id = ?")
        args = append(args, *filters.CampaignID)
    }
    // if filters.AdvertisementID != nil { ... } // 如果需要按创意过滤

    // 组合查询
    finalQuery := baseQuery + " WHERE " + strings.Join(conditions, " AND ") +
                  " GROUP BY evt.campaign_id, camp.name, evt.advertisement_id, adv.title" +
                  " ORDER BY evt.campaign_id, evt.advertisement_id" // 按活动和创意排序

    log.Printf("Executing ad performance summary query for user %d: %s with args: %v", userID, finalQuery, args)

    rows, err := s.db.QueryContext(ctx, finalQuery, args...)
    if err != nil {
        return nil, fmt.Errorf("store: failed to query ad performance for user %d: %w", userID, err)
    }
    defer rows.Close()

    var results []models.AdPerformanceSummary
    for rows.Next() {
        var summary models.AdPerformanceSummary
        err := rows.Scan(
            &summary.CampaignID,
            &summary.CampaignName,
            &summary.AdvertisementID,
            &summary.AdTitle,
            &summary.Impressions,
            &summary.Clicks,
        )
        if err != nil {
            log.Printf("store: failed to scan ad performance row for user %d: %v", userID, err)
            return nil, fmt.Errorf("store: error processing ad performance row: %w", err)
        }
        // CTR 计算将在 Handler 中进行
        results = append(results, summary)
    }

    if err = rows.Err(); err != nil {
        return nil, fmt.Errorf("store: error iterating over ad performance rows for user %d: %w", userID, err)
    }

    return results, nil
}

func (s *DBStore) GetRandomActiveCampaign(ctx context.Context) (*models.AdCampaign, error) {
    query := `
        SELECT id, advertisement_id, user_id, start_date, end_date, status, created_at, updated_at
        FROM ad_campaigns
        WHERE status = 'Active' -- 或者 'Approved' 并且在 start_date 和 end_date 之间
          AND start_date <= NOW()
          AND end_date >= NOW()
        ORDER BY RAND() -- 注意：RAND() 在大数据量下性能不佳，但对于示例可以接受
        LIMIT 1
    `
    var camp models.AdCampaign
    err := s.db.QueryRowContext(ctx, query).Scan(
         &camp.ID, &camp.AdvertisementID, &camp.UserID, &camp.StartDate, &camp.EndDate,
         &camp.Status, &camp.CreatedAt, &camp.UpdatedAt,
    )
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, ErrNotFound
        }
        log.Printf("Error getting random active campaign: %v", err)
        return nil, fmt.Errorf("store: failed to get active campaign: %w", err)
    }
    return &camp, nil
}

// --- 实现发票相关方法 ---

func (s *DBStore) GetSuccessfulRechargeTotalInRange(ctx context.Context, userID int, startDate, endDate time.Time) (int64, error) {
    query := `
        SELECT COALESCE(SUM(amount), 0)
        FROM recharge_transactions
        WHERE user_id = ?
          AND status = 'Success'
          AND created_at >= ?
          AND created_at <= ?
    `
    // 注意：这里的日期比较是基于 'created_at'，即充值记录创建的时间。
    // 如果需要基于其他时间（如实际支付完成时间，如果记录了的话），需要调整。
    // 确保 endDate 包含当天最后一秒
    endOfDay := endDate.AddDate(0, 0, 1).Add(-1 * time.Nanosecond)

    var totalAmount int64
    err := s.db.QueryRowContext(ctx, query, userID, startDate, endOfDay).Scan(&totalAmount)
    if err != nil {
         // 如果 QueryRowContext 返回 sql.ErrNoRows, COALESCE 会返回 0，Scan 不会出错
         // 所以这里主要是处理其他数据库错误
         log.Printf("Error calculating successful recharge total for user %d between %v and %v: %v", userID, startDate, endDate, err)
         return 0, fmt.Errorf("store: failed to calculate recharge total: %w", err)
    }
    log.Printf("Calculated total successful recharge for user %d (%v to %v): %d cents", userID, startDate, endDate, totalAmount)
    return totalAmount, nil
}

func (s *DBStore) CreateInvoiceRequest(ctx context.Context, req models.InvoiceRequest) (int64, error) {
	query := `
        INSERT INTO invoice_requests (
            user_id, status, invoice_period_start, invoice_period_end, total_amount,
            billing_title, tax_id, billing_address, requested_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
	// 处理可选字段 tax_id
	var taxIDArg sql.NullString
	if req.TaxID != nil && *req.TaxID != "" {
		taxIDArg = sql.NullString{String: *req.TaxID, Valid: true}
	} else {
		// 如果 TaxID 是 nil 或者空字符串，我们存 NULL
		taxIDArg = sql.NullString{Valid: false}
	}

	// 执行插入
	result, err := s.db.ExecContext(ctx, query,
		req.UserID,
		req.Status, // 应该是 "Pending"
		req.InvoicePeriodStart,
		req.InvoicePeriodEnd,
		req.TotalAmount,
		req.BillingTitle,
		taxIDArg, // 使用处理后的 sql.NullString
		req.BillingAddress,
		req.RequestedAt, // 应该是 time.Now() 或从调用者传入
	)
	if err != nil {
		log.Printf("Error creating invoice request for user %d: %v", req.UserID, err)
		// 这里可以检查特定错误，例如外键约束 (user_id不存在)
		return 0, fmt.Errorf("store: failed to create invoice request: %w", err)
	}

	// 获取新插入记录的 ID
	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("Error getting last insert ID for invoice request (user %d): %v", req.UserID, err)
		return 0, fmt.Errorf("store: failed to get invoice request ID after creation: %w", err)
	}

	log.Printf("Successfully created invoice request ID %d for user %d", id, req.UserID)
	return id, nil
}


func (s *DBStore) GetInvoiceRequestsByUserID(ctx context.Context, userID int, filters models.InvoiceRequestFilter) ([]models.InvoiceRequest, error) {
	baseQuery := `
        SELECT id, user_id, status, invoice_period_start, invoice_period_end, total_amount,
               billing_title, tax_id, billing_address, invoice_number, notes,
               requested_at, processed_at
        FROM invoice_requests
    `
	conditions := []string{"user_id = ?"}
	args := []interface{}{userID}

	// 添加过滤条件
	if filters.Status != nil {
		conditions = append(conditions, "status = ?")
		args = append(args, *filters.Status)
	}
	if filters.StartDate != nil {
		conditions = append(conditions, "requested_at >= ?") // 按请求日期过滤
		args = append(args, *filters.StartDate)
	}
	if filters.EndDate != nil {
        // 包含当天
        endOfDay := filters.EndDate.AddDate(0,0,1).Add(-1 * time.Nanosecond)
		conditions = append(conditions, "requested_at <= ?") // 按请求日期过滤
		args = append(args, endOfDay)
	}

	finalQuery := baseQuery + " WHERE " + strings.Join(conditions, " AND ") + " ORDER BY requested_at DESC"

	log.Printf("Executing filtered invoice requests query for user %d: %s with args: %v", userID, finalQuery, args)

	rows, err := s.db.QueryContext(ctx, finalQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: failed to query invoice requests for user %d: %w", userID, err)
	}
	defer rows.Close()

	var requests []models.InvoiceRequest
	for rows.Next() {
		var req models.InvoiceRequest
		// 用于处理 nullable 字段的临时变量
		var nullableTaxID sql.NullString
		var nullableInvoiceNumber sql.NullString
		var nullableNotes sql.NullString
		var nullableProcessedAt sql.NullTime // 使用 sql.NullTime 处理 nullable timestamp

		err := rows.Scan(
			&req.ID, &req.UserID, &req.Status, &req.InvoicePeriodStart, &req.InvoicePeriodEnd, &req.TotalAmount,
			&req.BillingTitle, &nullableTaxID, &req.BillingAddress, &nullableInvoiceNumber, &nullableNotes,
			&req.RequestedAt, &nullableProcessedAt, // Scan 到 nullable 类型
		)
		if err != nil {
			log.Printf("store: failed to scan invoice request row for user %d: %v", userID, err)
			return nil, fmt.Errorf("store: error processing invoice request row: %w", err)
		}

		// 将 nullable 类型转换为模型中的指针类型
		if nullableTaxID.Valid { req.TaxID = &nullableTaxID.String } else { req.TaxID = nil }
		if nullableInvoiceNumber.Valid { req.InvoiceNumber = &nullableInvoiceNumber.String } else { req.InvoiceNumber = nil }
		if nullableNotes.Valid { req.Notes = &nullableNotes.String } else { req.Notes = nil }
		if nullableProcessedAt.Valid { req.ProcessedAt = &nullableProcessedAt.Time } else { req.ProcessedAt = nil }

		requests = append(requests, req)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("store: error iterating over invoice request rows for user %d: %w", userID, err)
	}

	return requests, nil
}


func (s *DBStore) GetInvoiceRequestByIDAndUser(ctx context.Context, invoiceID int64, userID int) (*models.InvoiceRequest, error) {
	query := `
        SELECT id, user_id, status, invoice_period_start, invoice_period_end, total_amount,
               billing_title, tax_id, billing_address, invoice_number, notes,
               requested_at, processed_at
        FROM invoice_requests
        WHERE id = ? AND user_id = ?
    `
	log.Printf("Executing get invoice request by ID %d and user %d query", invoiceID, userID)

	var req models.InvoiceRequest
	var nullableTaxID sql.NullString
	var nullableInvoiceNumber sql.NullString
	var nullableNotes sql.NullString
	var nullableProcessedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, invoiceID, userID).Scan(
		&req.ID, &req.UserID, &req.Status, &req.InvoicePeriodStart, &req.InvoicePeriodEnd, &req.TotalAmount,
		&req.BillingTitle, &nullableTaxID, &req.BillingAddress, &nullableInvoiceNumber, &nullableNotes,
		&req.RequestedAt, &nullableProcessedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Printf("Invoice request not found or access denied: ID=%d, UserID=%d", invoiceID, userID)
			return nil, ErrNotFound
		}
		log.Printf("Error querying invoice request ID %d for user %d: %v", invoiceID, userID, err)
		return nil, fmt.Errorf("store: failed to get invoice request %d for user %d: %w", invoiceID, userID, err)
	}

	// 转换 nullable 字段
	if nullableTaxID.Valid { req.TaxID = &nullableTaxID.String } else { req.TaxID = nil }
	if nullableInvoiceNumber.Valid { req.InvoiceNumber = &nullableInvoiceNumber.String } else { req.InvoiceNumber = nil }
	if nullableNotes.Valid { req.Notes = &nullableNotes.String } else { req.Notes = nil }
	if nullableProcessedAt.Valid { req.ProcessedAt = &nullableProcessedAt.Time } else { req.ProcessedAt = nil }

	return &req, nil
}


// --- (管理员功能，可选实现) ---
/*
func (s *DBStore) UpdateInvoiceRequestStatus(ctx context.Context, invoiceID int64, status string, invoiceNumber *string, notes *string, processedAt *time.Time) error {
    query := `
        UPDATE invoice_requests
        SET status = ?, invoice_number = ?, notes = ?, processed_at = ?, updated_at = ?
        WHERE id = ?
    `
    // 处理 nullable 参数
    var invNumArg sql.NullString
    if invoiceNumber != nil { invNumArg = sql.NullString{String: *invoiceNumber, Valid: true} }
    var notesArg sql.NullString
    if notes != nil { notesArg = sql.NullString{String: *notes, Valid: true} }
    var processedAtArg sql.NullTime
    if processedAt != nil { processedAtArg = sql.NullTime{Time: *processedAt, Valid: true} }

    result, err := s.db.ExecContext(ctx, query, status, invNumArg, notesArg, processedAtArg, time.Now(), invoiceID)
    if err != nil {
        log.Printf("Error updating invoice request %d status: %v", invoiceID, err)
        return fmt.Errorf("store: failed to update status for invoice request %d: %w", invoiceID, err)
    }
    rowsAffected, _ := result.RowsAffected()
    if rowsAffected == 0 {
        log.Printf("No rows affected when updating invoice request %d", invoiceID)
        return ErrNotFound // Invoice ID 不存在
    }
    log.Printf("Successfully updated invoice request %d status to %s", invoiceID, status)
    return nil
}
*/

// --- Helper: Check if DBStore implements Store ---
// 这个赋值语句如果编译不通过，说明 DBStore 没有完全实现 Store 接口
var _ Store = (*DBStore)(nil)

