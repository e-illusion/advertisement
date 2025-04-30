package handlers

import (
	//"database/sql" 不再需要
	"errors"
	"encoding/json"
	"fmt"
	// "errors" // GetAdHandler 里的 sql.ErrNoRows 不用 errors.Is
	"time"
	"log"
	"math" // 需要 math 包处理金额转换
	"math/rand" // 用于模拟支付
	"net/http"
	"strings"
	"strconv" // 需要导入 strconv 来转换 URL 参数中的 ID

	// "github.com/golang-jwt/jwt/v5" // 不再直接用 jwt
	"golang.org/x/crypto/bcrypt"

	// --- 导入内部包 ---
	"advertisement/internal/store"	
	"advertisement/internal/models"
	"advertisement/internal/auth"      // 替换 "your_module_name"
	"advertisement/internal/middleware" // 替换 "your_module_name"
	"advertisement/internal/webutil"   // 替换 "your_module_name"
)

// ... Handler, NewHandler, other handlers ...
const DateFormat = "2006-01-02" // 定义日期格式常量

// --- 修改 Handler 结构体，依赖 Store 接口 ---
type Handler struct {
	Store store.Store // 不再是 *sql.Store，而是 Store 接口
}

// --- 别忘了在 NewHandler 中初始化 rand ---

func NewHandler(s store.Store) *Handler {
	// rand.Seed(time.Now().UnixNano()) // 初始化随机数种子
	rand.New(rand.NewSource(time.Now().UnixNano()))
	return &Handler{Store: s}
}

// --- 新增：定义提交广告请求的结构体 ---
// 这个结构体只包含客户端需要提交的字段
type SubmitAdRequest struct {
	Title     string `json:"title"`
	ImageURL  string `json:"image_url"`
	TargetURL string `json:"target_url"`
}

// --- 新增：定义审核广告请求的结构体 ---
type ReviewAdRequest struct {
	Status string `json:"status"` // 只能是 "Approved" 或 "Rejected"
}

// RegisterHandler 方法修改: 使用 webutil
func (h *Handler) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持 POST 方法", http.StatusMethodNotAllowed)
		return
	}
	var creds models.UserCredentials
	err := json.NewDecoder(r.Body).Decode(&creds)
    if err != nil {
        http.Error(w, "请求体格式错误", http.StatusBadRequest)
        return
    }
    defer r.Body.Close()
    if creds.Username == "" || creds.Password == "" {
        webutil.RespondWithError(w, http.StatusBadRequest, "用户名和密码不能为空") // 使用 webutil
        return
    }
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(creds.Password), bcrypt.DefaultCost)
    if err != nil {
        log.Printf("密码哈希失败: %v", err)
        webutil.RespondWithError(w, http.StatusInternalServerError, "服务器内部错误") // 使用 webutil
        return
    }
	// --- 调用 Store 创建用户 ---
	err = h.Store.CreateUser(r.Context(), creds.Username, string(hashedPassword))
	if err != nil {
		if errors.Is(err, store.ErrDuplicateUser) { // 检查是否是用户名重复错误
			webutil.RespondWithError(w, http.StatusConflict, "用户名已存在")
		} else {
			log.Printf("调用 Store 创建用户失败: %v", err) // 记录包装后的错误
			webutil.RespondWithError(w, http.StatusInternalServerError, "注册失败，请稍后重试")
		}
		return
	}
    log.Printf("用户注册成功: %s", creds.Username)
    webutil.RespondWithJSON(w, http.StatusCreated, webutil.Response{Message: "注册成功"}) // 使用 webutil
}


// LoginHandler 方法修改: 调用 auth.GenerateJWT, 使用 webutil
func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持 POST 方法", http.StatusMethodNotAllowed)
		return
	}
	var creds models.UserCredentials
	err := json.NewDecoder(r.Body).Decode(&creds)
    if err != nil {
        http.Error(w, "请求体格式错误", http.StatusBadRequest)
        return
    }
    defer r.Body.Close()
    if creds.Username == "" || creds.Password == "" {
        webutil.RespondWithError(w, http.StatusBadRequest, "用户名和密码不能为空") // 使用 webutil
        return
    }
	// --- 调用 Store 获取用户 ---
	user, err := h.Store.GetUserByUsername(r.Context(), creds.Username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) { // 检查是否是用户未找到错误
			webutil.RespondWithError(w, http.StatusUnauthorized, "用户名或密码错误")
		} else {
			log.Printf("调用 Store 获取用户失败: %v", err)
			webutil.RespondWithError(w, http.StatusInternalServerError, "登录失败，请稍后重试")
		}
		return
	}
    // --- 比较密码 ---
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(creds.Password))
	if err != nil {
		// 密码不匹配也返回通用错误信息
		webutil.RespondWithError(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

    // --- 修改：传递 user.Role 给 GenerateJWT ---
    tokenString, _, err := auth.GenerateJWT(user.ID, user.Username, user.Role) // 传递 Role
    if err != nil {
        log.Printf("生成 JWT 失败: %v", err)
        webutil.RespondWithError(w, http.StatusInternalServerError, "登录时无法生成凭证")
        return
    }

    log.Printf("用户登录成功: %s (Role: %s), 生成 Token", user.Username, user.Role) // 日志可以加上角色
    webutil.RespondWithJSON(w, http.StatusOK, webutil.Response{
        Message: "登录成功",
        Data:    map[string]string{"token": tokenString},
    })
}

// GetAdHandler 处理获取广告的请求 (给广告位调用)
func (h *Handler) GetAdHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        webutil.RespondWithError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
        return
    }

    // 1. 选择一个当前有效的广告活动
    // 在 store 层增加 GetActiveRandomCampaign 方法 (需要实现)
    // 或者简化：先随机选一个 Approved 的活动，再获取其广告
    campaign, err := h.Store.GetRandomActiveCampaign(r.Context()) // <--- 需要在 Store 实现此方法
    if err != nil {
        if errors.Is(err, store.ErrNotFound) {
            // 没有可投放的广告是正常情况
            log.Println("没有找到可投放的广告活动")
            webutil.RespondWithJSON(w, http.StatusOK, webutil.Response{Message: "没有可用的广告"}) // 返回 200 但内容为空
            return
        }
        log.Printf("选择活动失败: %v", err)
        webutil.RespondWithError(w, http.StatusInternalServerError, "获取广告时出错")
        return
    }

    // 2. 获取该活动关联的广告创意信息
    ad, err := h.Store.GetAdvertisementByID(r.Context(), campaign.AdvertisementID)
    if err != nil {
        // 如果广告被删除但活动还在，可能出现此情况
        log.Printf("获取活动 %d 关联的广告 %d 失败: %v", campaign.ID, campaign.AdvertisementID, err)
        webutil.RespondWithError(w, http.StatusInternalServerError, "获取广告详情时出错")
        return
    }

    // 3. --- 记录 Impression 事件 ---
    impressionEvent := models.AdEvent{
        EventType:       "Impression",
        AdvertisementID: ad.ID,
        CampaignID:      campaign.ID,
        UserID:          campaign.UserID, // 活动创建者的 ID
        EventTimestamp:  time.Now(),
    }
    logErr := h.Store.LogAdEvent(r.Context(), impressionEvent)
    if logErr != nil {
        // 记录失败不应阻止广告返回，但需要记录日志
        log.Printf("!!! 记录 Impression 事件失败 (但广告已返回): campaign %d, ad %d: %v", campaign.ID, ad.ID, logErr)
    } else {
         log.Printf("记录 Impression: campaign %d, ad %d", campaign.ID, ad.ID)
    }


    // 4. 准备并返回广告数据给广告位
    adResponse := struct {
        CampaignID      int    `json:"campaign_id"` // 传递 CampaignID 可能对后续点击追踪有用
        AdvertisementID int    `json:"advertisement_id"`
        Title           string `json:"title"`
        ImageURL        string `json:"image_url"`
        TargetURL       string `json:"target_url"` // 点击后跳转的地址
    }{
        CampaignID:      campaign.ID,
        AdvertisementID: ad.ID,
        Title:           ad.Title,
        ImageURL:        ad.ImageURL,
        TargetURL:       ad.TargetURL,
    }

    webutil.RespondWithJSON(w, http.StatusOK, webutil.Response{Data: adResponse})
}

