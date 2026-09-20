// Fast Pay后端入口：装配配置/数据库/服务/守卫/协议路由并启动 fun 框架。
// 注意：服务注册必须直接调用 f.BindService（fun gen 靠扫描该调用生成前端客户端）。
package main

import (
	"log"
	"strings"

	"github.com/cyi-cc/fun"

	"epay/config"
	"epay/database"
	"epay/guard"
	"epay/route"
	"epay/service"
	"epay/upstream"
)

func main() {
	f := fun.GetFun()

	// 单例装配（顺序：配置 → 数据库 → 通知器）
	cfg, err := fun.Wired[config.Config]()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	if _, err = fun.Wired[database.Database](); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	if _, err = fun.Wired[service.Notifier](); err != nil {
		log.Fatalf("初始化通知器失败: %v", err)
	}
	if _, err = fun.Wired[upstream.Client](); err != nil {
		log.Fatalf("初始化上游客户端失败: %v", err)
	}
	if _, err = fun.Wired[service.UpstreamPoller](); err != nil {
		log.Fatalf("初始化上游轮询器失败: %v", err)
	}

	// 公开服务（AuthSvc 的 Register/Login/Logout 在守卫策略表中放行）
	if err = f.BindService(&service.AuthSvc{}, &guard.AuthGuard{}); err != nil {
		log.Fatalf("注册 AuthSvc 失败: %v", err)
	}
	if err = f.BindService(&service.CashierSvc{}); err != nil {
		log.Fatalf("注册 CashierSvc 失败: %v", err)
	}
	// 商户服务（需登录）
	if err = f.BindService(&service.UserSvc{}, &guard.AuthGuard{}); err != nil {
		log.Fatalf("注册 UserSvc 失败: %v", err)
	}
	if err = f.BindService(&service.AppSvc{}, &guard.AuthGuard{}); err != nil {
		log.Fatalf("注册 AppSvc 失败: %v", err)
	}
	if err = f.BindService(&service.OrderSvc{}, &guard.AuthGuard{}); err != nil {
		log.Fatalf("注册 OrderSvc 失败: %v", err)
	}
	if err = f.BindService(&service.SettleSvc{}, &guard.AuthGuard{}); err != nil {
		log.Fatalf("注册 SettleSvc 失败: %v", err)
	}
	// 管理员服务
	if err = f.BindService(&service.AdminSvc{}, &guard.AuthGuard{}, &guard.AdminGuard{}); err != nil {
		log.Fatalf("注册 AdminSvc 失败: %v", err)
	}
	// 上游对接配置（管理员）
	if err = f.BindService(&service.UpstreamSvc{}, &guard.AuthGuard{}, &guard.AdminGuard{}); err != nil {
		log.Fatalf("注册 UpstreamSvc 失败: %v", err)
	}

	// 易支付协议路由（submit.php / mapi.php / api.php / health）
	epay, err := fun.Wired[route.Epay]()
	if err != nil {
		log.Fatalf("初始化协议路由失败: %v", err)
	}
	if err = epay.Bind(f); err != nil {
		log.Fatalf("绑定协议路由失败: %v", err)
	}

	// 跨域白名单（开发环境默认走 Vite 代理，不依赖此项）
	if origins := splitOrigins(cfg.CORSOrigins); len(origins) > 0 {
		f.CORS(origins...)
	}

	log.Printf("Fast Pay启动：%s → http://127.0.0.1:%d（RPC 端点 POST /cell）", cfg.SiteName, cfg.Port)
	f.Start(cfg.Port) // 阻塞
}

func splitOrigins(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
