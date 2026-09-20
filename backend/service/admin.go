package service

import (
	"context"
	"time"

	"github.com/cyi-cc/fun"

	"epay/database"
	"epay/db"
	"epay/domain"
)

// AdminSvc 管理员后台服务（AuthGuard + AdminGuard）。
type AdminSvc struct {
	fun.Ctx
	Db     *database.Database
	Notify *Notifier
}

type DashboardView struct {
	TodayCount    int64
	TodayMoney    int64
	TodayFee      int64
	UserCount     int64
	PaidCount     int64
	TotalMoney    int64
	PendingSettle int64
	Recent        []AdminOrderView
}

type ListUsersDto struct {
	Page     int64
	PageSize int64
	Kw       *string
}

type AdminUserPage struct {
	Total int64
	Items []AdminUserView
}

type AdminUserView struct {
	Id        int64
	Username  string
	Email     string
	Role      int64
	Status    int64
	Balance   int64
	CreatedAt int64
}

type SetUserStatusDto struct {
	Id     int64
	Status int64
}

type AdjustBalanceDto struct {
	Id     int64
	Amount int64 // 正数加款，负数扣款（分）
	Note   string
}

type TradeNoAdminDto struct {
	TradeNo string
}

type ListSettlesDto struct {
	Page     int64
	PageSize int64
	Status   *int64
}

type HandleSettleDto struct {
	Id      int64
	Approve bool
	Remark  *string
}

type SaveSettingsDto struct {
	Items []SettingItem
}

// Dashboard 平台运营总览。
func (s *AdminSvc) Dashboard() (DashboardView, error) {
	ctx := context.Background()
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()

	today, err := s.Db.Q.OrderStatsAll(ctx, db.OrderStatsAllParams{TFrom: dayStart, TTo: now.Unix() + 1})
	if err != nil {
		return DashboardView{}, fun.Error(5000, "统计失败")
	}
	totals, err := s.Db.Q.GlobalTotals(ctx)
	if err != nil {
		return DashboardView{}, fun.Error(5000, "统计失败")
	}
	rows, err := s.Db.Q.ListRecentOrdersAll(ctx, 10)
	if err != nil {
		return DashboardView{}, fun.Error(5000, "查询订单失败")
	}
	recent := make([]AdminOrderView, 0, len(rows))
	for _, r := range rows {
		recent = append(recent, AdminOrderView{
			OrderView: OrderView{
				Id: r.ID, TradeNo: r.TradeNo, OutTradeNo: r.OutTradeNo, Channel: r.Channel,
				Subject: r.Subject, Money: r.Money, Fee: r.Fee, Status: r.Status,
				Notified: r.Notified, PaidAt: r.PaidAt, ExpiredAt: r.ExpiredAt,
				CreatedAt: r.CreatedAt, AppName: r.AppName, Pid: r.Pid,
			},
			Merchant: r.Merchant, NotifyUrl: r.NotifyUrl, ReturnUrl: r.ReturnUrl,
		})
	}
	return DashboardView{
		TodayCount: today.Cnt, TodayMoney: domain.AnyI64(today.TotalMoney), TodayFee: domain.AnyI64(today.TotalFee),
		UserCount: totals.UserCnt, PaidCount: totals.PaidCnt,
		TotalMoney: domain.AnyI64(totals.TotalMoney), PendingSettle: totals.PendingSettleCnt,
		Recent: recent,
	}, nil
}

// Users 商户列表。
func (s *AdminSvc) Users(dto ListUsersDto) (AdminUserPage, error) {
	off, lim := clampPage(dto.Page, dto.PageSize)
	ctx := context.Background()
	rows, err := s.Db.Q.ListUsers(ctx, db.ListUsersParams{Kw: nstr(dto.Kw), Off: off, Lim: lim})
	if err != nil {
		return AdminUserPage{}, fun.Error(5000, "查询用户失败")
	}
	total, err := s.Db.Q.CountListUsers(ctx, dto.Kw)
	if err != nil {
		return AdminUserPage{}, fun.Error(5000, "统计用户失败")
	}
	items := make([]AdminUserView, 0, len(rows))
	for _, r := range rows {
		items = append(items, AdminUserView{
			Id: r.ID, Username: r.Username, Email: r.Email, Role: r.Role,
			Status: r.Status, Balance: r.Balance, CreatedAt: r.CreatedAt,
		})
	}
	return AdminUserPage{Total: total, Items: items}, nil
}