// --- 新增：SubmitAdHandler 方法处理广告提交 ---
func (h *Handler) SubmitAdHandler(w http.ResponseWriter, r *http.Request) {
	// 1. 检查请求方法
	if r.Method != http.MethodPost {
		webutil.RespondWithError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	// 2. 从 Context 获取用户信息 (由 AuthMiddleware 设置)
	userClaims, ok := r.Context().Value(middleware.UserContextKey).(*auth.Claims)
	if !ok || userClaims == nil {
		// 理论上中间件会拦截未授权请求，但以防万一
		log.Println("错误：无法从 context 中获取有效的用户信息")
		webutil.RespondWithError(w, http.StatusUnauthorized, "无效的认证凭证或无法获取用户信息")
		return
	}
	userID := userClaims.UserID // 获取用户 ID

	// 3. 解码请求体
	var req SubmitAdRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Printf("解码广告提交请求体失败: %v", err)
		webutil.RespondWithError(w, http.StatusBadRequest, "请求体格式错误")
		return
	}
	defer r.Body.Close()

	// 4. 验证输入数据 (基础验证)
	if strings.TrimSpace(req.Title) == "" {
		webutil.RespondWithError(w, http.StatusBadRequest, "广告标题不能为空")
		return
	}
	if strings.TrimSpace(req.ImageURL) == "" {
		webutil.RespondWithError(w, http.StatusBadRequest, "图片 URL 不能为空")
		return
	}
	if strings.TrimSpace(req.TargetURL) == "" {
		webutil.RespondWithError(w, http.StatusBadRequest, "目标 URL 不能为空")
		return
	}
	// 在真实应用中可能需要更复杂的验证, 比如 URL 格式检查

	// --- 创建 Advertisement 模型对象 ---
	ad := &models.Advertisement{
		Title:     req.Title,
		ImageURL:  req.ImageURL,
		TargetURL: req.TargetURL,
		UserID:    userID,      // 从 Token 获取
		Status:    "Pending", // 设置初始状态
	}

	// --- 调用 Store 创建广告 ---
	newAdID, err := h.Store.CreateAdvertisement(r.Context(), ad)
	if err != nil {
		log.Printf("调用 Store 创建广告失败: %v", err)
		webutil.RespondWithError(w, http.StatusInternalServerError, "保存广告失败")
		return
	}

	log.Printf("用户 %d 成功提交广告, 新广告 ID: %d", userID, newAdID)
	responsePayload := webutil.Response{
		Message: "广告提交成功，等待审核",
		Data:    map[string]int64{"new_ad_id": newAdID},
	}
	webutil.RespondWithJSON(w, http.StatusCreated, responsePayload)
}

