package service

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/cyi-cc/fun"

	"epay/config"
	"epay/database"
	"epay/db"
	"epay/domain"
	"epay/guard"
	"epay/upstream"
)

// 上游对接设置键（site 级，仅管理员可配）
const (
	keyUpUsername    = "up_username"     // 上游商户账号
	keyUpPassword    = "up_password_enc" // AES-GCM 加密的上游商户密码
	keyUpShop        = "up_shop"         // 店铺公开 token（link_website）
	keyUpNickname    = "up_nickname"
	keyUpGoodsKey    = "up_goods_key"
	keyUpGoodsID     = "up_goods_id"
	keyUpGoodsName   = "up_goods_name"
	keyUpUnitPrice   = "up_unit_price"   // 分
	keyUpStockAmount = "up_stock_amount" // 库存目标（分）
	keyUpProxyAPI    = "up_proxy_api"    // 代理取号 API；空 = 直连
	keyEncKey        = "enc_key"         // 本地加密密钥材料
)

// 自动建品：固定商品「0.01元兑换码」，分类「兑换码」
const (
	redeemGoodsName = "0.01元兑换码"
	redeemCategory  = "兑换码"
	redeemPriceFen  = int64(1)
)

// ---------- 视图 / DTO ----------

type UpstreamStatusView struct {
	Configured  int64 // 已保存上游账号
	Username    string
	Nickname    string
	Shop        string
	TokenAge    int64 // 商户令牌已用秒数，-1 未登录
	GoodsKey    string
	GoodsID     int64
	GoodsName   string
	UnitPrice   int64  // 分
	StockAmount int64  // 库存目标（分），默认 ¥1000
	UpAvailable int64  // 上游可提现余额（分）
	UpFrozen    int64  // 上游冻结金额（分）
	WalletReady int64  // 钱包数据是否读取成功
	ProxyAPI    string // 代理取号 API（掩码回显）
}

type UpstreamAccountDto struct {
	Username string
	Password string
	ProxyAPI string // 代理取号 API；掩码值 = 保持不变，空 = 清除
}

type UpstreamGoodsQueryDto struct {
	Keywords string
	Current  int64
}

type UpstreamGoodsItem struct {
	ID       int64
	GoodsKey string
	Name     string
	Price    int64 // 分
	Stock    int64
	Status   int64
}

type UpstreamGoodsPage struct {
	Total int64
	Items []UpstreamGoodsItem
}

type BindGoodsDto struct {
	GoodsKey string
	GoodsID  int64
	Name     string
}

type UpstreamCardQueryDto struct {
	Current  int64
	Keywords string
}

type UpstreamCardItem struct {
	ID         int64
	Secret     string
	Status     int64
	CreateTime int64
}

type UpstreamCardPage struct {
	Total int64
	Items []UpstreamCardItem
}

type UpstreamCardAddDto struct {
	Content string
}

// ---------- UpstreamSvc（管理员） ----------

type UpstreamSvc struct {
	fun.Ctx
	Db  *database.Database `fun:"auto"`
	Up  *upstream.Client   `fun:"auto"`
	Cfg *config.Config     `fun:"auto"`

	ensuring atomic.Bool // 自动建品去重
}

func (s *UpstreamSvc) Status() (UpstreamStatusView, error) {
	v := UpstreamStatusView{
		Username:    s.Db.SettingStr(keyUpUsername, ""),
		Nickname:    s.Db.SettingStr(keyUpNickname, ""),
		Shop:        s.Db.SettingStr(keyUpShop, ""),
		TokenAge:    s.Up.TokenAge(),
		GoodsKey:    s.Db.SettingStr(keyUpGoodsKey, ""),
		GoodsID:     s.Db.SettingInt(keyUpGoodsID, 0),
		GoodsName:   s.Db.SettingStr(keyUpGoodsName, ""),
		UnitPrice:   s.Db.SettingInt(keyUpUnitPrice, 0),
		StockAmount: s.Db.SettingInt(keyUpStockAmount, 100000),
		ProxyAPI:    maskProxyAPI(s.Db.SettingStr(keyUpProxyAPI, "")),
	}
	if v.Username != "" {
		v.Configured = 1
	}
	if v.Configured == 1 {
		if avail, frozen, err := s.Up.WalletInfo(context.Background()); err == nil {
			v.UpAvailable, v.UpFrozen, v.WalletReady = avail, frozen, 1
		}
	}
	return v, nil
}

