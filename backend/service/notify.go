package service

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cyi-cc/fun"

	"epay/database"
	"epay/db"
	"epay/domain"
)

// notifyBackoff 通知重试退避间隔：立即 / 15s / 1m / 5m / 15m。
var notifyBackoff = []time.Duration{0, 15 * time.Second, time.Minute, 5 * time.Minute, 15 * time.Minute}

// Notifier 商户异步通知器（fun.Wired 单例）。
type Notifier struct {
	Db *database.Database `fun:"auto"`
}

// NotifyParams 构造通知/跳转回执参数（不含 sign/sign_type）。
func NotifyParams(o db.Order, pid int64) map[string]string {
	return map[string]string{
		"pid":          int64Str(pid),
		"trade_no":     o.TradeNo,
		"out_trade_no": o.OutTradeNo,
		"type":         o.Channel,
		"name":         o.Subject,
		"money":        domain.FenToYuan(o.Money),
		"trade_status": "TRADE_SUCCESS",
	}
}

// BuildSignedURL 将参数签名后拼接到回调地址上。
func BuildSignedURL(base string, params map[string]string, key string) string {
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	q.Set("sign_type", "MD5")
	q.Set("sign", domain.Sign(params, key))
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + q.Encode()
}

// OnPaid 订单支付完成后异步通知商户，成功应答为纯文本 success。
func (n *Notifier) OnPaid(tradeNo string) {
	go n.deliver(tradeNo)
}

// Resend 重置通知状态并重新投递（管理员补发）。
func (n *Notifier) Resend(tradeNo string) {
	o, err := n.Db.Q.GetOrderByTradeNo(context.Background(), tradeNo)
	if err != nil || o.Status != domain.OrderStatusPaid {
		return
	}
	_ = n.Db.Q.ResetOrderNotify(context.Background(), db.ResetOrderNotifyParams{
		Now: time.Now().Unix(), ID: o.ID,
	})
	n.OnPaid(tradeNo)
}

func (n *Notifier) deliver(tradeNo string) {
	for i, delay := range notifyBackoff {
		if delay > 0 {
			time.Sleep(delay)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		o, err := n.Db.Q.GetOrderByTradeNo(ctx, tradeNo)
		cancel()
		if err != nil || o.Status != domain.OrderStatusPaid || o.Notified == 1 {
			return
		}
		if o.NotifyUrl == "" {
			return
		}
		if ok := n.attempt(o); ok {
			return
		}
		ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
		_ = n.Db.Q.IncNotifyAttempts(ctx2, db.IncNotifyAttemptsParams{Now: time.Now().Unix(), ID: o.ID})
		cancel2()
		log.Printf("[notify] %s 第 %d 次通知未确认，%s 后重试", tradeNo, i+1, nextDelayLabel(i))
	}
}

func (n *Notifier) attempt(o db.Order) bool {
	app, err := n.Db.Q.GetApp(context.Background(), o.AppID)
	if err != nil {
		return false
	}
	params := NotifyParams(o, app.AppID)
	params["sign"] = domain.Sign(params, app.AppKey)
	params["sign_type"] = "MD5"
	client := &http.Client{Timeout: 10 * time.Second}
	// 先 POST form 后 GET 兜底：不同商户接收端约定不一，任一返回 success 即确认
	if n.try(client, o.NotifyUrl, params) {
		return n.confirm(o)
	}
	if n.tryGet(client, o.NotifyUrl, params) {
		return n.confirm(o)
	}
	return false
}

// try POST application/x-www-form-urlencoded 通知。
func (n *Notifier) try(client *http.Client, target string, params map[string]string) bool {
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	resp, err := client.Post(target, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return okBody(resp)
}

func (n *Notifier) tryGet(client *http.Client, target string, params map[string]string) bool {
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	sep := "?"
	if strings.Contains(target, "?") {
		sep = "&"
	}
	resp, err := client.Get(target + sep + q.Encode())
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return okBody(resp)
}

func okBody(resp *http.Response) bool {
	if resp.StatusCode != http.StatusOK {
		return false
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
	return strings.TrimSpace(string(body)) == "success"
}

func (n *Notifier) confirm(o db.Order) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = n.Db.Q.MarkOrderNotified(ctx, db.MarkOrderNotifiedParams{
		Attempts: o.NotifyAttempts + 1, Now: time.Now().Unix(), ID: o.ID,
	})
	log.Printf("[notify] %s 商户确认成功", o.TradeNo)
	return true
}

func nextDelayLabel(i int) string {
	if i+1 >= len(notifyBackoff) {
		return "停止"
	}
	return notifyBackoff[i+1].String()
}

func int64Str(v int64) string {
	return strconv.FormatInt(v, 10)
}

// CompleteOrder 幂等完成待支付订单：置为已支付、入账商户、记录流水。
func CompleteOrder(d *database.Database, tradeNo string) error {
	return completeOrder(d, tradeNo, false)
}

// CompleteUpstreamOrder 完成已由上游确认收款的订单；允许处理刚过期关闭的订单，
// 避免本地过期清理与上游最终支付状态之间的竞态造成漏单。
func CompleteUpstreamOrder(d *database.Database, tradeNo string) error {
	return completeOrder(d, tradeNo, true)
}

func completeOrder(d *database.Database, tradeNo string, upstreamConfirmed bool) error {
	ctx := context.Background()
	o, err := d.Q.GetOrderByTradeNo(ctx, tradeNo)
	if err != nil {
		return errNotFound("订单不存在")
	}
	if o.Status == domain.OrderStatusPaid {
		return nil
	}
	if upstreamConfirmed {
		if o.UpstreamTradeNo == "" || (o.Status != domain.OrderStatusPending && o.Status != domain.OrderStatusClosed) {
			return fun.Error(4009, "订单当前状态不允许支付")
		}
	} else if o.Status != domain.OrderStatusPending {
		return fun.Error(4009, "订单当前状态不允许支付")
	}
	now := time.Now().Unix()
	income := o.Money - o.Fee
	return d.Tx(ctx, func(q *db.Queries) error {
		var rows int64
		var err error
		if upstreamConfirmed {
			rows, err = q.MarkUpstreamOrderPaid(ctx, db.MarkUpstreamOrderPaidParams{PaidAt: now, Now: now, ID: o.ID})
		} else {
			rows, err = q.MarkOrderPaid(ctx, db.MarkOrderPaidParams{PaidAt: now, Now: now, ID: o.ID})
		}
		if err != nil {
			return fun.Error(5000, "更新订单失败")
		}
		if rows == 0 {
			return fun.Error(4009, "订单已被处理")
		}
		bal, err := q.AddUserBalance(ctx, db.AddUserBalanceParams{Delta: income, Now: now, ID: o.UserID})
		if err != nil {
			return fun.Error(5000, "余额入账失败")
		}
		return q.CreateBalanceLog(ctx, db.CreateBalanceLogParams{
			UserID: o.UserID, Type: domain.BalanceLogOrderIncome, Amount: income,
			BalanceAfter: bal, RefID: o.ID, Note: "订单收入 " + o.TradeNo, Now: now,
		})
	})
}