// --- 新增：GetUserAdsHandler 方法获取用户自己的广告列表 ---
func (h *Handler) GetUserAdsHandler(w http.ResponseWriter, r *http.Request) {
	// 1. 检查请求方法
	if r.Method != http.MethodGet {
		webutil.RespondWithError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	// 2. 从 Context 获取用户信息
	userClaims, ok := r.Context().Value(middleware.UserContextKey).(*auth.Claims)
	if !ok || userClaims == nil {
		log.Println("错误：无法从 context 中获取有效的用户信息 (GetUserAdsHandler)")
		webutil.RespondWithError(w, http.StatusUnauthorized, "无效的认证凭证或无法获取用户信息")
		return
	}
	userID := userClaims.UserID

	// --- 调用 Store 获取用户广告列表 ---
	userAds, err := h.Store.GetAdvertisementsByUserID(r.Context(), userID)
	if err != nil {
		// 这里假设 Store 层已经记录了具体错误，Handler 只需返回通用错误
		log.Printf("调用 Store 获取用户 %d 的广告失败: %v", userID, err)
		webutil.RespondWithError(w, http.StatusInternalServerError, "获取广告列表失败")
		return
	}
	// 注意: Store 返回 nil 错误和空切片表示用户没有广告，这是正常情况

	log.Printf("成功获取用户 %d 的 %d 条广告", userID, len(userAds))
	webutil.RespondWithJSON(w, http.StatusOK, webutil.Response{Data: userAds}) // 直接返回从 Store 获取的切片
}

// --- 新增：ReviewAdHandler 方法处理广告审核 ---
func (h *Handler) ReviewAdHandler(w http.ResponseWriter, r *http.Request) {
	// 1. 检查请求方法 (使用 PATCH)
	if r.Method != http.MethodPatch {
		webutil.RespondWithError(w, http.StatusMethodNotAllowed, "仅支持 PATCH 方法")
		return
	}

	// 2. 从 URL 路径中获取广告 ID
	//    假设使用 Go 1.22+ 的路径参数: /ads/{id}/status
	//    如果低于 Go 1.22, 你需要使用第三方路由器或手动解析 r.URL.Path
	idStr := r.PathValue("id") // Go 1.22+
    /* // --- 如果低于 Go 1.22 或不用路由器，手动解析示例 ---
    pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
    // 假设 URL 是 /ads/{id}/status, parts 会是 ["ads", "{id}", "status"]
    if len(pathParts) != 3 || pathParts[0] != "ads" || pathParts[2] != "status" {
        webutil.RespondWithError(w, http.StatusBadRequest, "无效的请求路径格式")
        return
    }
    idStr := pathParts[1]
    // --- 手动解析结束 --- */
    if idStr == "" {
        webutil.RespondWithError(w, http.StatusBadRequest, "缺少广告 ID")
        return
    }

	adID, err := strconv.Atoi(idStr)
	if err != nil || adID <= 0 {
		webutil.RespondWithError(w, http.StatusBadRequest, "无效的广告 ID 格式")
		return
	}

    // (可选但推荐): 调用 Store 检查广告是否存在，可以提前返回 404
    _, err = h.Store.GetAdvertisementByID(r.Context(), adID)
    if err != nil {
        if errors.Is(err, store.ErrNotFound) {
            webutil.RespondWithError(w, http.StatusNotFound, "找不到指定的广告")
        } else {
            log.Printf("检查广告 %d 存在性失败: %v", adID, err)
            webutil.RespondWithError(w, http.StatusInternalServerError, "处理请求时出错")
        }
        return
    }


	// 3. 解码请求体中的新状态
	var req ReviewAdRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		webutil.RespondWithError(w, http.StatusBadRequest, "请求体格式错误，应为 {'status': 'Approved|Rejected'}")
		return
	}
	defer r.Body.Close()

	// 4. 验证状态值
	newStatus := strings.TrimSpace(req.Status)
	if newStatus != "Approved" && newStatus != "Rejected" {
		webutil.RespondWithError(w, http.StatusBadRequest, "无效的状态值，只能是 'Approved' 或 'Rejected'")
		return
	}

	// 5. 调用 Store 更新广告状态
	err = h.Store.UpdateAdvertisementStatus(r.Context(), adID, newStatus)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			webutil.RespondWithError(w, http.StatusNotFound, "找不到要更新的广告")
		} else {
			log.Printf("调用 Store 更新广告 %d 状态失败: %v", adID, err)
			webutil.RespondWithError(w, http.StatusInternalServerError, "更新广告状态失败")
		}
		return
	}

	// 6. 返回成功响应
	log.Printf("管理员成功将广告 %d 状态更新为 %s", adID, newStatus)
	webutil.RespondWithJSON(w, http.StatusOK, webutil.Response{
		Message: fmt.Sprintf("广告 %d 状态已更新为 %s", adID, newStatus),
	})
}

// --- RequestCampaignHandler 处理用户提交广告活动请求 ---
func (h *Handler) RequestCampaignHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        webutil.RespondWithError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
        return
    }

    // 1. 获取用户信息
    userClaims, ok := r.Context().Value(middleware.UserContextKey).(*auth.Claims)
    if !ok || userClaims == nil {
        webutil.RespondWithError(w, http.StatusUnauthorized, "无效的用户信息")
        return
    }
    userID := userClaims.UserID

    // 2. 解码请求体
    var reqData models.CampaignRequestData
    if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
        webutil.RespondWithError(w, http.StatusBadRequest, "请求体格式错误，需要 advertisement_id, start_date (YYYY-MM-DD), end_date (YYYY-MM-DD)")
        return
    }
    defer r.Body.Close()

    // 3. 验证 advertisement_id
    if reqData.AdvertisementID <= 0 {
        webutil.RespondWithError(w, http.StatusBadRequest, "无效的广告创意 ID")
        return
    }

    // 4. 解析并验证日期
    startDate, err := time.Parse(DateFormat, reqData.StartDate)
    if err != nil {
        webutil.RespondWithError(w, http.StatusBadRequest, "无效的开始日期格式，应为 YYYY-MM-DD")
        return
    }
    endDate, err := time.Parse(DateFormat, reqData.EndDate)
    if err != nil {
        webutil.RespondWithError(w, http.StatusBadRequest, "无效的结束日期格式，应为 YYYY-MM-DD")
        return
    }
    // 验证日期逻辑：结束日期不能早于开始日期，开始日期不能是过去（可选）
    if endDate.Before(startDate) {
        webutil.RespondWithError(w, http.StatusBadRequest, "结束日期不能早于开始日期")
        return
    }
    // Optional: Check if start date is in the future
    // if startDate.Before(time.Now().Truncate(24 * time.Hour)) {
    //     webutil.RespondWithError(w, http.StatusBadRequest, "开始日期不能是过去")
    //     return
    // }


    // 5. 验证广告创意是否存在、是否已批准、是否属于当前用户
    adCreative, err := h.Store.GetAdvertisementByID(r.Context(), reqData.AdvertisementID)
    if err != nil {
        if errors.Is(err, store.ErrNotFound) {
            webutil.RespondWithError(w, http.StatusNotFound, "指定的广告创意不存在")
        } else {
            log.Printf("获取广告创意 %d 失败: %v", reqData.AdvertisementID, err)
            webutil.RespondWithError(w, http.StatusInternalServerError, "验证广告创意时出错")
        }
        return
    }
    if adCreative.UserID != userID {
        webutil.RespondWithError(w, http.StatusForbidden, "不能为不属于自己的广告创意请求活动")
        return
    }
    if adCreative.Status != "Approved" {
        webutil.RespondWithError(w, http.StatusBadRequest, "只能为已批准的广告创意请求活动")
        return
    }

    // 6. 创建 AdCampaign 对象
    campaign := &models.AdCampaign{
        AdvertisementID: reqData.AdvertisementID,
        UserID:         userID,
        StartDate:      startDate,
        EndDate:        endDate,
        Status:         "Pending", // 新请求默认为 Pending
    }

    // 7. 调用 Store 创建活动请求
    campaignID, err := h.Store.CreateAdCampaign(r.Context(), campaign)
    if err != nil {
        log.Printf("创建广告活动请求失败: %v", err)
        webutil.RespondWithError(w, http.StatusInternalServerError, "提交广告活动请求失败")
        return
    }

    // 8. 返回成功响应
    log.Printf("用户 %d 成功为广告 %d 请求活动, 活动 ID: %d", userID, reqData.AdvertisementID, campaignID)
    webutil.RespondWithJSON(w, http.StatusCreated, webutil.Response{
        Message: "广告活动请求已提交，等待审核",
        Data:    map[string]int64{"campaign_id": campaignID},
    })
}

