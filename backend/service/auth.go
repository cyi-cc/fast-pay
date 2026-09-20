package service

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/cyi-cc/fun"

	"epay/database"
	"epay/db"
	"epay/domain"
	"epay/guard"
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]{3,20}$`)

// AuthSvc 认证服务（公开）。
type AuthSvc struct {
	fun.Ctx
	Db *database.Database
}

type RegisterDto struct {
	Username string
	Password string
	Email    *string
}

type LoginDto struct {
	Username string
	Password string
}

type LoginResult struct {
	Token string
	User  UserView
}

// Register 注册商户账号，自动开通默认应用并登录。
func (s *AuthSvc) Register(dto RegisterDto) (LoginResult, error) {
	dto.Username = strings.TrimSpace(dto.Username)
	if !usernamePattern.MatchString(dto.Username) {
		return LoginResult{}, errParam("用户名需为 3-20 位字母、数字或下划线")
	}
	if len(dto.Password) < 6 || len(dto.Password) > 64 {
		return LoginResult{}, errParam("密码长度需在 6-64 位之间")
	}
	email := ""
	if dto.Email != nil && strings.TrimSpace(*dto.Email) != "" {
		email = strings.TrimSpace(*dto.Email)
		if !strings.Contains(email, "@") {
			return LoginResult{}, errParam("邮箱格式不正确")
		}
	}

	ctx := context.Background()
	now := time.Now().Unix()
	hash, err := domain.HashPassword(dto.Password)
	if err != nil {
		return LoginResult{}, fun.Error(5000, "密码加密失败")
	}

	var view UserView
	err = s.Db.Tx(ctx, func(q *db.Queries) error {
		if _, err := q.GetUserByUsername(ctx, dto.Username); err == nil {
			return errParam("用户名已被注册")
		}
		uid, err := q.CreateUser(ctx, db.CreateUserParams{
			Username: dto.Username, PasswordHash: hash, Email: email, Role: domain.RoleUser, Now: now,
		})
		if err != nil {
			return fun.Error(5000, "创建用户失败")
		}
		// 默认应用
		maxRaw, _ := q.MaxAppId(ctx)
		maxID := domain.AnyI64(maxRaw)
		if maxID < 1000 {
			maxID = 1000
		}
		if _, err := q.CreateApp(ctx, db.CreateAppParams{
			UserID: uid, Name: "默认应用", AppID: maxID + 1, AppKey: domain.GenKey(),
			Rate: 20, NotifyUrl: "", Now: now,
		}); err != nil {
			return fun.Error(5000, "创建默认应用失败")
		}
		u, err := q.GetUser(ctx, uid)
		if err != nil {
			return fun.Error(5000, "读取用户失败")
		}
		view = toUserView(u)
		return nil
	})
	if err != nil {
		return LoginResult{}, err
	}

	token, err := s.issueSession(view.Id)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: token, User: view}, nil
}

// Login 登录并颁发会话令牌。
func (s *AuthSvc) Login(dto LoginDto) (LoginResult, error) {
	u, err := s.Db.Q.GetUserByUsername(context.Background(), strings.TrimSpace(dto.Username))
	if err != nil || !domain.CheckPassword(u.PasswordHash, dto.Password) {
		return LoginResult{}, fun.Error(4002, "用户名或密码错误")
	}
	if u.Status != 1 {
		return LoginResult{}, fun.Error(4011, "账号已被封禁")
	}
	token, err := s.issueSession(u.ID)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: token, User: toUserView(u)}, nil
}

// Logout 注销当前会话。
func (s *AuthSvc) Logout() error {
	if token := s.Ctx.State["token"]; token != "" {
		_ = s.Db.Q.DeleteSession(context.Background(), token)
	}
	return nil
}

type RefreshResult struct {
	Token string
}

// Refresh 显式轮换当前会话令牌：前端在令牌签发满 2 小时后由请求拦截器后台调用。
// 旧令牌保留 5 分钟宽限期供在途请求完成；宽限期内重复刷新返回同一枚新令牌（幂等）。
func (s *AuthSvc) Refresh() (RefreshResult, error) {
	oldToken := s.Ctx.State["token"]
	if oldToken == "" {
		return RefreshResult{}, fun.Error(4010, "请先登录")
	}
	token, err := s.Db.RefreshSession(oldToken, domain.GenKey(), time.Now().Unix())
	if err != nil {
		return RefreshResult{}, fun.Error(4011, "登录已失效，请重新登录")
	}
	return RefreshResult{Token: token}, nil
}

// Me 当前登录用户信息。
func (s *AuthSvc) Me() (UserView, error) {
	u, err := s.Db.Q.GetUser(context.Background(), guard.UID(s.Ctx))
	if err != nil {
		return UserView{}, errNotFound("用户不存在")
	}
	return toUserView(u), nil
}

func (s *AuthSvc) issueSession(uid int64) (string, error) {
	now := time.Now().Unix()
	token := domain.GenKey()
	if err := s.Db.Q.CreateSession(context.Background(), db.CreateSessionParams{
		Token: token, UserID: uid, ExpiresAt: now + 24*60*60, Now: now,
	}); err != nil {
		return "", fun.Error(5000, "创建会话失败")
	}
	return token, nil
}

// UserSvc 商户账号服务（需登录）。
type UserSvc struct {
	fun.Ctx
	Db *database.Database
}

type ChangePasswordDto struct {
	OldPassword string
	NewPassword string
}

// ChangePassword 修改密码并注销其他会话。
func (s *UserSvc) ChangePassword(dto ChangePasswordDto) error {
	uid := guard.UID(s.Ctx)
	u, err := s.Db.Q.GetUser(context.Background(), uid)
	if err != nil {
		return errNotFound("用户不存在")
	}
	if !domain.CheckPassword(u.PasswordHash, dto.OldPassword) {
		return errParam("原密码错误")
	}
	if len(dto.NewPassword) < 6 || len(dto.NewPassword) > 64 {
		return errParam("新密码长度需在 6-64 位之间")
	}
	hash, err := domain.HashPassword(dto.NewPassword)
	if err != nil {
		return fun.Error(5000, "密码加密失败")
	}
	ctx := context.Background()
	now := time.Now().Unix()
	if err := s.Db.Q.UpdatePassword(ctx, db.UpdatePasswordParams{PasswordHash: hash, Now: now, ID: uid}); err != nil {
		return fun.Error(5000, "更新密码失败")
	}
	if err := s.Db.Q.DeleteUserSessions(ctx, uid); err != nil {
		return fun.Error(5000, "注销会话失败")
	}
	return nil
}
