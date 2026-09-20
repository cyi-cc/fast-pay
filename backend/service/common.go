// Package service 实现全部 RPC 业务服务。
package service

import (
	"github.com/cyi-cc/fun"

	"epay/db"
	"epay/domain"
)

// ---------- 视图 DTO（fun 协议要求：定宽整型/字符串/bool/结构体/切片/指针） ----------

type UserView struct {
	Id        int64
	Username  string
	Email     string
	Role      int64
	Balance   int64
	CreatedAt int64
}

type AppView struct {
	Id        int64
	Name      string
	Pid       int64
	AppKey    string
	Rate      int64
	Status    int64
	NotifyUrl string
	CreatedAt int64
}

type OrderView struct {
	Id             int64
	TradeNo        string
	OutTradeNo     string
	Channel        string
	Subject        string
	Money          int64
	Fee            int64
	Status         int64
	Notified       int64
	NotifyAttempts int64
	NotifyUrl      string
	PaidAt         int64
	ExpiredAt      int64
	CreatedAt      int64
	AppName        string
	Pid            int64
}

type AdminOrderView struct {
	OrderView
	Merchant  string
	NotifyUrl string
	ReturnUrl string
}

type OrderPage struct {
	Total int64
	Items []OrderView
}

type AdminOrderPage struct {
	Total int64
	Items []AdminOrderView
}

type SettleView struct {
	Id        int64
	Amount    int64
	Fee       int64
	Account   string
	PayType   string
	Status    int64
	Remark    string
	CreatedAt int64
	HandledAt int64
}

type SettlePage struct {
	Total int64
	Items []SettleView
}

type AdminSettleView struct {
	SettleView
	Merchant string
}

type AdminSettlePage struct {
	Total int64
	Items []AdminSettleView
}

type BalanceLogView struct {
	Id           int64
	Type         int64
	Amount       int64
	BalanceAfter int64
	Note         string
	CreatedAt    int64
}

type BalanceLogPage struct {
	Total int64
	Items []BalanceLogView
}

type SettingItem struct {
	Key   string
	Value string
}

// ---------- 通用辅助 ----------

// errParam 参数错误
func errParam(msg string) error { return fun.Error(4001, msg) }

// errNotFound 资源不存在
func errNotFound(msg string) error { return fun.Error(4004, msg) }

// errDenied 权限不足
func errDenied(msg string) error { return fun.Error(4005, msg) }

// clampPage 规整分页参数
func clampPage(page, size int64) (offset, limit int64) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}
	return (page - 1) * size, size
}

// nint64 指针转可空查询参数
func nint64(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

// nstr 指针转可空查询参数
func nstr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func toUserView(u db.User) UserView {
	return UserView{
		Id: u.ID, Username: u.Username, Email: u.Email,
		Role: u.Role, Balance: u.Balance, CreatedAt: u.CreatedAt,
	}
}

func toAppView(a db.App) AppView {
	return AppView{
		Id: a.ID, Name: a.Name, Pid: a.AppID, AppKey: a.AppKey,
		Rate: a.Rate, Status: a.Status, NotifyUrl: a.NotifyUrl, CreatedAt: a.CreatedAt,
	}
}

func toOrderView(r db.ListOrdersByUserRow) OrderView {
	return OrderView{
		Id: r.ID, TradeNo: r.TradeNo, OutTradeNo: r.OutTradeNo, Channel: r.Channel,
		Subject: r.Subject, Money: r.Money, Fee: r.Fee, Status: r.Status,
		Notified: r.Notified, NotifyAttempts: r.NotifyAttempts, NotifyUrl: r.NotifyUrl,
		PaidAt: r.PaidAt, ExpiredAt: r.ExpiredAt,
		CreatedAt: r.CreatedAt, AppName: r.AppName, Pid: r.Pid,
	}
}

func toSettleView(s db.SettleRequest) SettleView {
	return SettleView{
		Id: s.ID, Amount: s.Amount, Fee: s.Fee, Account: s.Account,
		PayType: s.PayType, Status: s.Status, Remark: s.Remark,
		CreatedAt: s.CreatedAt, HandledAt: s.HandledAt,
	}
}

func channelLabel(c string) string { return domain.ChannelName(c) }