// --- ReviewCampaignHandler 处理管理员审核广告活动请求 ---
func (h *Handler) ReviewCampaignHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPatch {
        webutil.RespondWithError(w, http.StatusMethodNotAllowed, "仅支持 PATCH 方法")
        return
    }

    // 1. 从 URL 获取活动 ID (需要 Go 1.22+ 或路由器)
    campaignIDStr := r.PathValue("id") // Go 1.22+
    if campaignIDStr == "" { /* 处理 ID 缺失 */ return }
    campaignID, err := strconv.Atoi(campaignIDStr)
    if err != nil || campaignID <= 0 {
         webutil.RespondWithError(w, http.StatusBadRequest, "无效的活动 ID")
         return
    }

    // 2. 解码请求体
    var reqData models.CampaignReviewData
    if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
        webutil.RespondWithError(w, http.StatusBadRequest, "请求体格式错误，应为 {'status': 'Approved|Rejected'}")
        return
    }
    defer r.Body.Close()

    // 3. 验证状态值
    newStatus := strings.TrimSpace(reqData.Status)
    if newStatus != "Approved" && newStatus != "Rejected" {
        webutil.RespondWithError(w, http.StatusBadRequest, "无效的状态值，只能是 'Approved' 或 'Rejected'")
        return
    }

    // (可选) 检查活动是否存在且状态为 Pending
    // campaign, err := h.Store.GetAdCampaignByID(r.Context(), campaignID)
    // if err != nil { ... handle ErrNotFound ... }
    // if campaign.Status != "Pending" { ... handle already reviewed ... }


    // 4. 调用 Store 更新活动状态
    err = h.Store.UpdateAdCampaignStatus(r.Context(), campaignID, newStatus)
    if err != nil {
        if errors.Is(err, store.ErrNotFound) {
            webutil.RespondWithError(w, http.StatusNotFound, "找不到要审核的广告活动")
        } else {
            log.Printf("更新广告活动 %d 状态失败: %v", campaignID, err)
            webutil.RespondWithError(w, http.StatusInternalServerError, "更新活动状态失败")
        }
        return
    }

    // 5. 返回成功响应
    log.Printf("管理员成功将广告活动 %d 状态更新为 %s", campaignID, newStatus)
    webutil.RespondWithJSON(w, http.StatusOK, webutil.Response{
        Message: fmt.Sprintf("广告活动 %d 状态已更新为 %s", campaignID, newStatus),
    })
}

// --- RechargeHandler 处理充值请求 ---
func (h *Handler) RechargeHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        webutil.RespondWithError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
        return
    }

    // 1. 获取用户信息
    userClaims, ok := r.Context().Value(middleware.UserContextKey).(*auth.Claims)
    if !ok || userClaims == nil {
        webutil.RespondWithError(w, http.StatusUnauthorized, "无效的用户信息")
        return
    }
    userID := userClaims.UserID

    // 2. 解码请求体
    var req models.RechargeRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        webutil.RespondWithError(w, http.StatusBadRequest, "请求体格式错误，应为 {'amount': 100.50}")
        return
    }
    defer r.Body.Close()

    // 3. 验证金额
    if req.Amount <= 0 {
        webutil.RespondWithError(w, http.StatusBadRequest, "充值金额必须大于 0")
        return
    }
    // 将用户输入的金额（元）转换为分的整数
    // 注意：直接乘法可能因浮点数不精确导致错误，推荐先转为字符串处理或使用特定库
    // 简单处理：乘以 100 并四舍五入到最近的整数分
    amountInCents := int64(math.Round(req.Amount * 100))
    if amountInCents <= 0 {
        // 再次检查，防止极小的正浮点数四舍五入后变成 0
         webutil.RespondWithError(w, http.StatusBadRequest, "无效的充值金额（处理后为 0 分）")
         return
    }


    // 4. 创建初始充值记录 (Pending)
    paymentMethod := "Simulated" // 当前只支持模拟支付
    rechargeRecordID, err := h.Store.CreateRechargeTransaction(r.Context(), userID, amountInCents, paymentMethod)
    if err != nil {
        log.Printf("创建充值记录失败 (用户 %d): %v", userID, err)
        webutil.RespondWithError(w, http.StatusInternalServerError, "处理充值请求失败（无法创建记录）")
        return
    }
    log.Printf("用户 %d 发起充值 %d 分，创建记录 ID: %d", userID, amountInCents, rechargeRecordID)


    // --- 5. 模拟支付过程 ---
    // 简单模拟：80% 成功率
    paymentSuccess := rand.Float32() < 0.80 // 0.0 <= f < 1.0

    if paymentSuccess {
        // 模拟支付成功
        simulatedTxID := fmt.Sprintf("SIM_TX_%d_%d", time.Now().UnixNano(), rechargeRecordID)
        log.Printf("模拟支付成功 (记录 ID: %d), TxID: %s", rechargeRecordID, simulatedTxID)

        // 调用 Store 原子性地更新余额和交易状态
        err = h.Store.ProcessSuccessfulRecharge(r.Context(), userID, amountInCents, rechargeRecordID, simulatedTxID)
        if err != nil {
            log.Printf("处理成功充值失败 (用户 %d, 记录 %d): %v", userID, rechargeRecordID, err)
            // 此时数据库可能处于不一致状态（记录可能是 Pending 但支付已模拟成功）
            // 最好尝试将记录标记为需要人工处理的状态，但这里简化处理
            // 仍然尝试将记录标记为 Failed，虽然不完全准确
            _ = h.Store.UpdateRechargeTransactionStatus(r.Context(), rechargeRecordID, "Failed", "PROCESSING_ERROR") // 忽略错误
            webutil.RespondWithError(w, http.StatusInternalServerError, "充值处理失败（更新账户时出错）")
            return
        }

        log.Printf("用户 %d 充值 %d 分成功 (记录 ID: %d)", userID, amountInCents, rechargeRecordID)
        webutil.RespondWithJSON(w, http.StatusOK, webutil.Response{
            Message: fmt.Sprintf("充值 %.2f 元成功", req.Amount),
            Data:    map[string]string{"transaction_id": simulatedTxID},
        })

    } else {
        // 模拟支付失败
        simulatedTxID := fmt.Sprintf("SIM_FAIL_%d_%d", time.Now().UnixNano(), rechargeRecordID)
        log.Printf("模拟支付失败 (记录 ID: %d), FailID: %s", rechargeRecordID, simulatedTxID)

        // 更新交易记录状态为 Failed
        err = h.Store.UpdateRechargeTransactionStatus(r.Context(), rechargeRecordID, "Failed", simulatedTxID)
        if err != nil {
            log.Printf("更新充值记录为失败状态失败 (记录 %d): %v", rechargeRecordID, err)
            // 即使更新状态失败，也应告知用户支付失败
        }

        webutil.RespondWithError(w, http.StatusBadRequest, "充值失败（模拟支付未通过）") // 返回 400 表示客户端操作导致的问题（支付失败）
    }
}

