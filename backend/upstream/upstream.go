// Package upstream 对接上游平台（链动小铺 wzyp.cn）：
// 买家接口（shopApi）无鉴权，商户接口（merchantApi）使用 Merchant-Token。
// 商户令牌为 JWT（约 10 天有效期），平时复用，失效时自动重登并重试一次。
package upstream

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"epay/database"
	"epay/domain"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultBase = "https://wzyp.cn"

// 与 service/upstream.go 共享的设置键
const (
	keyUpUsername  = "up_username"
	keyUpPassword  = "up_password_enc"
	keyEncKey      = "enc_key"
	keyUpToken     = "up_token"
	keyUpTokenAt   = "up_token_at"
	tokenMaxAgeSec = 9 * 24 * 60 * 60 // JWT 约 10 天，9 天为安全上限
)

// ErrAuth 上游会话失效（需要重新登录）。
var ErrAuth = fmt.Errorf("upstream auth expired")

type apiError struct{ msg string }

func (e *apiError) Error() string { return e.msg }

// envelope 上游统一响应壳：code=1 成功。
type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// Client 上游客户端单例（fun.Wired）。
type Client struct {
	Db *database.Database `fun:"auto"`

	mu       sync.Mutex
	loginMu  sync.Mutex
	hc       *http.Client
	base     string
	username string
	password string
	token    string
	loggedAt int64
	nickname string
	shop     string // 店铺公开 token（link_website），买家接口用

	channelID int64 // 微信支付通道缓存（10 分钟）
	channelAt int64

	orderMu sync.Mutex // 串行化「查库存→补库存→下单」，避免并发订单抢同一批卡
}

// OrderLock 取下单互斥锁，返回解锁函数。
func (c *Client) OrderLock() func() {
	c.orderMu.Lock()
	return c.orderMu.Unlock
}

// New 启动时从设置恢复凭据与已签发的商户令牌，避免每次重启都触发上游登录（有频率限制）。
func (c *Client) New() error {
	c.hc = &http.Client{Timeout: 20 * time.Second}
	c.base = defaultBase
	if c.Db == nil {
		return nil
	}
	// 恢复商户凭据（密码加密落库）
	if u := c.Db.SettingStr(keyUpUsername, ""); u != "" {
		if key := c.Db.SettingStr(keyEncKey, ""); key != "" {
			if pwd, err := domain.DecryptText(key, c.Db.SettingStr(keyUpPassword, "")); err == nil {
				c.username, c.password = u, pwd
			}
		}
	}
	// 恢复商户令牌：9 天内视为有效直接复用，失效时首个请求会自动重登
	if tok := c.Db.SettingStr(keyUpToken, ""); tok != "" {
		if at := c.Db.SettingInt(keyUpTokenAt, 0); time.Now().Unix()-at < tokenMaxAgeSec {
			c.token, c.loggedAt = tok, at
		}
	}
	return nil
}

// ---------- 会话 ----------

// SetCredentials 更新商户凭据并立即登录。
func (c *Client) SetCredentials(username, password string) error {
	c.mu.Lock()
	c.username, c.password = strings.TrimSpace(username), password
	c.token, c.loggedAt = "", 0
	c.channelID, c.channelAt = 0, 0
	c.mu.Unlock()
	_, err := c.login()
	return err
}

// Nickname 当前登录商户昵称。
func (c *Client) Nickname() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.nickname
}

// Shop 店铺公开 token。
func (c *Client) Shop() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.shop
}

// TokenAge 当前商户令牌已使用秒数（未登录返回 -1）。
func (c *Client) TokenAge() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == "" {
		return -1
	}
	return time.Now().Unix() - c.loggedAt
}

