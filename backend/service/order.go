package service

import (
	"context"
	"time"

	"github.com/cyi-cc/fun"

	"epay/database"
	"epay/db"
	"epay/domain"
	"epay/guard"
)

// OrderSvc 商户订单服务。
type OrderSvc struct {
	fun.Ctx
	Db *database.Database
}

type ListOrdersDto struct {
	Page     int64
	PageSize int64
	Status   *int64
	Channel  *string
	Kw       *string
	From     *int64
	To       *int64
}

type StatsView struct {
	TodayCount  int64
	TodayMoney  int64
	TodayIncome int64
	TotalCount  int64
	TotalIncome int64
}

type RecentDto struct {
	Limit int64
}

// List 商户订单分页查询。
func (s *OrderSvc) List(dto ListOrdersDto) (OrderPage, error) {
	off, lim := clampPage(dto.Page, dto.PageSize)
	arg := db.ListOrdersByUserParams{
		UserID: guard.UID(s.Ctx), Status: nint64(dto.Status), Channel: nstr(dto.Channel),
		Kw: nstr(dto.Kw), TFrom: nint64(dto.From), TTo: nint64(dto.To), Off: off, Lim: lim,
	}
	rows, err := s.Db.Q.ListOrdersByUser(context.Background(), arg)
	if err != nil {
		return OrderPage{}, fun.Error(5000, "查询订单失败")
	}
	total, err := s.Db.Q.CountOrdersByUser(context.Background(), db.CountOrdersByUserParams{
		UserID: arg.UserID, Status: arg.Status, Channel: arg.Channel,
		Kw: arg.Kw, TFrom: arg.TFrom, TTo: arg.TTo,
	})
	if err != nil {
		return OrderPage{}, fun.Error(5000, "统计订单失败")
	}
	items := make([]OrderView, 0, len(rows))
	for _, r := range rows {
		items = append(items, toOrderView(r))
	}
	return OrderPage{Total: total, Items: items}, nil
}

// Stats 今日与累计经营统计。
func (s *OrderSvc) Stats() (StatsView, error) {
	ctx := context.Background()
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()

	uid := guard.UID(s.Ctx)
	today, err := s.Db.Q.OrderStatsByUser(ctx, db.OrderStatsByUserParams{
		UserID: uid, TFrom: dayStart, TTo: now.Unix() + 1,
	})
	if err != nil {
		return StatsView{}, fun.Error(5000, "统计失败")
	}
	total, err := s.Db.Q.OrderStatsByUser(ctx, db.OrderStatsByUserParams{
		UserID: uid, TFrom: 0, TTo: now.Unix() + 1,
	})
	if err != nil {
		return StatsView{}, fun.Error(5000, "统计失败")
	}
	return StatsView{
		TodayCount:  today.Cnt,
		TodayMoney:  domain.AnyI64(today.TotalMoney),
		TodayIncome: domain.AnyI64(today.TotalMoney) - domain.AnyI64(today.TotalFee),
		TotalCount:  total.Cnt,
		TotalIncome: domain.AnyI64(total.TotalMoney),
	}, nil
}

// Recent 最近订单（仪表盘用）。
func (s *OrderSvc) Recent(dto RecentDto) ([]OrderView, error) {
	lim := dto.Limit
	if lim < 1 || lim > 50 {
		lim = 10
	}
	rows, err := s.Db.Q.ListRecentOrdersByUser(context.Background(), db.ListRecentOrdersByUserParams{
		UserID: guard.UID(s.Ctx), Lim: lim,
	})
	if err != nil {
		return []OrderView{}, fun.Error(5000, "查询订单失败")
	}
	items := make([]OrderView, 0, len(rows))
	for _, r := range rows {
		items = append(items, OrderView{
			Id: r.ID, TradeNo: r.TradeNo, OutTradeNo: r.OutTradeNo, Channel: r.Channel,
			Subject: r.Subject, Money: r.Money, Fee: r.Fee, Status: r.Status,
			Notified: r.Notified, PaidAt: r.PaidAt, ExpiredAt: r.ExpiredAt,
			CreatedAt: r.CreatedAt, AppName: r.AppName, Pid: r.Pid,
		})
	}
	return items, nil
}