// SetUserStatus 封禁/解封商户。
func (s *AdminSvc) SetUserStatus(dto SetUserStatusDto) error {
	if dto.Status != 0 && dto.Status != 1 {
		return errParam("状态取值不合法")
	}
	rows, err := s.Db.Q.UpdateUserStatus(context.Background(), db.UpdateUserStatusParams{
		Status: dto.Status, Now: time.Now().Unix(), ID: dto.Id,
	})
	if err != nil || rows == 0 {
		return errNotFound("用户不存在")
	}
	if dto.Status == 0 {
		_ = s.Db.Q.DeleteUserSessions(context.Background(), dto.Id)
	}
	return nil
}

// AdjustBalance 管理员手动调整余额。
func (s *AdminSvc) AdjustBalance(dto AdjustBalanceDto) error {
	if dto.Amount == 0 {
		return errParam("调整金额不能为 0")
	}
	ctx := context.Background()
	now := time.Now().Unix()
	return s.Db.Tx(ctx, func(q *db.Queries) error {
		bal, err := q.AddUserBalance(ctx, db.AddUserBalanceParams{Delta: dto.Amount, Now: now, ID: dto.Id})
		if err != nil {
			return errNotFound("用户不存在")
		}
		return q.CreateBalanceLog(ctx, db.CreateBalanceLogParams{
			UserID: dto.Id, Type: domain.BalanceLogAdminAdjust, Amount: dto.Amount,
			BalanceAfter: bal, RefID: 0, Note: dto.Note, Now: now,
		})
	})
}

// Orders 平台订单分页查询。
func (s *AdminSvc) Orders(dto ListOrdersDto) (AdminOrderPage, error) {
	off, lim := clampPage(dto.Page, dto.PageSize)
	ctx := context.Background()
	arg := db.ListOrdersAllParams{
		Status: nint64(dto.Status), Channel: nstr(dto.Channel), Kw: nstr(dto.Kw),
		TFrom: nint64(dto.From), TTo: nint64(dto.To), Off: off, Lim: lim,
	}
	rows, err := s.Db.Q.ListOrdersAll(ctx, arg)
	if err != nil {
		return AdminOrderPage{}, fun.Error(5000, "查询订单失败")
	}
	total, err := s.Db.Q.CountOrdersAll(ctx, db.CountOrdersAllParams{
		Status: arg.Status, Channel: arg.Channel, Kw: arg.Kw, TFrom: arg.TFrom, TTo: arg.TTo,
	})
	if err != nil {
		return AdminOrderPage{}, fun.Error(5000, "统计订单失败")
	}
	items := make([]AdminOrderView, 0, len(rows))
	for _, r := range rows {
		items = append(items, AdminOrderView{
			OrderView: OrderView{
				Id: r.ID, TradeNo: r.TradeNo, OutTradeNo: r.OutTradeNo, Channel: r.Channel,
				Subject: r.Subject, Money: r.Money, Fee: r.Fee, Status: r.Status,
				Notified: r.Notified, PaidAt: r.PaidAt, ExpiredAt: r.ExpiredAt,
				CreatedAt: r.CreatedAt, AppName: r.AppName, Pid: r.Pid,
			},
			Merchant: r.Merchant, NotifyUrl: r.NotifyUrl, ReturnUrl: r.ReturnUrl,
		})
	}
	return AdminOrderPage{Total: total, Items: items}, nil
}

// Compense 补单：待支付订单强制完成；已支付订单补发通知。
func (s *AdminSvc) Compense(dto TradeNoAdminDto) error {
	o, err := s.Db.Q.GetOrderByTradeNo(context.Background(), dto.TradeNo)
	if err != nil {
		return errNotFound("订单不存在")
	}
	switch o.Status {
	case domain.OrderStatusPending:
		if err := CompleteOrder(s.Db, o.TradeNo); err != nil {
			return err
		}
		s.Notify.OnPaid(o.TradeNo)
		return nil
	case domain.OrderStatusPaid:
		s.Notify.Resend(o.TradeNo)
		return nil
	default:
		return fun.Error(4009, "订单状态不允许补单")
	}
}

// ResendNotify 对已支付订单补发商户通知。
func (s *AdminSvc) ResendNotify(dto TradeNoAdminDto) error {
	o, err := s.Db.Q.GetOrderByTradeNo(context.Background(), dto.TradeNo)
	if err != nil {
		return errNotFound("订单不存在")
	}
	if o.Status != domain.OrderStatusPaid {
		return fun.Error(4009, "仅已支付订单可补发通知")
	}
	s.Notify.Resend(o.TradeNo)
	return nil
}