func (c *Client) login() (string, error) {
	// 上游登录有限频：串行化并在获得锁后复查，避免并发失效请求重复登录。
	c.loginMu.Lock()
	defer c.loginMu.Unlock()
	c.mu.Lock()
	if c.token != "" && time.Now().Unix()-c.loggedAt < tokenMaxAgeSec {
		tok := c.token
		c.mu.Unlock()
		return tok, nil
	}
	u, p := c.username, c.password
	c.mu.Unlock()
	if u == "" || p == "" {
		return "", &apiError{"未配置上游商户账号"}
	}
	var out struct {
		MerchantToken string `json:"merchant_token"`
	}
	if err := c.post(context.Background(), "/merchantApi/user/login",
		map[string]any{"username": u, "password": p}, "", &out); err != nil {
		return "", err
	}
	if out.MerchantToken == "" {
		return "", &apiError{"上游登录未返回令牌"}
	}
	c.mu.Lock()
	c.token, c.loggedAt = out.MerchantToken, time.Now().Unix()
	c.mu.Unlock()
	// 令牌持久化：进程重启后直接复用，不重复登录
	if c.Db != nil {
		_ = c.Db.SetSetting(keyUpToken, out.MerchantToken)
		_ = c.Db.SetSetting(keyUpTokenAt, strconv.FormatInt(c.loggedAt, 10))
	}
	return out.MerchantToken, nil
}

// ensureSession 取有效商户令牌：无令牌时登录；令牌接近 JWT 上限时提前重登。
func (c *Client) ensureSession() (string, error) {
	c.mu.Lock()
	tok, at := c.token, c.loggedAt
	c.mu.Unlock()
	if tok != "" && time.Now().Unix()-at < tokenMaxAgeSec {
		return tok, nil
	}
	return c.login()
}

// EnsureSession 对外暴露的会话保障：有有效令牌则直接返回，否则用已存凭据重登。
func (c *Client) EnsureSession() error {
	_, err := c.ensureSession()
	return err
}

// merchantPost 商户接口调用：携带 Merchant-Token；鉴权失效自动重登并重试一次。
func (c *Client) merchantPost(ctx context.Context, path string, body, out any) error {
	tok, err := c.ensureSession()
	if err != nil {
		return err
	}
	err = c.post(ctx, path, body, tok, out)
	if err != ErrAuth {
		return err
	}
	c.mu.Lock()
	if c.token == tok {
		c.token, c.loggedAt = "", 0
	}
	c.mu.Unlock()
	if _, err := c.login(); err != nil {
		return err
	}
	c.mu.Lock()
	tok = c.token
	c.mu.Unlock()
	return c.post(ctx, path, body, tok, out)
}

// ---------- 底层请求 ----------

func (c *Client) post(ctx context.Context, path string, body any, merchantToken string, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if merchantToken != "" {
		req.Header.Set("Merchant-Token", merchantToken)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("上游请求失败: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ErrAuth
	}
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("上游响应解析失败: %w", err)
	}
	if env.Code != 1 {
		msg := env.Msg
		if strings.Contains(msg, "登录") || strings.Contains(msg, "登陆") || strings.Contains(msg, "授权") {
			return ErrAuth
		}
		if msg == "" {
			msg = fmt.Sprintf("上游返回 code=%d", env.Code)
		}
		return &apiError{msg}
	}
	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("上游数据解析失败: %w", err)
		}
	}
	return nil
}

// ---------- 商户接口 ----------

// Userinfo 拉取商户资料，顺带刷新昵称与店铺公开 token。
func (c *Client) Userinfo(ctx context.Context) (nickname, shop string, err error) {
	var out struct {
		Nickname    string `json:"nickname"`
		LinkWebsite string `json:"link_website"`
	}
	if err := c.merchantPost(ctx, "/merchantApi/user/userinfo", map[string]any{}, &out); err != nil {
		return "", "", err
	}
	c.mu.Lock()
	if c.shop != out.LinkWebsite {
		c.channelID, c.channelAt = 0, 0
	}
	c.nickname, c.shop = out.Nickname, out.LinkWebsite
	c.mu.Unlock()
	return out.Nickname, out.LinkWebsite, nil
}

// MerchantGoods 商户商品列表项。
type MerchantGoods struct {
	ID        int64   `json:"id"`
	GoodsKey  string  `json:"goods_key"`
	Name      string  `json:"name"`
	Price     int64   `json:"-"` // 分
	PriceYuan float64 `json:"price"`
	Status    int64   `json:"status"`
	Stock     int64   `json:"-"`
	Extend    struct {
		StockCount int64 `json:"stock_count"`
	} `json:"extend"`
}

