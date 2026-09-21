// Package route 实现易支付标准协议的 HTTP 端点（BindRoute 自定义路由）：
// submit.php 页面跳转支付、mapi.php API 支付、api.php 商户查单。
package route

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cyi-cc/fun"
	"github.com/valyala/fasthttp"

	"epay/config"
	"epay/database"
	"epay/db"
	"epay/domain"
	"epay/upstream"
)

const maxOrderFen = int64(100_000_000) // 单笔上限 100 万元

// Epay 协议路由依赖（fun.Wired 单例，字段经 fun:"auto" 注入）。
type Epay struct {
	Db  *database.Database `fun:"auto"`
	Cfg *config.Config     `fun:"auto"`
	Up  *upstream.Client   `fun:"auto"`

	orderLocksMu sync.Mutex
	orderLocks   map[string]*orderKeyLock
}

type orderKeyLock struct {
	mu   sync.Mutex
	refs int
}

// Bind 注册全部协议路由，须在 fun.Start 前调用。
func (e *Epay) Bind(f *fun.Fun) error {
	handlers := []struct {
		method, path string
		h            fun.RouteHandler
	}{
		{"GET", "/submit.php", e.submit},
		{"POST", "/submit.php", e.submit},
		{"GET", "/mapi.php", e.mapi},
		{"POST", "/mapi.php", e.mapi},
		{"GET", "/api.php", e.api},
		{"POST", "/api.php", e.api},
		{"GET", "/health", e.health},
	}
	for _, r := range handlers {
		if err := f.BindRoute(r.method, r.path, r.h); err != nil {
			return err
		}
	}
	return nil
}

func (e *Epay) health(c *fun.RouteCtx) error {
	c.RequestCtx.WriteString("ok")
	return nil
}

// submit 页面跳转支付：校验签名创建订单后 302 到前端收银台。
func (e *Epay) submit(c *fun.RouteCtx) error {
	order, err := e.createOrder(c)
	if err != nil {
		return writeFail(c, err)
	}
	c.RequestCtx.Redirect(e.payURL(order), fasthttp.StatusFound)
	return nil
}

// mapi API 支付：返回 JSON（code=1 及收银台链接）。
func (e *Epay) mapi(c *fun.RouteCtx) error {
	order, err := e.createOrder(c)
	if err != nil {
		return writeFail(c, err)
	}
	return writeJSON(c, map[string]any{
		"code":     1,
		"msg":      "succ",
		"trade_no": order.TradeNo,
		"payurl":   e.payURL(order),
		"qrcode":   order.Qrcode,
		"img":      "",
	})
}

// api 商户查单：act=order & pid & key & (out_trade_no|trade_no)。
func (e *Epay) api(c *fun.RouteCtx) error {
	if c.Param("act") != "order" {
		return writeJSON(c, map[string]any{"code": -1, "msg": "不支持的操作"})
	}
	pid, err := strconv.ParseInt(strings.TrimSpace(c.Param("pid")), 10, 64)
	if err != nil {
		return writeJSON(c, map[string]any{"code": -1, "msg": "PID 参数错误"})
	}
	key := strings.TrimSpace(c.Param("key"))
	ctx := context.Background()
	app, err := e.Db.Q.GetAppByAppId(ctx, pid)
	if err != nil || app.AppKey != key {
		return writeJSON(c, map[string]any{"code": -1, "msg": "PID 或 KEY 错误"})
	}
	var order db.Order
	switch {
	case c.Param("out_trade_no") != "":
		order, err = e.Db.Q.GetOrderForOut(ctx, db.GetOrderForOutParams{AppID: app.ID, OutTradeNo: c.Param("out_trade_no")})
	case c.Param("trade_no") != "":
		var o db.Order
		o, err = e.Db.Q.GetOrderByTradeNo(ctx, c.Param("trade_no"))
		if err == nil && o.AppID != app.ID {
			err = sql.ErrNoRows
		}
		order = o
	default:
		return writeJSON(c, map[string]any{"code": -1, "msg": "请传入 out_trade_no 或 trade_no"})
	}
	if err != nil {
		return writeJSON(c, map[string]any{"code": -1, "msg": "订单不存在"})
	}
	status := int64(0)
	if order.Status == domain.OrderStatusPaid {
		status = 1
	}
	return writeJSON(c, map[string]any{
		"code":         1,
		"msg":          "查询订单号成功",
		"trade_no":     order.TradeNo,
		"out_trade_no": order.OutTradeNo,
		"name":         order.Subject,
		"money":        domain.FenToYuan(order.Money),
		"status":       status,
	})
}