// ensureGoodsAsync 后台确保绑定商品存在（幂等、去重）。
func (s *UpstreamSvc) ensureGoodsAsync() {
	if !s.ensuring.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer s.ensuring.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if err := s.ensureGoods(ctx); err != nil {
			log.Printf("[upstream] 自动建品失败: %v", err)
		}
	}()
}

// ensureGoods 查找商户名下「0.01元兑换码」商品：存在则直接绑定；
// 不存在则先确保分类「兑换码」，再建品 ¥0.01 并填充初始卡密库存。
func (s *UpstreamSvc) ensureGoods(ctx context.Context) error {
	if s.Db.SettingStr(keyUpGoodsKey, "") != "" &&
		s.Db.SettingInt(keyUpGoodsID, 0) > 0 &&
		s.Db.SettingInt(keyUpUnitPrice, 0) == redeemPriceFen {
		return nil
	}
	list, _, err := s.Up.GoodsList(ctx, redeemGoodsName, 1)
	if err != nil {
		return err
	}
	var goodsID int64
	var goodsKey string
	for _, it := range list {
		if it.Name == redeemGoodsName && it.Price == redeemPriceFen && it.Status == 1 {
			goodsID, goodsKey = it.ID, it.GoodsKey
			break
		}
	}
	created := false
	if goodsID == 0 {
		// 分类「兑换码」：没有则创建后重新查询拿 ID
		cats, err := s.Up.CategoryListAll(ctx)
		if err != nil {
			return err
		}
		var catID int64
		for _, cat := range cats {
			if cat.Name == redeemCategory {
				catID = cat.ID
				break
			}
		}
		if catID == 0 {
			if err := s.Up.CategoryAdd(ctx, redeemCategory); err != nil {
				return err
			}
			if cats, err = s.Up.CategoryListAll(ctx); err != nil {
				return err
			}
			for _, cat := range cats {
				if cat.Name == redeemCategory {
					catID = cat.ID
					break
				}
			}
		}
		if catID == 0 {
			return errParam("上游分类创建失败")
		}
		if goodsID, goodsKey, err = s.Up.GoodsAdd(ctx, redeemGoodsName, catID, 0.01); err != nil {
			return err
		}
		if goodsID <= 0 || goodsKey == "" {
			return errParam("上游商品创建未返回有效标识")
		}
		created = true
		log.Printf("[upstream] 已新建商品「%s」id=%d key=%s", redeemGoodsName, goodsID, goodsKey)
	}
	// goods_key 最后写入，作为完整配置的提交标记，避免部分写入后误判已完成。
	for _, item := range []struct{ key, value string }{
		{keyUpGoodsID, int64Str(goodsID)},
		{keyUpGoodsName, redeemGoodsName},
		{keyUpUnitPrice, int64Str(redeemPriceFen)},
		{keyUpGoodsKey, goodsKey},
	} {
		if err := s.Db.SetSetting(item.key, item.value); err != nil {
			return err
		}
	}
	log.Printf("[upstream] 已绑定商品「%s」id=%d 单价 ¥0.01", redeemGoodsName, goodsID)
	if created {
		// 建品后填充初始库存；与下单共用互斥锁，避免初始填充和下单补库重复执行。
		unlock := s.Up.OrderLock()
		defer unlock()
		if target := s.Db.SettingInt(keyUpStockAmount, 100000) / redeemPriceFen; target > 0 {
			if err := s.Up.CardAddN(ctx, goodsID, target); err != nil {
				return err
			}
		}
	}
	return nil
}