// GoodsList 商户商品列表（卡密类）。
func (c *Client) GoodsList(ctx context.Context, keywords string, current int64) ([]MerchantGoods, int64, error) {
	var out struct {
		Total int64           `json:"total"`
		List  []MerchantGoods `json:"list"`
	}
	body := map[string]any{
		"current": current, "pageSize": 20, "goods_type": "card",
		"status": 999, "name": keywords, "is_proxy": "0",
	}
	if err := c.merchantPost(ctx, "/merchantApi/Goods/list", body, &out); err != nil {
		return nil, 0, err
	}
	for i := range out.List {
		out.List[i].Price = yuanFen(out.List[i].PriceYuan)
		out.List[i].Stock = out.List[i].Extend.StockCount
	}
	return out.List, out.Total, nil
}

// WalletInfo 商户钱包：返回平台可提现余额与冻结金额（分）。
func (c *Client) WalletInfo(ctx context.Context) (availableFen, frozenFen int64, err error) {
	var out struct {
		Platform struct {
			AvailableMoney float64 `json:"available_money"`
			FreezeMoney    float64 `json:"freeze_money"`
		} `json:"platform"`
	}
	if err := c.merchantPost(ctx, "/merchantApi/wallet/info", map[string]any{}, &out); err != nil {
		return 0, 0, err
	}
	return yuanFen(out.Platform.AvailableMoney), yuanFen(out.Platform.FreezeMoney), nil
}

// GoodsInfo 商品详情：返回售价（分）与库存。
func (c *Client) GoodsInfo(ctx context.Context, goodsID int64) (priceFen int64, name string, stock int64, err error) {
	var out struct {
		Name   string  `json:"name"`
		Price  float64 `json:"price"`
		Extend struct {
			StockCount int64 `json:"stock_count"`
		} `json:"extend"`
	}
	if err := c.merchantPost(ctx, "/merchantApi/Goods/info", map[string]any{"id": fmt.Sprint(goodsID)}, &out); err != nil {
		return 0, "", 0, err
	}
	return yuanFen(out.Price), out.Name, out.Extend.StockCount, nil
}

// ---------- 商品分类 / 建品 ----------

// CategoryListAll 全部卡密分类（value=分类 ID）。
func (c *Client) CategoryListAll(ctx context.Context) ([]struct {
	ID   int64
	Name string
}, error) {
	var out []struct {
		Value int64  `json:"value"`
		Label string `json:"label"`
	}
	if err := c.merchantPost(ctx, "/merchantApi/GoodsCategory/listAll", map[string]any{"goods_type": "card"}, &out); err != nil {
		return nil, err
	}
	cats := make([]struct {
		ID   int64
		Name string
	}, 0, len(out))
	for _, v := range out {
		cats = append(cats, struct {
			ID   int64
			Name string
		}{v.Value, v.Label})
	}
	return cats, nil
}

// CategoryAdd 新建卡密分类（响应 data 为 null，需再查 listAll 拿 ID）。
func (c *Client) CategoryAdd(ctx context.Context, name string) error {
	return c.merchantPost(ctx, "/merchantApi/GoodsCategory/update", map[string]any{
		"id": 0, "name": name, "image": "", "sort": 0, "goods_type": "card",
	}, nil)
}

// GoodsAdd 新建卡密商品（payload 对齐上游后台表单），返回商品 ID 与 key。
func (c *Client) GoodsAdd(ctx context.Context, name string, categoryID int64, priceYuan float64) (int64, string, error) {
	var out struct {
		ID       int64  `json:"id"`
		GoodsKey string `json:"goods_key"`
	}
	body := map[string]any{
		"goods_type": "card", "id": 0, "name": name, "image": "", "category_id": categoryID,
		"price": priceYuan, "market_price": 0, "description": "", "sort": 0, "coupon_status": 1,
		"status": 1, "fee_payer": -1, "show": 1, "contact_format": "any", "agent_status": 0,
		"agent_price1": 0, "agent_price2": 0, "agent_price3": 0, "agent_price_limit": 0,
		"description_sync": 0, "name_sync": 0, "parent_id": 0, "cost_price": 0,
		"add_type": 1, "add_rate": 0, "add_price": 0,
		"extend": map[string]any{
			"instructions": "<p><br></p>", "stock_notice": 0, "lock_card": 0,
			"limit_count": 1, "limit_count_max": 0, "show_stock_type": 0,
			"send_order": 0, "query_password_status": 0,
		},
	}
	if err := c.merchantPost(ctx, "/merchantApi/Goods/update", body, &out); err != nil {
		return 0, "", err
	}
	return out.ID, out.GoodsKey, nil
}

