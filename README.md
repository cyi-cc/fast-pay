# Fast Pay

基于 [fun 框架](https://fun.cyi.cc/) 的易支付（Epay 协议）项目。

## 技术栈

| 层 | 选型 |
|---|---|
| 后端框架 | [fun](https://github.com/cyi-cc/fun) v1.4.2 —— 单端点 RPC（`POST /cell`），依赖注入 / Guard 鉴权 / 自定义路由 |
| 数据访问 | [sqlc](https://sqlc.dev/) v1.31 —— SQL 优先，生成类型安全的 Go 查询代码 |
| 数据库 | SQLite（modernc.org/sqlite，纯 Go 无 CGO，WAL 模式） |
| 前端 | Vue 3 + Vite + Naive UI + Pinia + Vue Router |
| 前端 API 层 | `fun gen ts` 生成的类型完备 TS 客户端（BigInt 安全、拦截器、`result<T>` 归一化） |

## 目录结构

```
epay-fun/
├── backend/                     # Go 后端
│   ├── main.go                  # 组装：配置→数据库→服务→守卫→协议路由→启动
│   ├── config/                  # config.json 配置单例（首次启动自动生成）
│   ├── database/                # SQLite 连接 + schema.sql（迁移+播种）+ 事务辅助
│   ├── store/query.sql          # sqlc 查询（注意：注释仅 ASCII，中文注释会触发 sqlc bug）
│   ├── db/                      # sqlc 生成代码（勿手改）
│   ├── domain/                  # epay MD5 签名、金额分/元换算、bcrypt、常量
│   ├── guard/                   # AuthGuard（端点策略表：缺省拒绝）+ AdminGuard
│   ├── service/                 # RPC 服务：Auth/User/App/Order/Settle/Cashier/Admin + Notifier
│   └── route/                   # epay 协议路由：submit.php / mapi.php / api.php / health
└── frontend/                    # Vue 3 前端
    └── src/
        ├── api/                 # fun gen 生成的 TS 客户端 + 手写封装（token 注入/401 跳转）
        ├── layouts/             # 侧边栏布局
        └── views/               # 登录 / 订单历史 / 个人信息
```

## 快速启动

```bash
# 1. 后端（首次启动自动生成 config.json 与 funpay.db，并播种管理员 admin/admin123）
cd backend
go run .                  # 监听 :9200

# 2. 前端
cd frontend
npm install
npm run dev               # http://localhost:5173，/api 代理到后端 :9200
```

默认管理员：`admin` / `admin123`（登录后请在「个人信息」修改密码）。

## RPC 调用约定（fun 框架）

- 所有业务走 `POST /cell`，请求体 `{"serviceName","methodName","data","state"}`，响应 `{"status","code?","msg?","data?"}`。
- `status`：`0` 成功；`2` 业务失败（`code/msg` 透传，如 `4010` 未登录）。
- 登录态：会话令牌放请求 `state.token`（前端由请求拦截器自动注入）。
- 服务清单：`AuthSvc`（注册/登录/登出/Me）、`UserSvc`、`AppSvc`、`OrderSvc`、`SettleSvc`、`CashierSvc`、`AdminSvc`。

### 重新生成前端客户端

服务/DTO 变更后（在 backend 目录执行）：

```bash
fun gen ts -o ../frontend/src/api     # 产物在 frontend/src/api/ts/
mv ../frontend/src/api/ts/* ../frontend/src/api && rmdir ../frontend/src/api/ts
```

> fun-cli（github.com/cyi-cc/fun-cli）v1.4.1 及之前在 Windows 无法编译，
> 已提交 GOOS 拆分补丁（`7d5c6b0..e957ce4`），合并后 `go install github.com/cyi-cc/fun-cli/cmd/fun@latest` 即可在 Windows 使用。

## 易支付协议端点（BindRoute 自定义路由）

| 端点 | 说明 |
|---|---|
| `GET/POST /submit.php` | 页面跳转支付：验签下单后 302 到收银台 |
| `GET/POST /mapi.php` | API 支付：返回 `{"code":1,"trade_no","payurl"}` |
| `GET/POST /api.php?act=order` | 商户查单（pid + key + out_trade_no/trade_no） |
| `GET /health` | 健康检查 |

**请求参数**（submit/mapi）：`pid` `type`(alipay/wxpay/qqpay) `out_trade_no` `name` `money`(元,两位小数) `notify_url` `return_url` `sign` `sign_type=MD5`

**签名算法**：参数按 key ASCII 升序拼 `k=v&…`（剔除 `sign`/`sign_type`/空值），末尾直接拼接商户密钥后取 MD5。

**异步通知**：支付成功后 GET 商户 `notify_url`，参数含 `trade_status=TRADE_SUCCESS` + 签名，商户应答纯文本 `success`；失败按 0/15s/1m/5m/15m 退避重试。

## 金额与时间约定

- 金额一律 **分**（INTEGER）存储/传输，展示层转换元；协议层用元字符串（两位小数）。
- 时间一律 Unix 秒时间戳。

## 已实现 / 可扩展

- ✅ 商户注册登录（会话 + bcrypt）、订单历史、个人信息、余额与资金流水
- ✅ 应用（PID/密钥）管理、费率、提现结算、管理员后台（补单/退款/调额/封禁/站点设置）
- ✅ 演示支付通道：`CashierSvc.Confirm` 模拟渠道回调（入账 + 异步通知商户）
- 🔌 接真实支付渠道：在 `service/cashier.go` 的 Confirm 前接入渠道驱动，或将渠道回调转发到订单完成逻辑（`service.CompleteOrder` 为幂等入口）

> 前端当前按需求仅包含登录 / 订单历史 / 个人信息三页；后端全部服务与协议端点均已实现，可直接对接。

## 开发备注

- sqlc 的 SQLite 引擎在**查询文件注释包含中文时会解析失败**（schema 文件不受影响），`store/query.sql` 注释请保持 ASCII。
- SQLite 单连接（`SetMaxOpenConns(1)`）规避 BUSY，关键写操作走 `Database.Tx` 事务。
- 可选过滤参数在 sqlc 中生成为 `interface{}`（`@x IS NULL OR …` 模式），传 `nil` 即忽略。