// createOrder submit/mapi 共用的下单逻辑：验签 → 幂等复用或新建订单。
func (e *Epay) createOrder(c *fun.RouteCtx) (db.Order, error) {
	ctx := context.Background()
	pidStr := strings.TrimSpace(c.Param("pid"))
	if pidStr == "" {
		return db.Order{}, errors.New("PID 不能为空")
	}
	pid, err := strconv.ParseInt(pidStr, 10, 64)
	if err != nil {
		return db.Order{}, errors.New("PID 参数错误")
	}
	channel := strings.TrimSpace(c.Param("type"))
	if channel != "wxpay" {
		return db.Order{}, errors.New("当前仅支持微信支付（wxpay）")
	}
	outTradeNo := strings.TrimSpace(c.Param("out_trade_no"))
	if outTradeNo == "" || len(outTradeNo) > 64 {
		return db.Order{}, errors.New("商户订单号 out_trade_no 需为 1-64 位")
	}
	subject := strings.TrimSpace(c.Param("name"))
	if subject == "" || len(subject) > 100 {
		return db.Order{}, errors.New("商品名称 name 需为 1-100 个字符")
	}
	money, err := domain.YuanToFen(c.Param("money"))
	if err != nil {
		return db.Order{}, err
	}
	if money <= 0 || money > maxOrderFen {
		return db.Order{}, errors.New("订单金额超出允许范围")
	}
	notifyUrl := strings.TrimSpace(c.Param("notify_url"))
	returnUrl := strings.TrimSpace(c.Param("return_url"))
	for _, u := range []string{notifyUrl, returnUrl} {
		if u != "" && !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			return db.Order{}, errors.New("notify_url / return_url 须为合法 URL")
		}
	}

	app, err := e.Db.Q.GetAppByAppId(ctx, pid)
	if err != nil {
		return db.Order{}, errors.New("商户 PID 不存在")
	}
	if app.Status != 1 {
		return db.Order{}, errors.New("商户应用已停用")
	}
	if !domain.VerifySign(c.Data, app.AppKey, strings.TrimSpace(c.Param("sign"))) {
		return db.Order{}, errors.New("签名验证失败")
	}

	// 同一应用的同一商户单号串行执行，保证并发重试只创建一笔上游订单。
	unlockOrder := e.lockOrder(strconv.FormatInt(app.ID, 10) + ":" + outTradeNo)
	defer unlockOrder()

	// 所有支付必须走自动对接的上游商品。
	goodsKey := e.Db.SettingStr("up_goods_key", "")
	goodsID := e.Db.SettingInt("up_goods_id", 0)
	unitPrice := e.Db.SettingInt("up_unit_price", 0)
	stockAmount := e.Db.SettingInt("up_stock_amount", 100000)
	if goodsKey == "" || goodsID <= 0 || unitPrice <= 0 {
		return db.Order{}, errors.New("上游商品未配置：请先在个人页登录上游账号，系统将自动创建兑换码商品")
	}
	if money%unitPrice != 0 {
		return db.Order{}, errors.New("非法金额：订单金额必须是商品单价 " + domain.FenToYuan(unitPrice) + " 元的整数倍")
	}
	quantity := money / unitPrice
	if quantity <= 0 {
		return db.Order{}, errors.New("非法金额")
	}

	now := time.Now().Unix()
	// 幂等：同应用同商户单号
	if exist, err := e.Db.Q.GetOrderForOut(ctx, db.GetOrderForOutParams{AppID: app.ID, OutTradeNo: outTradeNo}); err == nil {
		switch {
		case exist.Money != money:
			return db.Order{}, errors.New("商户订单号重复且金额不一致")
		case exist.Status == domain.OrderStatusPaid:
			return db.Order{}, errors.New("该订单已支付")
		case exist.Status != domain.OrderStatusPending:
			return db.Order{}, errors.New("订单已关闭，请更换订单号")
		case exist.ExpiredAt > 0 && exist.ExpiredAt < now:
			return db.Order{}, errors.New("订单已过期，请更换订单号")
		default:
			return exist, nil
		}
	}

	expire := now + e.Db.SettingInt("order_expire_minutes", 30)*60
	fee := money * app.Rate / 1000
	for attempt := 0; attempt < 3; attempt++ {
		id, err := e.Db.Q.CreateOrder(ctx, db.CreateOrderParams{
			TradeNo: domain.GenTradeNo(), OutTradeNo: outTradeNo, AppID: app.ID, UserID: app.UserID,
			Channel: channel, Subject: subject, Money: money, Fee: fee,
			NotifyUrl: notifyUrl, ReturnUrl: returnUrl,
			ClientIp: c.RequestCtx.RemoteIP().String(), ExpiredAt: expire, Now: now,
		})
		if err != nil {
			if attempt == 2 {
				return db.Order{}, errors.New("创建订单失败，请重试")
			}
			continue
		}
		order, err := e.Db.Q.GetOrder(ctx, id)
		if err != nil {
			return db.Order{}, errors.New("创建订单失败")
		}
		log.Printf("[epay] 新订单 %s pid=%d 金额=%s", order.TradeNo, app.AppID, domain.FenToYuan(money))
		if quantity > 0 {
			if err := e.placeUpstream(ctx, &order, goodsKey, goodsID, stockAmount/unitPrice, quantity, unitPrice); err != nil {
				// 上游单未落地才关闭本地单；已落地的留待支付，轮询器兜底结算
				if order.UpstreamTradeNo == "" {
					_, _ = e.Db.Q.SetOrderStatus(ctx, db.SetOrderStatusParams{
						Status: domain.OrderStatusClosed, Now: time.Now().Unix(), ID: order.ID,
					})
				}
				return db.Order{}, err
			}
		}
		return order, nil
	}
	return db.Order{}, errors.New("创建订单失败")
}