// SaveAccount 保存上游商户账号并登录验证；密码留空时复用已存凭据重登刷新令牌。
func (s *UpstreamSvc) SaveAccount(dto UpstreamAccountDto) (UpstreamStatusView, error) {
	dto.Username = strings.TrimSpace(dto.Username)
	if dto.Username == "" {
		return UpstreamStatusView{}, errParam("账号不能为空")
	}
	password := dto.Password
	if password == "" {
		u, pwd, err := decryptPassword(s.Db)
		if err != nil || u != dto.Username || pwd == "" {
			return UpstreamStatusView{}, errParam("请输入密码")
		}
		password = pwd
	}
	// 代理 API：提交了非掩码新值才更新（留空不动现有配置，单独管理见 SetProxy）
	curProxy := s.Db.SettingStr(keyUpProxyAPI, "")
	if v := strings.TrimSpace(dto.ProxyAPI); v != "" && v != maskProxyAPI(curProxy) && v != curProxy {
		if err := s.Db.SetSetting(keyUpProxyAPI, v); err != nil {
			return UpstreamStatusView{}, fun.Error(5000, "保存配置失败")
		}
	}
	s.Up.SetProxyAPI(s.Db.SettingStr(keyUpProxyAPI, ""))
	if err := s.Up.SetCredentials(dto.Username, password); err != nil {
		return UpstreamStatusView{}, err
	}
	nickname, shop, err := s.Up.Userinfo(context.Background())
	if err != nil {
		return UpstreamStatusView{}, err
	}
	enc, err := s.encrypt(password)
	if err != nil {
		return UpstreamStatusView{}, fun.Error(5000, "凭据加密失败")
	}
	oldUsername := s.Db.SettingStr(keyUpUsername, "")
	settings := map[string]string{
		keyUpUsername: dto.Username, keyUpPassword: enc,
		keyUpNickname: nickname, keyUpShop: shop,
	}
	if oldUsername != "" && oldUsername != dto.Username {
		settings[keyUpGoodsKey], settings[keyUpGoodsID] = "", ""
		settings[keyUpGoodsName], settings[keyUpUnitPrice] = "", ""
	}
	for k, v := range settings {
		if err := s.Db.SetSetting(k, v); err != nil {
			return UpstreamStatusView{}, fun.Error(5000, "保存配置失败")
		}
	}
	// 账号与店铺信息持久化成功后仅触发一次自动建品/绑定。
	s.ensureGoodsAsync()
	return s.Status()
}

// GoodsList 上游商品列表（绑定选择器用）。
func (s *UpstreamSvc) GoodsList(dto UpstreamGoodsQueryDto) (UpstreamGoodsPage, error) {
	if dto.Current < 1 {
		dto.Current = 1
	}
	list, total, err := s.Up.GoodsList(context.Background(), dto.Keywords, dto.Current)
	if err != nil {
		return UpstreamGoodsPage{}, err
	}
	items := make([]UpstreamGoodsItem, 0, len(list))
	for _, g := range list {
		items = append(items, UpstreamGoodsItem{
			ID: g.ID, GoodsKey: g.GoodsKey, Name: g.Name,
			Price: g.Price, Stock: g.Stock, Status: g.Status,
		})
	}
	return UpstreamGoodsPage{Total: total, Items: items}, nil
}

// BindGoods 绑定上游商品：以商品详情页的售价（非含通道费的实付价）为单价落库。
func (s *UpstreamSvc) BindGoods(dto BindGoodsDto) (UpstreamStatusView, error) {
	if dto.GoodsKey == "" || dto.GoodsID == 0 {
		return UpstreamStatusView{}, errParam("缺少商品标识")
	}
	price, name, _, err := s.Up.GoodsInfo(context.Background(), dto.GoodsID)
	if err != nil {
		return UpstreamStatusView{}, err
	}
	if price <= 0 {
		return UpstreamStatusView{}, errParam("上游商品单价异常")
	}
	if name != "" {
		dto.Name = name
	}
	for k, v := range map[string]string{
		keyUpGoodsKey: dto.GoodsKey, keyUpGoodsID: int64Str(dto.GoodsID),
		keyUpGoodsName: dto.Name, keyUpUnitPrice: int64Str(price),
	} {
		if err := s.Db.SetSetting(k, v); err != nil {
			return UpstreamStatusView{}, fun.Error(5000, "保存配置失败")
		}
	}
	return s.Status()
}

type UpstreamProxyDto struct {
	ProxyAPI string // 代理取号 API；掩码值 = 保持不变，空 = 清除
}