// CardItem 卡密库存项。
type CardItem struct {
	ID         int64  `json:"id"`
	Secret     string `json:"secret"`
	Status     int64  `json:"status"`
	CreateTime int64  `json:"create_time"`
}

// CardList 商品卡密库存分页。返回条目与列表总数。
func (c *Client) CardList(ctx context.Context, goodsID int64, keywords string, current int64) ([]CardItem, int64, error) {
	var out struct {
		Total int64      `json:"total"`
		List  []CardItem `json:"list"`
	}
	body := map[string]any{
		"goods_id": fmt.Sprint(goodsID), "current": current, "pageSize": 20,
		"keywords": keywords, "status": "", "first": "",
	}
	if err := c.merchantPost(ctx, "/merchantApi/goodsCardStorage/list", body, &out); err != nil {
		return nil, 0, err
	}
	return out.List, out.Total, nil
}

// CardAdd 导入卡密（一行一张）。返回上游提示语。
func (c *Client) CardAdd(ctx context.Context, goodsID int64, content string) (string, error) {
	err := c.merchantPost(ctx, "/merchantApi/GoodsCardStorage/add", map[string]any{
		"goods_id": goodsID, "content": content, "first": 0, "remove_repeat": 0,
	}, nil)
	if err != nil {
		return "", err
	}
	return "导入成功", nil
}

// cardAddBatch 上游单次导入上限
const cardAddBatch = int64(10000)

// CardAddN 批量导入 n 张随机卡密，按每次 1 万张分批提交。
func (c *Client) CardAddN(ctx context.Context, goodsID, n int64) error {
	if n < 0 || n > 1_000_000 {
		return fmt.Errorf("单次补充卡密数量超出限制")
	}
	for left := n; left > 0; {
		b := left
		if b > cardAddBatch {
			b = cardAddBatch
		}
		if _, err := c.CardAdd(ctx, goodsID, RandCards(b)); err != nil {
			return err
		}
		left -= b
	}
	return nil
}

// ---------- 买家接口（无鉴权） ----------

// Channel 支付通道。
type Channel struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	ShowName     string `json:"show_name"`
	Status       int64  `json:"status"`
	CustomStatus int64  `json:"custom_status"`
}

// WechatChannelID 取店铺可用的微信支付通道 id（优先 Native），缓存 10 分钟。
func (c *Client) WechatChannelID(ctx context.Context, shop string) (int64, error) {
	c.mu.Lock()
	if c.channelID > 0 && time.Now().Unix()-c.channelAt < 600 {
		id := c.channelID
		c.mu.Unlock()
		return id, nil
	}
	c.mu.Unlock()
	var list []Channel
	if err := c.post(ctx, "/shopApi/Shop/getUserChannel", map[string]any{"token": shop}, "", &list); err != nil {
		return 0, err
	}
	var fallback int64
	for _, ch := range list {
		if ch.Status != 1 || ch.CustomStatus != 1 {
			continue
		}
		if fallback == 0 {
			fallback = ch.ID
		}
		if strings.Contains(ch.Name, "微信") || strings.Contains(ch.ShowName, "微信") {
			fallback = ch.ID
			break
		}
	}
	if fallback == 0 {
		return 0, &apiError{"上游没有可用的微信支付通道"}
	}
	c.mu.Lock()
	c.channelID, c.channelAt = fallback, time.Now().Unix()
	c.mu.Unlock()
	return fallback, nil
}

// GoodsPrice 查询商品实付金额（返回分）。
func (c *Client) GoodsPrice(ctx context.Context, goodsKey string, quantity, channelID int64) (int64, error) {
	var out struct {
		TotalAmount float64 `json:"total_amount"`
	}
	err := c.post(ctx, "/shopApi/Shop/getGoodsPrice", map[string]any{
		"goods_key": goodsKey, "quantity": quantity, "coupon_code": "", "channel_id": channelID,
	}, "", &out)
	if err != nil {
		return 0, err
	}
	return yuanFen(out.TotalAmount), nil
}