// --- GetBalanceHandler 获取用户当前余额 ---
func (h *Handler) GetBalanceHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        webutil.RespondWithError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
        return
    }

    // 1. 获取用户信息
    userClaims, ok := r.Context().Value(middleware.UserContextKey).(*auth.Claims)
    if !ok || userClaims == nil {
        webutil.RespondWithError(w, http.StatusUnauthorized, "无效的用户信息")
        return
    }
    userID := userClaims.UserID

    // 2. 调用 Store 获取余额 (单位：分)
    balanceInCents, err := h.Store.GetUserBalance(r.Context(), userID)
    if err != nil {
        if errors.Is(err, store.ErrNotFound) {
             webutil.RespondWithError(w, http.StatusNotFound, "找不到用户信息")
        } else {
            log.Printf("获取用户 %d 余额失败: %v", userID, err)
            webutil.RespondWithError(w, http.StatusInternalServerError, "获取余额失败")
        }
        return
    }

    // 3. 将余额从分转换为元 (float64)
    balanceInYuan := float64(balanceInCents) / 100.0

    // 4. 返回响应
    response := models.BalanceResponse{
        Balance: balanceInYuan,
        Currency: "CNY", // 或从配置读取
    }
    webutil.RespondWithJSON(w, http.StatusOK, webutil.Response{Data: response})
}

// --- 修改 GetRechargeHistoryHandler 以支持过滤 ---
func (h *Handler) GetRechargeHistoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		webutil.RespondWithError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	// 1. 获取用户信息 (不变)
	userClaims, ok := r.Context().Value(middleware.UserContextKey).(*auth.Claims)
	if !ok || userClaims == nil {
		webutil.RespondWithError(w, http.StatusUnauthorized, "无效的用户信息")
		return
	}
	userID := userClaims.UserID

	// 2. 解析查询参数并构建过滤器
	filters := models.RechargeHistoryFilters{}
	query := r.URL.Query() // 获取查询参数

	// 解析日期范围
	if startDateStr := query.Get("start_date"); startDateStr != "" {
		t, err := time.Parse(DateFormat, startDateStr) // DateFormat = "2006-01-02"
		if err != nil {
			webutil.RespondWithError(w, http.StatusBadRequest, "无效的开始日期格式，应为 YYYY-MM-DD")
			return
		}
		// 将时间设为当天的开始 (00:00:00) 以包含当天所有记录
		startOfDay := t.Truncate(24 * time.Hour)
		filters.StartDate = &startOfDay
	}
	if endDateStr := query.Get("end_date"); endDateStr != "" {
		t, err := time.Parse(DateFormat, endDateStr)
		if err != nil {
			webutil.RespondWithError(w, http.StatusBadRequest, "无效的结束日期格式，应为 YYYY-MM-DD")
			return
		}
		// 将时间设为当天的结束 (23:59:59.999...) 以包含当天所有记录
        // 加上一天再减去一纳秒是一种常用的方法
		endOfDay := t.AddDate(0, 0, 1).Add(-1 * time.Nanosecond)
		filters.EndDate = &endOfDay
	}

	// 校验日期逻辑：如果同时提供了开始和结束日期，确保结束不早于开始
    if filters.StartDate != nil && filters.EndDate != nil && filters.EndDate.Before(*filters.StartDate) {
         webutil.RespondWithError(w, http.StatusBadRequest, "结束日期不能早于开始日期")
         return
    }


	// 解析金额范围 (用户输入的是元，需转换为分)
	if minAmountStr := query.Get("min_amount"); minAmountStr != "" {
		minAmountYuan, err := strconv.ParseFloat(minAmountStr, 64)
		if err != nil || minAmountYuan < 0 {
			webutil.RespondWithError(w, http.StatusBadRequest, "无效的最小金额格式或值")
			return
		}
		minAmountCents := int64(math.Round(minAmountYuan * 100))
		filters.MinAmount = &minAmountCents
	}
	if maxAmountStr := query.Get("max_amount"); maxAmountStr != "" {
		maxAmountYuan, err := strconv.ParseFloat(maxAmountStr, 64)
		if err != nil || maxAmountYuan < 0 {
			webutil.RespondWithError(w, http.StatusBadRequest, "无效的最大金额格式或值")
			return
		}
		maxAmountCents := int64(math.Round(maxAmountYuan * 100))
		filters.MaxAmount = &maxAmountCents
	}
    // 校验金额范围
    if filters.MinAmount != nil && filters.MaxAmount != nil && *filters.MaxAmount < *filters.MinAmount {
        webutil.RespondWithError(w, http.StatusBadRequest, "最大金额不能小于最小金额")
        return
    }

	// 解析状态
	if status := query.Get("status"); status != "" {
        // 可以添加验证，确保 status 是 'Success', 'Failed', 'Pending' 之一
        validStatuses := map[string]bool{"Success": true, "Failed": true, "Pending": true}
        normalizedStatus := strings.Title(strings.ToLower(status)) // 规范化大小写 (e.g., success -> Success)
        if !validStatuses[normalizedStatus] {
             webutil.RespondWithError(w, http.StatusBadRequest, "无效的状态值，应为 Success, Failed 或 Pending")
             return
        }
		filters.Status = &normalizedStatus
	}

	// 3. 调用 Store 获取带过滤的充值历史
	history, err := h.Store.GetUserRechargeHistory(r.Context(), userID, filters) // 传递 filters
	if err != nil {
		log.Printf("获取用户 %d 充值历史失败 (带过滤): %v", userID, err)
		webutil.RespondWithError(w, http.StatusInternalServerError, "获取充值记录失败")
		return
	}

	// 4. 返回响应 (不变)
	webutil.RespondWithJSON(w, http.StatusOK, webutil.Response{Data: history})
}

