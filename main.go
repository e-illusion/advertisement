package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/rs/cors"
	_ "github.com/go-sql-driver/mysql"

	// --- 导入内部包 ---
	"advertisement/internal/handlers"   // 替换 "your_module_name"
	"advertisement/internal/middleware" // 替换 "your_module_name"
	"advertisement/internal/store"
)

// initDB 函数保持不变
func initDB() (*sql.DB, error){
    // ... (和之前一样) ...
    dsn := "root:123456@tcp(127.0.0.1:3306)/advertisement?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("数据库配置错误: %w", err)
	}
	err = db.Ping()
	if err != nil {
		db.Close() // 如果 ping 失败，关闭连接
		return nil, fmt.Errorf("无法连接到数据库: %w", err)
	}
	log.Println("数据库连接成功!")
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	return db, nil // 返回 db 连接
}

func main() {
	// --- 初始化数据库连接 ---
	db, err := initDB()
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	// 使用 defer db.Close() 确保在 main 退出时关闭数据库连接
	// 注意：这只在程序正常或 panic 退出时有效，如果是 kill -9 则无效
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("关闭数据库连接时出错: %v", err)
		} else {
			log.Println("数据库连接已关闭。")
		}
	}()

	
	// --- 创建 Store 实例 ---
	dataStore := store.NewDBStore(db) // 使用 db 创建具体的 DBStore

	// --- 创建 Handler 实例，注入 Store ---
	h := handlers.NewHandler(dataStore) // 将 Store 实例传递给 Handler

