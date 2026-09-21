# Fast Pay

**一个开箱即用的个人收款网关**：兼容易支付（Epay）协议，接入上游卡密平台（链动小铺）即可获得真实可用的微信原生扫码收款能力——无需营业执照、无需微信商户号。

## 特点

- **真实可收款** —— 对接上游卡密商品，买家微信扫码即付；订单支付状态自动轮询、自动结算入账
- **易支付协议兼容** —— `submit.php` / `mapi.php` / `api.php` 全套端点，现有对接易支付的站点可直接接入
- **全自动上游对接** —— 登录上游账号后自动建分类、建商品、填库存；卡密按目标金额自动批量补充（每批 1 万张）
- **代理出口** —— 商户操作固定 IP、下单自动轮换新 IP；兼容 JSON / 纯文本 / SOCKS5 取号接口，失效自动切换
- **异步通知** —— POST 优先 GET 兜底，退避重试（0/15s/1m/5m/15m），重启自动补发
- **轻量零依赖** —— SQLite 单文件数据库、单容器部署，一条 `docker compose up` 即可上线
- **管理控制台** —— 订单管理、提现结算、应用密钥、费率配置、管理员后台，开箱即用

## 快速部署

### Docker（推荐）

```bash
docker compose up -d
# 前端控制台 → http://localhost:4000
```

默认管理员 `admin` / `admin123`，**登录后请立即修改密码**。

### 本地开发

```bash
# 后端（首次启动自动生成 config.json 与 funpay.db）
cd backend && go run .        # :9200

# 前端
cd frontend && npm install && npm run dev   # :5173，/api 已代理到后端
```

## 使用流程

1. 登录控制台 →「系统配置」→ 填入**上游商户账号**（链动小铺），系统自动创建 `0.01元兑换码` 商品并填充库存
2. 设置**库存目标金额**（如 ¥300 = 3 万张兑换码），消耗后自动补齐
3. 上游有 IP 风控时，在「代理配置」填入代理取号 API（商户操作固定 IP，下单自动换 IP）
4. 「商户接入凭证」拿到 PID + 密钥，你的站点按易支付协议对接即可
5. 「测试支付」发起真实订单，扫码体验完整链路

## 协议端点

| 端点 | 说明 |
|---|---|
| `GET/POST /submit.php` | 页面跳转支付：验签下单后 302 到收银台 |
| `GET/POST /mapi.php` | API 支付：返回 `{"code":1,"trade_no","payurl","qrcode"}` |
| `GET/POST /api.php?act=order` | 商户查单 |
| `GET /health` | 健康检查 |

**请求参数**：`pid` `type`(wxpay) `out_trade_no` `name` `money`(元,两位小数) `notify_url` `return_url` `sign` `sign_type=MD5`

**签名**：参数按 key ASCII 升序拼 `k=v&…`（剔除 `sign`/`sign_type`/空值），末尾拼接商户密钥取 MD5。

**异步通知**：支付成功后 POST（GET 兜底）`notify_url`，含 `trade_status=TRADE_SUCCESS` + 签名；商户应答纯文本 `success`，失败退避重试。

## 技术栈

| 层 | 选型 |
|---|---|
| 后端 | Go + [fun](https://github.com/cyi-cc/fun)（单端点 RPC / DI / Guard） |
| 数据 | SQLite（WAL）+ sqlc 生成类型安全查询 |
| 前端 | Vue 3 + Vite + Naive UI + Pinia |
| 部署 | Docker Compose + nginx（前端静态 + API 反代） |

## 目录结构

```
├── backend/            # Go 后端
│   ├── service/        # RPC 服务 + 通知器 + 上游对接服务
│   ├── route/          # 易支付协议路由
│   ├── upstream/       # 上游平台客户端（会话/代理/下单/取码/库存）
│   ├── store/          # sqlc 查询定义
│   └── db/             # sqlc 生成代码（勿手改）
├── frontend/           # Vue 3 控制台 + 收银台
└── deploy/nginx/       # nginx 配置（容器版/本机版）
```

## License

MIT