// --- GetUserCampaignsHandler 获取用户的广告活动列表 ---
func (h *Handler) GetUserCampaignsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		webutil.RespondWithError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	// 1. 获取用户信息
	userClaims, ok := r.Context().Value(middleware.UserContextKey).(*auth.Claims)
	if !ok || userClaims == nil {
		webutil.RespondWithError(w, http.StatusUnauthorized, "无效的用户信息")
		return
	}
	userID := userClaims.UserID

	// 2. 解析查询参数并构建过滤器 (类似 GetRechargeHistoryHandler)
	filters := models.CampaignFilters{}
	query := r.URL.Query()

	// 解析日期范围 (假设 start_date, end_date 指的是活动的投放日期)
	if startDateStr := query.Get("start_date"); startDateStr != "" {
		t, err := time.Parse(DateFormat, startDateStr)
		if err != nil { webutil.RespondWithError(w, http.StatusBadRequest, "无效的开始日期格式 (YYYY-MM-DD)"); return }
		filters.StartDate = &t
	}
	if endDateStr := query.Get("end_date"); endDateStr != "" {
		t, err := time.Parse(DateFormat, endDateStr)
		if err != nil { webutil.RespondWithError(w, http.StatusBadRequest, "无效的结束日期格式 (YYYY-MM-DD)"); return }
        // 注意：这里的日期比较逻辑可能需要根据业务调整，是筛选创建日期还是活动投放日期
        // Store 层当前是按 start_date 和 end_date 字段过滤
		filters.EndDate = &t
	}
    if filters.StartDate != nil && filters.EndDate != nil && filters.EndDate.Before(*filters.StartDate) {
         webutil.RespondWithError(w, http.StatusBadRequest, "结束日期不能早于开始日期"); return
    }

	// 解析状态
	if status := query.Get("status"); status != "" {
        // 可以添加状态验证
        validStatuses := map[string]bool{"Pending": true, "Approved": true, "Rejected": true, "Cancelled": true, "Active": true, "Completed": true} // 可能的状态
        normalizedStatus := strings.Title(strings.ToLower(status))
        if !validStatuses[normalizedStatus] {
            webutil.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("无效的状态值: %s", status)); return
        }
		filters.Status = &normalizedStatus
	}

	// 3. 调用 Store 获取活动列表
	campaigns, err := h.Store.GetAdCampaignsByUserID(r.Context(), userID, filters)
	if err != nil {
		log.Printf("获取用户 %d 活动列表失败 (带过滤): %v", userID, err)
		webutil.RespondWithError(w, http.StatusInternalServerError, "获取广告活动列表失败")
		return
	}

	// 4. 返回响应
	webutil.RespondWithJSON(w, http.StatusOK, webutil.Response{Data: campaigns})
}

// --- GetUserCampaignDetailsHandler 获取用户单个广告活动详情 ---
func (h *Handler) GetUserCampaignDetailsHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        webutil.RespondWithError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
        return
    }

    // 1. 获取用户信息
    userClaims, ok := r.Context().Value(middleware.UserContextKey).(*auth.Claims)
    if !ok || userClaims == nil { webutil.RespondWithError(w, http.StatusUnauthorized, "无效的用户信息"); return }
    userID := userClaims.UserID

    // 2. 从 URL 获取活动 ID (Go 1.22+)
    campaignIDStr := r.PathValue("id")
    if campaignIDStr == "" { webutil.RespondWithError(w, http.StatusBadRequest, "缺少活动 ID"); return }
    campaignID, err := strconv.Atoi(campaignIDStr)
    if err != nil || campaignID <= 0 {
        webutil.RespondWithError(w, http.StatusBadRequest, "无效的活动 ID"); return
    }

    // 3. 调用 Store 获取详情 (已包含用户 ID 校验)
    campaignDetails, err := h.Store.GetAdCampaignByIDAndUser(r.Context(), campaignID, userID)
    if err != nil {
        if errors.Is(err, store.ErrNotFound) {
            webutil.RespondWithError(w, http.StatusNotFound, "找不到指定的广告活动或无权访问")
        } else {
            log.Printf("获取用户 %d 的活动详情 %d 失败: %v", userID, campaignID, err)
            webutil.RespondWithError(w, http.StatusInternalServerError, "获取活动详情失败")
        }
        return
    }

    // 4. 返回响应
    webutil.RespondWithJSON(w, http.StatusOK, webutil.Response{Data: campaignDetails})
}


// --- CancelCampaignHandler 用户取消自己的广告活动 ---
func (h *Handler) CancelCampaignHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPatch { // 或者 POST，看 API 设计风格
        webutil.RespondWithError(w, http.StatusMethodNotAllowed, "仅支持 PATCH 方法")
        return
    }

    // 1. 获取用户信息
    userClaims, ok := r.Context().Value(middleware.UserContextKey).(*auth.Claims)
    if !ok || userClaims == nil { webutil.RespondWithError(w, http.StatusUnauthorized, "无效的用户信息"); return }
    userID := userClaims.UserID

    // 2. 从 URL 获取活动 ID (Go 1.22+)
    campaignIDStr := r.PathValue("id")
    if campaignIDStr == "" { webutil.RespondWithError(w, http.StatusBadRequest, "缺少活动 ID"); return }
    campaignID, err := strconv.Atoi(campaignIDStr)
    if err != nil || campaignID <= 0 {
        webutil.RespondWithError(w, http.StatusBadRequest, "无效的活动 ID"); return
    }

    // (可选) 检查请求体，如果需要传递额外参数（例如取消原因）

    // 3. 调用 Store 更新状态为 'Cancelled'
    newStatus := "Cancelled"
    err = h.Store.UpdateAdCampaignStatusByUser(r.Context(), campaignID, userID, newStatus)
    if err != nil {
        if errors.Is(err, store.ErrNotFound) {
            // Store 返回 ErrNotFound 表示活动不存在、不属于该用户或状态不允许取消
            webutil.RespondWithError(w, http.StatusNotFound, "找不到要取消的活动，或该活动无法被取消")
        } else {
            // 其他 Store 层错误 (如数据库连接问题)
            log.Printf("用户 %d 取消活动 %d 失败: %v", userID, campaignID, err)
            webutil.RespondWithError(w, http.StatusInternalServerError, "取消活动时出错")
        }
        return
    }

    // 4. 返回成功响应
    log.Printf("用户 %d 成功取消了活动 %d", userID, campaignID)
    webutil.RespondWithJSON(w, http.StatusOK, webutil.Response{
        Message: fmt.Sprintf("广告活动 %d 已成功取消", campaignID),
    })
}

