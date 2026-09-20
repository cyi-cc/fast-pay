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

// SettleSvc 商户结算（提现）服务。
type SettleSvc struct {
	fun.Ctx
	Db *database.Database
}

type ApplySettleDto struct {
	Amount  int64 // 分
	Account string
	PayType string // alipay / bank
}

type PageDto struct {
	Page     int64
	PageSize int64
}

type SettleListResult struct {
	Total int64
	Items []SettleView
}

type BalanceLogListResult struct {
	Total int64
	Items []BalanceLogView
}

// Apply 申请提现：校验最低金额与余额，事务扣款并生成结算单。
func (s *SettleSvc) Apply(dto ApplySettleDto) error {
	account := strings.TrimSpace(dto.Account)
	if account == "" || len(account) > 100 {
		return errParam("收款账号不能为空")
	}
	if dto.PayType != "alipay" && dto.PayType != "bank" {
		return errParam("收款方式仅支持 alipay / bank")
	}
	min := s.Db.SettingInt("min_settle_fen", 1000)
	if dto.Amount < min {
		return errParam("提现金额不能低于 " + domain.FenToYuan(min) + " 元")
	}

	ctx := context.Background()
	uid := guard.UID(s.Ctx)
	now := time.Now().Unix()
	return s.Db.Tx(ctx, func(q *db.Queries) error {
		u, err := q.GetUser(ctx, uid)
		if err != nil {
			return errNotFound("用户不存在")
		}
		if u.Balance < dto.Amount {
			return errParam("余额不足")
		}
		bal, err := q.AddUserBalance(ctx, db.AddUserBalanceParams{Delta: -dto.Amount, Now: now, ID: uid})
		if err != nil {
			return fun.Error(5000, "扣款失败")
		}
		sid, err := q.CreateSettle(ctx, db.CreateSettleParams{
			UserID: uid, Amount: dto.Amount, Fee: 0, Account: account, PayType: dto.PayType, Now: now,
		})
		if err != nil {
			return fun.Error(5000, "创建结算单失败")
		}
		return q.CreateBalanceLog(ctx, db.CreateBalanceLogParams{
			UserID: uid, Type: domain.BalanceLogWithdraw, Amount: -dto.Amount,
			BalanceAfter: bal, RefID: sid, Note: "申请提现", Now: now,
		})
	})
}

// List 我的结算记录。
func (s *SettleSvc) List(dto PageDto) (SettleListResult, error) {
	off, lim := clampPage(dto.Page, dto.PageSize)
	ctx := context.Background()
	rows, err := s.Db.Q.ListSettlesByUser(ctx, db.ListSettlesByUserParams{
		UserID: guard.UID(s.Ctx), Lim: lim, Off: off,
	})
	if err != nil {
		return SettleListResult{}, fun.Error(5000, "查询结算记录失败")
	}
	total, err := s.Db.Q.CountSettlesByUser(ctx, guard.UID(s.Ctx))
	if err != nil {
		return SettleListResult{}, fun.Error(5000, "统计结算记录失败")
	}
	items := make([]SettleView, 0, len(rows))
	for _, r := range rows {
		items = append(items, toSettleView(r))
	}
	return SettleListResult{Total: total, Items: items}, nil
}

// Logs 资金变动流水。
func (s *SettleSvc) Logs(dto PageDto) (BalanceLogListResult, error) {
	off, lim := clampPage(dto.Page, dto.PageSize)
	ctx := context.Background()
	uid := guard.UID(s.Ctx)
	rows, err := s.Db.Q.ListBalanceLogsByUser(ctx, db.ListBalanceLogsByUserParams{
		UserID: uid, Lim: lim, Off: off,
	})
	if err != nil {
		return BalanceLogListResult{}, fun.Error(5000, "查询流水失败")
	}
	total, err := s.Db.Q.CountBalanceLogsByUser(ctx, uid)
	if err != nil {
		return BalanceLogListResult{}, fun.Error(5000, "统计流水失败")
	}
	items := make([]BalanceLogView, 0, len(rows))
	for _, r := range rows {
		items = append(items, BalanceLogView{
			Id: r.ID, Type: r.Type, Amount: r.Amount, BalanceAfter: r.BalanceAfter,
			Note: r.Note, CreatedAt: r.CreatedAt,
		})
	}
	return BalanceLogListResult{Total: total, Items: items}, nil
}