// Refund 退款：订单置为已退款并扣回商户入账。
func (s *AdminSvc) Refund(dto TradeNoAdminDto) error {
	ctx := context.Background()
	o, err := s.Db.Q.GetOrderByTradeNo(ctx, dto.TradeNo)
	if err != nil {
		return errNotFound("订单不存在")
	}
	if o.Status != domain.OrderStatusPaid {
		return fun.Error(4009, "仅已支付订单可退款")
	}
	now := time.Now().Unix()
	income := o.Money - o.Fee
	return s.Db.Tx(ctx, func(q *db.Queries) error {
		rows, err := q.MarkOrderRefunded(ctx, db.MarkOrderRefundedParams{Now: now, ID: o.ID})
		if err != nil {
			return fun.Error(5000, "更新订单失败")
		}
		if rows == 0 {
			return fun.Error(4009, "订单已被处理")
		}
		bal, err := q.AddUserBalance(ctx, db.AddUserBalanceParams{Delta: -income, Now: now, ID: o.UserID})
		if err != nil {
			return fun.Error(5000, "扣回余额失败")
		}
		return q.CreateBalanceLog(ctx, db.CreateBalanceLogParams{
			UserID: o.UserID, Type: domain.BalanceLogRefund, Amount: -income,
			BalanceAfter: bal, RefID: o.ID, Note: "订单退款 " + o.TradeNo, Now: now,
		})
	})
}

// Settles 结算单列表。
func (s *AdminSvc) Settles(dto ListSettlesDto) (AdminSettlePage, error) {
	off, lim := clampPage(dto.Page, dto.PageSize)
	ctx := context.Background()
	rows, err := s.Db.Q.ListSettlesAll(ctx, db.ListSettlesAllParams{
		Status: nint64(dto.Status), Off: off, Lim: lim,
	})
	if err != nil {
		return AdminSettlePage{}, fun.Error(5000, "查询结算单失败")
	}
	total, err := s.Db.Q.CountSettlesAll(ctx, dto.Status)
	if err != nil {
		return AdminSettlePage{}, fun.Error(5000, "统计结算单失败")
	}
	items := make([]AdminSettleView, 0, len(rows))
	for _, r := range rows {
		items = append(items, AdminSettleView{
			SettleView: SettleView{
				Id: r.ID, Amount: r.Amount, Fee: r.Fee, Account: r.Account,
				PayType: r.PayType, Status: r.Status, Remark: r.Remark,
				CreatedAt: r.CreatedAt, HandledAt: r.HandledAt,
			},
			Merchant: r.Merchant,
		})
	}
	return AdminSettlePage{Total: total, Items: items}, nil
}

// HandleSettle 审核结算单：通过=线下打款；驳回=返还余额。
func (s *AdminSvc) HandleSettle(dto HandleSettleDto) error {
	ctx := context.Background()
	now := time.Now().Unix()
	remark := ""
	if dto.Remark != nil {
		remark = *dto.Remark
	}
	status := domain.SettleStatusPaid
	if !dto.Approve {
		status = domain.SettleStatusReject
	}
	return s.Db.Tx(ctx, func(q *db.Queries) error {
		rows, err := q.HandleSettle(ctx, db.HandleSettleParams{
			Status: status, Remark: remark, HandledAt: now, ID: dto.Id,
		})
		if err != nil || rows == 0 {
			return errNotFound("结算单不存在或已处理")
		}
		if !dto.Approve {
			st, err := q.GetSettle(ctx, dto.Id)
			if err != nil {
				return fun.Error(5000, "读取结算单失败")
			}
			refund := st.Amount + st.Fee
			bal, err := q.AddUserBalance(ctx, db.AddUserBalanceParams{Delta: refund, Now: now, ID: st.UserID})
			if err != nil {
				return fun.Error(5000, "返还余额失败")
			}
			return q.CreateBalanceLog(ctx, db.CreateBalanceLogParams{
				UserID: st.UserID, Type: domain.BalanceLogWithdrawRet, Amount: refund,
				BalanceAfter: bal, RefID: st.ID, Note: "提现驳回返还", Now: now,
			})
		}
		return nil
	})
}

// GetSettings 读取全部站点设置。
func (s *AdminSvc) GetSettings() ([]SettingItem, error) {
	rows, err := s.Db.Q.ListSettings(context.Background())
	if err != nil {
		return []SettingItem{}, fun.Error(5000, "读取设置失败")
	}
	items := make([]SettingItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, SettingItem{Key: r.Key, Value: r.Value})
	}
	return items, nil
}

// SaveSettings 保存站点设置（仅限白名单键）。
func (s *AdminSvc) SaveSettings(dto SaveSettingsDto) error {
	allowed := map[string]bool{
		"site_name": true, "order_expire_minutes": true, "min_settle_fen": true,
	}
	for _, item := range dto.Items {
		if !allowed[item.Key] {
			return errParam("不支持的设置项：" + item.Key)
		}
		if len(item.Value) > 100 {
			return errParam("设置值过长")
		}
	}
	for _, item := range dto.Items {
		if err := s.Db.SetSetting(item.Key, item.Value); err != nil {
			return fun.Error(5000, "保存设置失败")
		}
	}
	return nil
}
