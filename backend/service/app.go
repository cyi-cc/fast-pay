package service

import (
	"context"
	"strings"
	"time"

	"github.com/cyi-cc/fun"

	"epay/database"
	"epay/db"
	"epay/domain"
	"epay/guard"
)

// AppSvc 商户应用（PID/密钥）管理服务。
type AppSvc struct {
	fun.Ctx
	Db *database.Database
}

type CreateAppDto struct {
	Name      string
	NotifyUrl *string
	Rate      *int64
}

type UpdateAppDto struct {
	Id        int64
	Name      string
	NotifyUrl string
	Rate      int64
	Status    int64
}

type ResetKeyDto struct {
	Id int64
}

type DeleteAppDto struct {
	Id int64
}

type ResetKeyResult struct {
	AppKey string
}

// List 当前商户的全部应用。
func (s *AppSvc) List() ([]AppView, error) {
	apps, err := s.Db.Q.ListAppsByUser(context.Background(), guard.UID(s.Ctx))
	if err != nil {
		return []AppView{}, fun.Error(5000, "查询应用失败")
	}
	views := make([]AppView, 0, len(apps))
	for _, a := range apps {
		views = append(views, toAppView(a))
	}
	return views, nil
}

// Create 新建应用：PID 自增分配（自 1001 起），密钥随机生成。
func (s *AppSvc) Create(dto CreateAppDto) (AppView, error) {
	name := strings.TrimSpace(dto.Name)
	if name == "" || len(name) > 50 {
		return AppView{}, errParam("应用名称需为 1-50 个字符")
	}
	rate := int64(20)
	if dto.Rate != nil {
		rate = *dto.Rate
	}
	if rate < 0 || rate > 1000 {
		return AppView{}, errParam("费率需在 0-1000（千分比）之间")
	}
	notifyUrl := ""
	if dto.NotifyUrl != nil {
		notifyUrl = strings.TrimSpace(*dto.NotifyUrl)
		if notifyUrl != "" && !strings.HasPrefix(notifyUrl, "http") {
			return AppView{}, errParam("通知地址需以 http 开头")
		}
	}

	ctx := context.Background()
	now := time.Now().Unix()
	var view AppView
	err := s.Db.Tx(ctx, func(q *db.Queries) error {
		maxRaw, _ := q.MaxAppId(ctx)
		maxID := domain.AnyI64(maxRaw)
		if maxID < 1000 {
			maxID = 1000
		}
		id, err := q.CreateApp(ctx, db.CreateAppParams{
			UserID: guard.UID(s.Ctx), Name: name, AppID: maxID + 1, AppKey: domain.GenKey(),
			Rate: rate, NotifyUrl: notifyUrl, Now: now,
		})
		if err != nil {
			return fun.Error(5000, "创建应用失败")
		}
		a, err := q.GetApp(ctx, id)
		if err != nil {
			return fun.Error(5000, "读取应用失败")
		}
		view = toAppView(a)
		return nil
	})
	return view, err
}

// Update 全量更新应用信息（密钥除外）。
func (s *AppSvc) Update(dto UpdateAppDto) error {
	name := strings.TrimSpace(dto.Name)
	if name == "" || len(name) > 50 {
		return errParam("应用名称需为 1-50 个字符")
	}
	if dto.Rate < 0 || dto.Rate > 1000 {
		return errParam("费率需在 0-1000（千分比）之间")
	}
	if dto.Status != 0 && dto.Status != 1 {
		return errParam("状态取值不合法")
	}
	if dto.NotifyUrl != "" && !strings.HasPrefix(dto.NotifyUrl, "http") {
		return errParam("通知地址需以 http 开头")
	}
	rows, err := s.Db.Q.UpdateApp(context.Background(), db.UpdateAppParams{
		Name: name, NotifyUrl: dto.NotifyUrl, Rate: dto.Rate, Status: dto.Status,
		Now: time.Now().Unix(), ID: dto.Id, UserID: guard.UID(s.Ctx),
	})
	if err != nil {
		return fun.Error(5000, "更新应用失败")
	}
	if rows == 0 {
		return errNotFound("应用不存在")
	}
	return nil
}

// ResetKey 重置应用密钥。
func (s *AppSvc) ResetKey(dto ResetKeyDto) (ResetKeyResult, error) {
	app, err := s.ownApp(dto.Id)
	if err != nil {
		return ResetKeyResult{}, err
	}
	key := domain.GenKey()
	if err := s.Db.Q.ResetAppKey(context.Background(), db.ResetAppKeyParams{
		AppKey: key, Now: time.Now().Unix(), ID: app.ID,
	}); err != nil {
		return ResetKeyResult{}, fun.Error(5000, "重置密钥失败")
	}
	return ResetKeyResult{AppKey: key}, nil
}

// Delete 删除应用（历史订单保留）。
func (s *AppSvc) Delete(dto DeleteAppDto) error {
	app, err := s.ownApp(dto.Id)
	if err != nil {
		return err
	}
	if _, err := s.Db.Q.DeleteApp(context.Background(), db.DeleteAppParams{ID: app.ID, UserID: guard.UID(s.Ctx)}); err != nil {
		return fun.Error(5000, "删除应用失败")
	}
	return nil
}

func (s *AppSvc) ownApp(id int64) (db.App, error) {
	app, err := s.Db.Q.GetApp(context.Background(), id)
	if err != nil {
		return db.App{}, errNotFound("应用不存在")
	}
	if app.UserID != guard.UID(s.Ctx) {
		return db.App{}, errDenied("无权操作该应用")
	}
	return app, nil
}