// placeUpstream 在上游平台按绑定商品数量下单，回写上游订单号与收银台地址。
func (e *Epay) placeUpstream(ctx context.Context, order *db.Order, goodsKey string, goodsID, target, quantity, unitPrice int64) error {
	shop := e.Db.SettingStr("up_shop", "")
	if shop == "" {
		return errors.New("上游店铺未配置：请先在个人页登录上游账号")
	}
	channelID, err := e.Up.WechatChannelID(ctx, shop)
	if err != nil {
		return errors.New("上游微信通道不可用：" + err.Error())
	}
	if addr, perr := e.Up.OrderProxy(ctx); perr == nil && addr != "" {
		ctx = upstream.WithPinnedProxy(ctx, addr)
	}
	contact, queryPwd := upstream.RandContact(), upstream.RandQueryPwd()
	res, err := e.Up.CreateOrder(ctx, goodsKey, quantity, channelID, contact, queryPwd)
	if err != nil && strings.Contains(err.Error(), "库存") {
		if stockErr := e.Up.MaintainStock(ctx, goodsID, max(target, quantity)); stockErr != nil {
			return errors.New("上游库存补充失败：" + stockErr.Error())
		}
		res, err = e.Up.CreateOrder(ctx, goodsKey, quantity, channelID, contact, queryPwd)
	}
	if err != nil {
		return errors.New("上游下单失败：" + err.Error())
	}
	order.Payurl = res.PayURL
	order.UpstreamTradeNo = res.TradeNo
	e.Up.RecordStockUse(goodsID, target, quantity)
	if res.TotalAmount != order.Money {
		log.Printf("[epay] 订单 %s 上游金额 %s 与本地 %s 不一致", order.TradeNo, domain.FenToYuan(res.TotalAmount), domain.FenToYuan(order.Money))
	}
	// 先落库上游订单信息：即使抓二维码失败，订单仍留在待支付由轮询器兜底结算
	if err := e.Db.Q.SetOrderUpstream(ctx, db.SetOrderUpstreamParams{
		UpTradeNo: res.TradeNo, GoodsKey: goodsKey, Quantity: quantity,
		UnitPrice: unitPrice, Payurl: res.PayURL, Qrcode: "", BuyerContact: contact,
		QueryPwd: queryPwd, Now: time.Now().Unix(), ID: order.ID,
	}); err != nil {
		return errors.New("回写上游订单信息失败")
	}
	// 抓上游收银台里的微信支付二维码内容（weixin:// 串）
	qrcode, qrErr := e.Up.FetchQRCode(ctx, res.TradeNo)
	if qrErr != nil {
		return errors.New("上游未返回二维码：" + qrErr.Error())
	}
	if qrcode == "" {
		return errors.New("上游未返回二维码")
	}
	if err := e.Db.Q.SetOrderQrcode(ctx, db.SetOrderQrcodeParams{
		Qrcode: qrcode, Now: time.Now().Unix(), ID: order.ID,
	}); err != nil {
		return errors.New("回写二维码失败")
	}
	order.Qrcode = qrcode
	return nil
}

// payURL 订单收银台地址：已对接上游时直接给上游收银台（含支付二维码），否则本地收银台。
func (e *Epay) payURL(o db.Order) string {
	if o.Payurl != "" {
		return o.Payurl
	}
	return e.cashierURL(o.TradeNo)
}

func (e *Epay) cashierURL(tradeNo string) string {
	return strings.TrimRight(e.Cfg.FrontendURL, "/") + "/cashier/" + tradeNo
}

func (e *Epay) lockOrder(key string) func() {
	e.orderLocksMu.Lock()
	if e.orderLocks == nil {
		e.orderLocks = make(map[string]*orderKeyLock)
	}
	l := e.orderLocks[key]
	if l == nil {
		l = &orderKeyLock{}
		e.orderLocks[key] = l
	}
	l.refs++
	e.orderLocksMu.Unlock()

	l.mu.Lock()
	return func() {
		l.mu.Unlock()
		e.orderLocksMu.Lock()
		l.refs--
		if l.refs == 0 {
			delete(e.orderLocks, key)
		}
		e.orderLocksMu.Unlock()
	}
}

func writeJSON(c *fun.RouteCtx, obj map[string]any) error {
	raw, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	c.RequestCtx.Response.Header.SetContentType("application/json; charset=utf-8")
	c.RequestCtx.Write(raw)
	return nil
}

func writeFail(c *fun.RouteCtx, err error) error {
	return writeJSON(c, map[string]any{"code": -1, "msg": err.Error()})
}