// PayOrderResult 上游下单结果。
type PayOrderResult struct {
	TradeNo     string `json:"trade_no"`
	TotalAmount int64  `json:"-"`
	PayURL      string `json:"payurl"`

	totalAmountYuan float64
}

// CreateOrder 上游买家下单（微信支付通道），返回平台订单号与收银台地址。
func (c *Client) CreateOrder(ctx context.Context, goodsKey string, quantity, channelID int64, contact, queryPwd string) (PayOrderResult, error) {
	var raw struct {
		TradeNo     string  `json:"trade_no"`
		TotalAmount float64 `json:"total_amount"`
		PayURL      string  `json:"payurl"`
	}
	err := c.post(ctx, "/shopApi/Pay/order", map[string]any{
		"goods_key": goodsKey, "quantity": quantity, "coupon_code": "",
		"channel_id": channelID, "contact": contact, "query_password": queryPwd,
		"select_cards_ids": []any{},
		"extend":           map[string]any{"juuid": randHex(12)},
	}, "", &raw)
	if err != nil {
		return PayOrderResult{}, err
	}
	return PayOrderResult{TradeNo: raw.TradeNo, TotalAmount: yuanFen(raw.TotalAmount), PayURL: raw.PayURL}, nil
}

// FetchQRCode 抓取上游收银台页面里的微信支付二维码内容（weixin:// 串）。
// 收银台 HTML 内嵌 generateQrcode.html?str=<双重编码的码内容>，解码两次还原。
func (c *Client) FetchQRCode(ctx context.Context, tradeNo string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.base+"/payApi/WeixinNative/pay.html?trade_no="+tradeNo, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取收银台页面失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	m := qrStrRe.FindSubmatch(body)
	if len(m) < 2 {
		return "", fmt.Errorf("收银台页面未找到二维码")
	}
	once, err := url.QueryUnescape(string(m[1]))
	if err != nil {
		return "", err
	}
	twice, err := url.QueryUnescape(once)
	if err != nil {
		return "", err
	}
	return twice, nil
}

var qrStrRe = regexp.MustCompile(`generateQrcode\.html\?str=([^"'<>\s&]+)`)

// OrderPaid 查询上游订单是否已支付。
func (c *Client) OrderPaid(ctx context.Context, tradeNo string) (bool, error) {
	// Pay/query：未支付时 code=0 msg="not pay"，已支付 code=1
	raw, err := c.postRaw(ctx, "/shopApi/Pay/query", map[string]any{"trade_no": tradeNo})
	if err != nil {
		return false, err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return false, err
	}
	return env.Code == 1, nil
}

// OrderCards 取已支付订单交付的卡密列表。
func (c *Client) OrderCards(ctx context.Context, tradeNo string) ([]string, error) {
	var out struct {
		Status   int64 `json:"status"`
		Response struct {
			Cards []string `json:"cards"`
		} `json:"response"`
	}
	if err := c.post(ctx, "/shopApi/Order/info", map[string]any{"trade_no": tradeNo, "dump": 1}, "", &out); err != nil {
		return nil, err
	}
	return out.Response.Cards, nil
}

// postRaw 返回原始响应体（用于 code!=1 也是合法应答的接口）。
func (c *Client) postRaw(ctx context.Context, path string, body any) ([]byte, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("上游请求失败: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// ---------- 工具 ----------

func yuanFen(y float64) int64 { return int64(math.Round(y * 100)) }

// RandContact 随机买家联系邮箱（仅用于上游订单查询）。
func RandContact() string { return "p" + randHex(10) + "@outlook.com" }

// RandQueryPwd 随机订单查询密码。
func RandQueryPwd() string { return randHex(8) }

// RandCards 生成 n 张随机卡密（一行一张），下单前导入上游补足库存。
func RandCards(n int64) string {
	var b strings.Builder
	for i := int64(0); i < n; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("FPAY-")
		b.WriteString(randHex(16))
	}
	return b.String()
}

func randHex(n int) string {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		for i := range raw {
			raw[i] = byte(time.Now().UnixNano() >> (i % 8))
		}
	}
	const digits = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = digits[raw[i]%16]
	}
	return string(b)
}
