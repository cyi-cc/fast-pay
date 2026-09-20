// Package guard 提供登录态与管理员鉴权守卫。
package guard

import (
	"context"
	"errors"
	"time"

	"github.com/cyi-cc/fun"

	"epay/database"
	"epay/db"
	"epay/domain"
)

// 错误码约定：4010 未登录 / 4011 会话失效或被封禁 / 4005 权限不足
func errUnauthored() error { return fun.Error(4010, "请先登录") }

// publicEndpoints 显式端点策略表（缺省拒绝）：绑定 AuthGuard 的服务中无需登录即可调用的端点。
var publicEndpoints = map[string]bool{
	"AuthSvc.Register": true,
	"AuthSvc.Login":    true,
	"AuthSvc.Logout":   true,
}

// AuthGuard 校验 state.token 对应的会话，注入 uid/role/username。
type AuthGuard struct {
	Db *database.Database
}

func (g *AuthGuard) Guard(ctx fun.Ctx) error {
	if publicEndpoints[ctx.ServiceName+"."+ctx.MethodName] {
		return nil
	}
	token := ctx.State["token"]
	if token == "" {
		return errUnauthored()
	}
	u, err := g.Db.Q.GetSessionUser(context.Background(), db.GetSessionUserParams{
		Token: token, Now: time.Now().Unix(),
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fun.Error(5000, "服务繁忙，请重试")
		}
		return fun.Error(4011, "登录已失效，请重新登录")
	}
	ctx.RequestCtx.SetUserValue("uid", u.ID)
	ctx.RequestCtx.SetUserValue("role", u.Role)
	ctx.RequestCtx.SetUserValue("username", u.Username)
	return nil
}

// AdminGuard 在 AuthGuard 之后执行，要求管理员角色。
type AdminGuard struct {
}

func (g *AdminGuard) Guard(ctx fun.Ctx) error {
	role, _ := ctx.RequestCtx.UserValue("role").(int64)
	if role != domain.RoleAdmin {
		return fun.Error(4005, "需要管理员权限")
	}
	return nil
}

// UID 从请求上下文取当前登录用户 id（AuthGuard 已保证存在）。
func UID(ctx fun.Ctx) int64 {
	v, _ := ctx.RequestCtx.UserValue("uid").(int64)
	return v
}