// SetProxy 单独保存代理取号 API（无需重新登录），立即生效。
func (s *UpstreamSvc) SetProxy(dto UpstreamProxyDto) (UpstreamStatusView, error) {
	cur := s.Db.SettingStr(keyUpProxyAPI, "")
	v := strings.TrimSpace(dto.ProxyAPI)
	if v == maskProxyAPI(cur) {
		v = cur
	}
	if v != cur {
		if err := s.Db.SetSetting(keyUpProxyAPI, v); err != nil {
			return UpstreamStatusView{}, fun.Error(5000, "保存失败")
		}
	}
	s.Up.SetProxyAPI(v)
	return s.Status()
}

type UpstreamStockDto struct {
	StockAmount string // 元
}

// SetStock 设置库存目标金额：下单前库存不足时一次性补到该目标。
func (s *UpstreamSvc) SetStock(dto UpstreamStockDto) (UpstreamStatusView, error) {
	amount, err := domain.YuanToFen(dto.StockAmount)
	if err != nil {
		return UpstreamStatusView{}, errParam(err.Error())
	}
	if amount <= 0 {
		return UpstreamStatusView{}, errParam("库存目标必须大于 0")
	}
	if amount > 1_000_000 {
		return UpstreamStatusView{}, errParam("库存金额不能超过 10000 元")
	}
	if p := s.Db.SettingInt(keyUpUnitPrice, 0); p > 0 && amount%p != 0 {
		return UpstreamStatusView{}, errParam("库存目标必须是商品单价 " + domain.FenToYuan(p) + " 元的整数倍")
	}
	if err := s.Db.SetSetting(keyUpStockAmount, int64Str(amount)); err != nil {
		return UpstreamStatusView{}, fun.Error(5000, "保存失败")
	}
	return s.Status()
}

// CardList 绑定商品的卡密库存。
func (s *UpstreamSvc) CardList(dto UpstreamCardQueryDto) (UpstreamCardPage, error) {
	goodsID := s.Db.SettingInt(keyUpGoodsID, 0)
	if goodsID == 0 {
		return UpstreamCardPage{}, errParam("请先绑定上游商品")
	}
	list, stock, err := s.Up.CardList(context.Background(), goodsID, dto.Keywords, dto.Current)
	if err != nil {
		return UpstreamCardPage{}, err
	}
	items := make([]UpstreamCardItem, 0, len(list))
	for _, c := range list {
		items = append(items, UpstreamCardItem{
			ID: c.ID, Secret: c.Secret, Status: c.Status, CreateTime: c.CreateTime,
		})
	}
	return UpstreamCardPage{Total: stock, Items: items}, nil
}

// CardAdd 向绑定商品导入卡密（一行一张）。
func (s *UpstreamSvc) CardAdd(dto UpstreamCardAddDto) (string, error) {
	goodsID := s.Db.SettingInt(keyUpGoodsID, 0)
	if goodsID == 0 {
		return "", errParam("请先绑定上游商品")
	}
	if dto.Content == "" {
		return "", errParam("卡密内容不能为空")
	}
	return s.Up.CardAdd(context.Background(), goodsID, dto.Content)
}

type TestPayDto struct {
	Money string
}

type TestPayResult struct {
	TradeNo    string
	OutTradeNo string
	Money      int64 // 分
	Quantity   int64
	Qrcode     string
	PayURL     string
}