// GetAdPerformanceHandler 获取广告效果汇总数据
func (h *Handler) GetAdPerformanceHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        webutil.RespondWithError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法"); return
    }

    // 1. 获取用户信息
    userClaims, ok := r.Context().Value(middleware.UserContextKey).(*auth.Claims)
    if !ok || userClaims == nil { webutil.RespondWithError(w, http.StatusUnauthorized, "无效的用户信息"); return }
    userID := userClaims.UserID

    // 2. 解析查询参数 (过滤条件)
    filters := models.AdPerformanceFilter{}
    query := r.URL.Query()

    // 解析日期范围
    if startDateStr := query.Get("start_date"); startDateStr != "" {
        t, err := time.Parse(DateFormat, startDateStr)
        if err == nil { filters.StartDate = &t }
    }
    if endDateStr := query.Get("end_date"); endDateStr != "" {
        t, err := time.Parse(DateFormat, endDateStr)
        if err == nil { filters.EndDate = &t }
    }
    if filters.StartDate != nil && filters.EndDate != nil && filters.EndDate.Before(*filters.StartDate) {
        webutil.RespondWithError(w, http.StatusBadRequest, "结束日期不能早于开始日期"); return
    }
     // 设置默认日期范围（例如过去 7 天）如果未提供? 可选
    if filters.StartDate == nil && filters.EndDate == nil {
        now := time.Now()
        sevenDaysAgo := now.AddDate(0, 0, -7).Truncate(24 * time.Hour)
        todayEnd := now.Truncate(24 * time.Hour).AddDate(0,0,1).Add(-1*time.Nanosecond) // 包含今天
        filters.StartDate = &sevenDaysAgo
        filters.EndDate = &todayEnd
        log.Printf("未指定日期范围，默认查询 %v 到 %v", sevenDaysAgo, todayEnd)
    } else if filters.StartDate != nil && filters.EndDate == nil {
         // 如果只提供了开始日期，则默认结束日期为当前时间
         nowEnd := time.Now()
         filters.EndDate = &nowEnd
    } else if filters.StartDate == nil && filters.EndDate != nil {
        // 如果只提供了结束日期，可以报错或设置一个默认的开始日期（如1年前）
         webutil.RespondWithError(w, http.StatusBadRequest, "请提供开始日期或不提供任何日期以使用默认范围"); return
    }


    // 解析可选的 CampaignID
    if campaignIDStr := query.Get("campaign_id"); campaignIDStr != "" {
        campID, err := strconv.Atoi(campaignIDStr)
        if err == nil && campID > 0 {
            filters.CampaignID = &campID
        } else {
             webutil.RespondWithError(w, http.StatusBadRequest, "无效的活动 ID"); return
        }
    }

    // 3. 调用 Store 获取汇总数据
    summaryData, err := h.Store.GetAdPerformanceSummary(r.Context(), userID, filters)
    if err != nil {
        log.Printf("获取用户 %d 广告效果失败: %v", userID, err)
        webutil.RespondWithError(w, http.StatusInternalServerError, "获取广告效果数据失败")
        return
    }

    // 4. 计算 CTR 并处理结果
    for i := range summaryData {
        if summaryData[i].Impressions > 0 {
            // 计算 CTR，保留几位小数
            summaryData[i].CTR = math.Round((float64(summaryData[i].Clicks)/float64(summaryData[i].Impressions))*100*100) / 100 // 保留两位小数
        } else {
            summaryData[i].CTR = 0.0
        }
    }

    // 5. 返回响应
    webutil.RespondWithJSON(w, http.StatusOK, webutil.Response{Data: summaryData})
}

// AdClickHandler 模拟处理广告点击并记录事件
func (h *Handler) AdClickHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet { // 通常点击是通过 GET 请求
        webutil.RespondWithError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法"); return
    }

    // 1. 从 URL 路径或查询参数获取 campaign_id 和 advertisement_id
    //    例如，路径: /ads/click/{campaign_id}/{advertisement_id}
    //    或查询: /ads/click?campaign_id=X&ad_id=Y
    campaignIDStr := r.PathValue("campaign_id") // Go 1.22+
    adIDStr := r.PathValue("advertisement_id")  // Go 1.22+

    campaignID, errCamp := strconv.Atoi(campaignIDStr)
    adID, errAd := strconv.Atoi(adIDStr)
    if errCamp != nil || errAd != nil || campaignID <= 0 || adID <= 0 {
        webutil.RespondWithError(w, http.StatusBadRequest, "无效的活动或广告 ID"); return
    }

    // 2. (重要) 查询活动和广告信息，特别是 user_id 和 target_url
    //    需要一个方法 GetCampaignAndAdInfo (或分开查)
    campaign, errCampGet := h.Store.GetAdCampaignByID(r.Context(), campaignID) // 需要 GetAdCampaignByID
    if errCampGet != nil { webutil.RespondWithError(w, http.StatusNotFound, "找不到活动"); return }
    ad, errAdGet := h.Store.GetAdvertisementByID(r.Context(), adID)          // 需要 GetAdvertisementByID
    if errAdGet != nil { webutil.RespondWithError(w, http.StatusNotFound, "找不到广告"); return }

    // 验证广告是否属于该活动
    if campaign.AdvertisementID != ad.ID {
         webutil.RespondWithError(w, http.StatusBadRequest, "广告与活动不匹配"); return
    }
    // 验证活动所有者与广告所有者是否一致 (可选，取决于你的数据模型)
    if campaign.UserID != ad.UserID {
        log.Printf("警告: 活动 %d (用户 %d) 与广告 %d (用户 %d) 所有者不匹配", campaignID, campaign.UserID, adID, ad.UserID)
        // 根据业务逻辑决定是否阻止
    }


    // 3. 记录 Click 事件
    clickEvent := models.AdEvent{
        EventType:       "Click",
        AdvertisementID: adID,
        CampaignID:      campaignID,
        UserID:          campaign.UserID, // 使用活动创建者的 ID
        EventTimestamp:  time.Now(),
    }
    logErr := h.Store.LogAdEvent(r.Context(), clickEvent)
    if logErr != nil {
        // 记录失败也应尝试重定向，但需记录日志
        log.Printf("!!! 记录 Click 事件失败 (但将尝试重定向): campaign %d, ad %d: %v", campaignID, adID, logErr)
    } else {
        log.Printf("记录 Click: campaign %d, ad %d", campaignID, adID)
    }

    // 4. 重定向到广告的目标 URL
    targetURL := ad.TargetURL // 从查询到的广告信息中获取
    if targetURL == "" {
        // 如果没有目标 URL，返回一个错误或默认页面
        log.Printf("广告 %d 没有目标 URL", adID)
        webutil.RespondWithError(w, http.StatusInternalServerError, "无法完成点击跳转")
        return
    }

    // 执行 302 Found 重定向
    http.Redirect(w, r, targetURL, http.StatusFound)
}

