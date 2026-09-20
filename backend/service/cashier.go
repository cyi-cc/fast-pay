package service

import (
	"context"
	"time"

	"github.com/cyi-cc/fun"

	"epay/database"
	"epay/db"
	"epay/domain"
)

// CashierSvc 收银台服务（公开：扫码页、模拟支付确认、状态轮询）。
// 支付通道当前为演示用模拟通道，生产环境可在 Confirm 前接入真实渠道回调。
type CashierSvc struct {
	fun.Ctx
	Db     *database.Database
	Notify *Notifier
}

type TradeNoDto struct {
	TradeNo string
}

type CashierView struct {
	TradeNo     string
	OutTradeNo  string
	Channel     string
	ChannelName string
	Subject     string
	Money       int64
	Status      int64
	AppName     string
	SiteName    string
	CreateTime  int64
	ExpireTime  int64
}

type CashierStatus struct {
	Status    int64
	ReturnUrl string
}

// Get 收银台订单信息；待支付且已过期的订单顺带关闭。
func (s *CashierSvc) Get(dto TradeNoDto) (CashierView, error) {
	o, err := s.loadOrder(dto.TradeNo)
	if err != nil {
		return CashierView{}, err
	}
	app, err := s.Db.Q.GetApp(context.Background(), o.AppID)
	if err != nil {
		return CashierView{}, errNotFound("应用不存在")
	}
	return CashierView{
		TradeNo: o.TradeNo, OutTradeNo: o.OutTradeNo,
		Channel: o.Channel, ChannelName: domain.ChannelName(o.Channel),
		Subject: o.Subject, Money: o.Money, Status: o.Status,
		AppName: app.Name, SiteName: s.Db.SettingStr("site_name", "Fast Pay"),
		CreateTime: o.CreatedAt, ExpireTime: o.ExpiredAt,
	}, nil
}

// Confirm 已停用。真实订单只能由上游支付状态轮询确认，禁止公开接口手动入账。
func (s *CashierSvc) Confirm(dto TradeNoDto) error {
	return fun.Error(4009, "手动确认支付已停用")
}

// Status 订单状态轮询；已支付且配置了 return_url 时返回签名后的回跳地址。
func (s *CashierSvc) Status(dto TradeNoDto) (CashierStatus, error) {
	o, err := s.loadOrder(dto.TradeNo)
	if err != nil {
		return CashierStatus{}, err
	}
	res := CashierStatus{Status: o.Status}
	if o.Status == domain.OrderStatusPaid && o.ReturnUrl != "" {
		app, err := s.Db.Q.GetApp(context.Background(), o.AppID)
		if err == nil {
			res.ReturnUrl = BuildSignedURL(o.ReturnUrl, NotifyParams(o, app.AppID), app.AppKey)
		}
	}
	return res, nil
}

// loadOrder 读取订单，待支付且过期则懒关闭。
func (s *CashierSvc) loadOrder(tradeNo string) (db.Order, error) {
	ctx := context.Background()
	o, err := s.Db.Q.GetOrderByTradeNo(ctx, tradeNo)
	if err != nil {
		return db.Order{}, errNotFound("订单不存在")
	}
	if o.Status == domain.OrderStatusPending && o.ExpiredAt > 0 && o.ExpiredAt < time.Now().Unix() {
		if _, err := s.Db.Q.SetOrderStatus(ctx, db.SetOrderStatusParams{
			Status: domain.OrderStatusClosed, Now: time.Now().Unix(), ID: o.ID,
		}); err == nil {
			o.Status = domain.OrderStatusClosed
		}
	}
	return o, nil
}