// TestPay 发起一笔测试支付：以当前商户应用身份自签 mapi.php 请求，走完整下单链路。
func (s *UpstreamSvc) TestPay(dto TestPayDto) (TestPayResult, error) {
	money, err := domain.YuanToFen(dto.Money)
	if err != nil {
		return TestPayResult{}, errParam(err.Error())
	}
	if money <= 0 {
		return TestPayResult{}, errParam("金额必须大于 0")
	}
	ctx := context.Background()
	apps, err := s.Db.Q.ListAppsByUser(ctx, guard.UID(s.Ctx))
	if err != nil {
		return TestPayResult{}, errNotFound("暂无可用应用")
	}
	var app db.App
	for _, candidate := range apps {
		if candidate.Status == 1 {
			app = candidate
			break
		}
	}
	if app.ID == 0 {
		return TestPayResult{}, errNotFound("暂无可用应用")
	}
	outTradeNo := "TEST" + int64Str(time.Now().UnixNano()/1e6)
	params := map[string]string{
		"pid":          int64Str(app.AppID),
		"type":         "wxpay",
		"out_trade_no": outTradeNo,
		"name":         "测试支付",
		"money":        domain.FenToYuan(money),
		"clientip":     "127.0.0.1",
		"sign_type":    "MD5",
	}
	params["sign"] = domain.Sign(params, app.AppKey)

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"http://127.0.0.1:"+int64Str(int64(s.Cfg.Port))+"/mapi.php", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return TestPayResult{}, fun.Error(5000, "调用本地支付接口失败")
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out struct {
		Code    int    `json:"code"`
		Msg     string `json:"msg"`
		TradeNo string `json:"trade_no"`
		PayURL  string `json:"payurl"`
		Qrcode  string `json:"qrcode"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return TestPayResult{}, fun.Error(5000, "支付接口响应解析失败")
	}
	if out.Code != 1 {
		return TestPayResult{}, errParam(out.Msg)
	}
	var quantity int64
	if p := s.Db.SettingInt(keyUpUnitPrice, 0); p > 0 {
		quantity = money / p
	}
	return TestPayResult{
		TradeNo: out.TradeNo, OutTradeNo: outTradeNo, Money: money,
		Quantity: quantity, Qrcode: out.Qrcode, PayURL: out.PayURL,
	}, nil
}

// maskProxyAPI 代理 API 回显掩码：提交回掩码值表示「保持不变」（同 vivid 站点设置惯例）。
func maskProxyAPI(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	return strings.Repeat("•", min(utf8.RuneCountInString(raw), 40))
}

func (s *UpstreamSvc) encrypt(plain string) (string, error) {
	key := s.Db.SettingStr(keyEncKey, "")
	if key == "" {
		key = domain.GenKey() + domain.GenKey()
		if err := s.Db.SetSetting(keyEncKey, key); err != nil {
			return "", err
		}
	}
	return domain.EncryptText(key, plain)
}

// decryptPassword 读取并解密上游商户密码（启动时恢复会话用）。
func decryptPassword(d *database.Database) (string, string, error) {
	user := d.SettingStr(keyUpUsername, "")
	enc := d.SettingStr(keyUpPassword, "")
	key := d.SettingStr(keyEncKey, "")
	if user == "" || enc == "" || key == "" {
		return "", "", nil
	}
	pwd, err := domain.DecryptText(key, enc)
	if err != nil {
		return "", "", err
	}
	return user, pwd, nil
}

// ---------- UpstreamPoller 上游订单支付轮询（fun.Wired 单例） ----------

type UpstreamPoller struct {
	Db     *database.Database `fun:"auto"`
	Up     *upstream.Client   `fun:"auto"`
	Notify *Notifier          `fun:"auto"`
}

func (p *UpstreamPoller) New() error {
	// 商户凭据与令牌由 upstream.Client.New 从设置中恢复，这里只启动轮询
	go p.loop()
	return nil
}

func (p *UpstreamPoller) loop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		p.sweep()
	}
}

func (p *UpstreamPoller) sweep() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	orders, err := p.Db.Q.ListPendingUpstreamOrders(ctx, db.ListPendingUpstreamOrdersParams{
		MinCreatedAt: time.Now().Add(-24 * time.Hour).Unix(), Lim: 20,
	})
	if err != nil {
		return
	}
	for _, o := range orders {
		paid, err := p.Up.OrderPaid(ctx, o.UpstreamTradeNo)
		if err != nil || !paid {
			continue
		}
		// 卡密是为上游发货机制生成的随机占位内容，不下载/落库存储；
		// Pay/query 已确认收款后直接完成本地结算，避免大数量卡密响应过大。
		if err := CompleteUpstreamOrder(p.Db, o.TradeNo); err != nil {
			log.Printf("[upstream] 订单 %s 结算失败: %v", o.TradeNo, err)
			continue
		}
		p.Notify.OnPaid(o.TradeNo)
		log.Printf("[upstream] 订单 %s 上游已支付，完成结算", o.TradeNo)
	}
}