// --- RequestInvoiceHandler 用户提交开票请求 ---
func (h *Handler) RequestInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		webutil.RespondWithError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	// 1. 获取用户信息
	userClaims, ok := r.Context().Value(middleware.UserContextKey).(*auth.Claims)
	if !ok || userClaims == nil { webutil.RespondWithError(w, http.StatusUnauthorized, "无效的用户信息"); return }
	userID := userClaims.UserID

	// 2. 解码请求体
	var payload models.InvoiceRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		webutil.RespondWithError(w, http.StatusBadRequest, "请求体格式错误")
		return
	}
	defer r.Body.Close()

	// 3. 验证输入
	if payload.BillingTitle == "" { webutil.RespondWithError(w, http.StatusBadRequest, "发票抬头不能为空"); return }
	if payload.BillingAddress == "" { webutil.RespondWithError(w, http.StatusBadRequest, "邮寄地址/邮箱不能为空"); return }
	startDate, errStart := time.Parse(DateFormat, payload.StartDate)
	if errStart != nil { webutil.RespondWithError(w, http.StatusBadRequest, "无效的开始日期格式 (YYYY-MM-DD)"); return }
	endDate, errEnd := time.Parse(DateFormat, payload.EndDate)
	if errEnd != nil { webutil.RespondWithError(w, http.StatusBadRequest, "无效的结束日期格式 (YYYY-MM-DD)"); return }
	if endDate.Before(startDate) { webutil.RespondWithError(w, http.StatusBadRequest, "结束日期不能早于开始日期"); return }

	// 4. 计算开票金额 (查询指定日期范围内成功的充值总额)
    // 注意：这里我们信任 Store 层会正确处理日期范围的边界
	totalAmountCents, err := h.Store.GetSuccessfulRechargeTotalInRange(r.Context(), userID, startDate, endDate)
	if err != nil {
		log.Printf("计算用户 %d 开票金额失败 (%s to %s): %v", userID, payload.StartDate, payload.EndDate, err)
		webutil.RespondWithError(w, http.StatusInternalServerError, "计算开票金额时出错")
		return
	}

	// 5. 检查金额是否足够开票（例如，大于 0）
	if totalAmountCents <= 0 {
		webutil.RespondWithError(w, http.StatusBadRequest, "该时间段内没有可开票的成功充值记录")
		return
	}

    // (可选) 检查是否对同一时间段重复申请了发票？这需要更复杂的查询逻辑

	// 6. 创建发票请求记录
	invoiceReq := models.InvoiceRequest{
		UserID:             userID,
		Status:             "Pending", // 初始状态
		InvoicePeriodStart: startDate,
		InvoicePeriodEnd:   endDate,
		TotalAmount:        totalAmountCents,
		BillingTitle:       payload.BillingTitle,
		BillingAddress:     payload.BillingAddress,
		RequestedAt:        time.Now(),
	}
    // 处理可选的 TaxID
    if payload.TaxID != "" {
        invoiceReq.TaxID = &payload.TaxID
    }


	newInvoiceID, err := h.Store.CreateInvoiceRequest(r.Context(), invoiceReq)
	if err != nil {
		log.Printf("创建用户 %d 的发票请求失败: %v", userID, err)
		webutil.RespondWithError(w, http.StatusInternalServerError, "提交发票请求失败")
		return
	}

	// 7. 返回成功响应
    log.Printf("User %d submitted invoice request ID %d for period %s to %s", userID, newInvoiceID, payload.StartDate, payload.EndDate)
	webutil.RespondWithJSON(w, http.StatusCreated, webutil.Response{
		Message: "发票请求已提交成功",
		Data:    map[string]int64{"invoice_request_id": newInvoiceID},
	})
}


// --- GetUserInvoicesHandler 获取用户的发票请求历史 ---
func (h *Handler) GetUserInvoicesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		webutil.RespondWithError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	// 1. 获取用户信息
	userClaims, ok := r.Context().Value(middleware.UserContextKey).(*auth.Claims)
	if !ok || userClaims == nil { webutil.RespondWithError(w, http.StatusUnauthorized, "无效的用户信息"); return }
	userID := userClaims.UserID

	// 2. 解析查询参数 (可选过滤)
    filters := models.InvoiceRequestFilter{}
    query := r.URL.Query()
    if status := query.Get("status"); status != "" {
        // Add status validation if needed
        filters.Status = &status
    }
    if startDateStr := query.Get("start_date"); startDateStr != "" { // 按请求日期过滤
        t, err := time.Parse(DateFormat, startDateStr)
        if err == nil { filters.StartDate = &t } else { /* log warning or ignore */ }
    }
    if endDateStr := query.Get("end_date"); endDateStr != "" { // 按请求日期过滤
        t, err := time.Parse(DateFormat, endDateStr)
        if err == nil { filters.EndDate = &t } else { /* log warning or ignore */ }
    }


	// 3. 调用 Store 获取历史记录
	invoices, err := h.Store.GetInvoiceRequestsByUserID(r.Context(), userID, filters)
	if err != nil {
		log.Printf("获取用户 %d 发票历史失败: %v", userID, err)
		webutil.RespondWithError(w, http.StatusInternalServerError, "获取发票历史记录失败")
		return
	}

	// 4. 返回响应 (金额是分，前端可能需要转换)
	webutil.RespondWithJSON(w, http.StatusOK, webutil.Response{Data: invoices})
}

// --- GetUserInvoiceDetailsHandler 获取用户单个发票请求详情 ---
func (h *Handler) GetUserInvoiceDetailsHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        webutil.RespondWithError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法"); return
    }

    // 1. 获取用户信息
    userClaims, ok := r.Context().Value(middleware.UserContextKey).(*auth.Claims)
    if !ok || userClaims == nil { webutil.RespondWithError(w, http.StatusUnauthorized, "无效的用户信息"); return }
    userID := userClaims.UserID

    // 2. 从 URL 获取发票请求 ID (Go 1.22+)
    invoiceIDStr := r.PathValue("id")
    invoiceID, err := strconv.ParseInt(invoiceIDStr, 10, 64) // 发票 ID 是 BIGINT
    if err != nil || invoiceID <= 0 {
        webutil.RespondWithError(w, http.StatusBadRequest, "无效的发票请求 ID"); return
    }

    // 3. 调用 Store 获取详情
    invoiceDetails, err := h.Store.GetInvoiceRequestByIDAndUser(r.Context(), invoiceID, userID)
    if err != nil {
        if errors.Is(err, store.ErrNotFound) {
            webutil.RespondWithError(w, http.StatusNotFound, "找不到指定的发票请求或无权访问")
        } else {
            log.Printf("获取用户 %d 的发票详情 %d 失败: %v", userID, invoiceID, err)
            webutil.RespondWithError(w, http.StatusInternalServerError, "获取发票详情失败")
        }
        return
    }

    // 4. 返回响应
    webutil.RespondWithJSON(w, http.StatusOK, webutil.Response{Data: invoiceDetails})
}