// --- 定义需要认证和授权的 Handler ---
	// 基础认证
	authHandler := middleware.AuthMiddleware

	// 管理员认证 (先认证 Auth，再检查 Admin)
    // 更正：应该是先 Auth 再 Admin！ Auth 负责解析 token 放入 context，Admin 负责从 context 取出并检查 Role
    adminRequiredHandler := func(next http.Handler) http.Handler {
        return authHandler(middleware.AdminMiddleware(next)) // 正确顺序: Auth -> Admin -> Handler
    }


	// --- 注册路由 (使用 Go 1.22+ Mux) ---
	mux := http.NewServeMux()

	// 公开接口
	mux.HandleFunc("POST /register", h.RegisterHandler)
	mux.HandleFunc("POST /login", h.LoginHandler)  
	mux.HandleFunc("GET /get-ad", h.GetAdHandler) // 广告位获取广告（会记录Impression）
	// --- (可选/模拟) 广告点击处理 ---
	// 注意：这个接口通常不需要用户认证
    mux.HandleFunc("GET /ads/click/{campaign_id}/{advertisement_id}", h.AdClickHandler)


	// 需要普通认证的接口
	//mux.Handle("GET /get-ad", authHandler(http.HandlerFunc(h.GetAdHandler)))
	mux.Handle("POST /ads", authHandler(http.HandlerFunc(h.SubmitAdHandler)))
	mux.Handle("GET /my-ads", authHandler(http.HandlerFunc(h.GetUserAdsHandler)))
	mux.Handle("POST /campaigns", authHandler(http.HandlerFunc(h.RequestCampaignHandler)))
    // --- 新增：充值和余额接口 ---
    mux.Handle("POST /recharge", authHandler(http.HandlerFunc(h.RechargeHandler)))
    mux.Handle("GET /balance", authHandler(http.HandlerFunc(h.GetBalanceHandler)))
    mux.Handle("GET /recharges", authHandler(http.HandlerFunc(h.GetRechargeHistoryHandler)))
	// --- 新增：用户管理自己的广告活动 ---
	mux.Handle("GET /my-campaigns", authHandler(http.HandlerFunc(h.GetUserCampaignsHandler)))
	mux.Handle("GET /my-campaigns/{id}", authHandler(http.HandlerFunc(h.GetUserCampaignDetailsHandler)))
	mux.Handle("PATCH /my-campaigns/{id}/cancel", authHandler(http.HandlerFunc(h.CancelCampaignHandler)))
	// --- 新增：用户查看广告效果 ---
	mux.Handle("GET /my-performance", authHandler(http.HandlerFunc(h.GetAdPerformanceHandler)))
	// --- 新增：发票相关接口 ---
	mux.Handle("POST /invoices/request", authHandler(http.HandlerFunc(h.RequestInvoiceHandler)))
	mux.Handle("GET /invoices", authHandler(http.HandlerFunc(h.GetUserInvoicesHandler)))
	mux.Handle("GET /invoices/{id}", authHandler(http.HandlerFunc(h.GetUserInvoiceDetailsHandler)))
 
    // --- 新增：管理员获取待审核列表的接口 ---
    mux.Handle("GET /admin/ads/pending", adminRequiredHandler(http.HandlerFunc(h.AdminGetPendingAdsHandler)))
    mux.Handle("GET /admin/campaigns/pending", adminRequiredHandler(http.HandlerFunc(h.AdminGetPendingCampaignsHandler)))
	// 需要管理员认证的接口
	mux.Handle("PATCH /ads/{id}/status", adminRequiredHandler(http.HandlerFunc(h.ReviewAdHandler)))
	mux.Handle("PATCH /campaigns/{id}/status", adminRequiredHandler(http.HandlerFunc(h.ReviewCampaignHandler)))
    // --- (可选) 管理员处理发票接口 ---
    // mux.Handle("PATCH /admin/invoices/{id}/status", adminRequiredHandler(http.HandlerFunc(h.AdminUpdateInvoiceStatusHandler))) // 需要实现 AdminUpdateInvoiceStatusHandler
	
	c := cors.New(cors.Options{
        AllowedOrigins: []string{"http://localhost:5173"}, // 允许来自前端的地址
        AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}, // 允许的 HTTP 方法
        AllowedHeaders: []string{"Authorization", "Content-Type"}, // 允许的请求头
        AllowCredentials: true, // 允许携带认证信息
        Debug: true, // 开启 Debug 模式，可以在后端终端看到 CORS 相关的日志
    })

    // 使用 CORS 中间件包裹您的 Mux
    handler := c.Handler(mux)
	
	// 启动服务器
	port := ":8080"
	log.Printf("服务器启动，监听端口 %s", port)
	log.Println("可用接口:")
	log.Printf("  POST http://localhost%s/register (公开)", port)
	log.Printf("  POST http://localhost%s/login    (公开)", port)
	log.Printf("  GET  http://localhost%s/get-ad   (公开, 广告位获取广告, 记录 Impression)", port)
	log.Printf("  GET  http://localhost%s/ads/click/{cid}/{aid} (公开, 模拟广告点击, 记录 Click)", port)
	//log.Printf("  GET  http://localhost%s/get-ad  (需要认证)", port)
	log.Printf("  POST http://localhost%s/ads      (需要认证)", port)
	log.Printf("  GET  http://localhost%s/my-ads  (需要认证)", port)
	log.Printf("  POST http://localhost%s/campaigns (需要认证)", port)
    log.Printf("  POST http://localhost%s/recharge (需要认证)", port) // <-- 更新日志
    log.Printf("  GET  http://localhost%s/balance  (需要认证)", port) // <-- 更新日志
	log.Printf("  GET  http://localhost%s/recharges (需要认证)", port) // <-- 更新日志
	log.Printf("  GET  http://localhost%s/my-performance   (需要认证, 用户查看广告效果)", port) // <-- 更新日志
	log.Printf("  POST http://localhost%s/invoices/request (需要认证, 用户请求开票)", port) // <-- 更新日志
    log.Printf("  GET  http://localhost%s/invoices        (需要认证, 用户查看发票历史)", port) // <-- 更新日志
    log.Printf("  GET  http://localhost%s/invoices/{id}   (需要认证, 用户查看发票详情)", port) // <-- 更新日志
	log.Printf("  GET  http://localhost%s/my-campaigns (需要认证, 用户查看自己的活动列表)", port) // <-- 更新日志
    log.Printf("  GET  http://localhost%s/my-campaigns/{id} (需要认证, 用户查看活动详情)", port) // <-- 更新日志
    log.Printf("  PATCH http://localhost%s/my-campaigns/{id}/cancel (需要认证, 用户取消活动)", port) // <-- 更新日志
	log.Printf("  PATCH http://localhost%s/ads/{id}/status (需要管理员认证)", port)
	log.Printf("  PATCH http://localhost%s/campaigns/{id}/status (需要管理员认证)", port)
	log.Printf("  GET  http://localhost%s/admin/ads/pending (需要管理员认证, 获取待审核广告)", port)
    log.Printf("  GET  http://localhost%s/admin/campaigns/pending (需要管理员认证, 获取待审核活动)", port)

	serveErr := http.ListenAndServe(port, handler) // <-- 修改为使用包裹后的 handler
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		log.Fatalf("服务器启动失败: %v", serveErr)
	}
	log.Println("服务器已停止。")
}
