-- FunPay 易支付数据库结构（SQLite）
-- 金额一律以“分”为单位的 INTEGER 存储；时间一律 Unix 秒时间戳

CREATE TABLE IF NOT EXISTS users (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  username      TEXT    NOT NULL UNIQUE,
  password_hash TEXT    NOT NULL,
  email         TEXT    NOT NULL DEFAULT '',
  role          INTEGER NOT NULL DEFAULT 0,  -- 0 商户 1 管理员
  status        INTEGER NOT NULL DEFAULT 1,  -- 1 正常 0 封禁
  balance       INTEGER NOT NULL DEFAULT 0,  -- 余额（分）
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS apps (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL,
  name       TEXT    NOT NULL,
  app_id     INTEGER NOT NULL UNIQUE,        -- 商户 PID
  app_key    TEXT    NOT NULL,               -- 商户密钥
  rate       INTEGER NOT NULL DEFAULT 20,    -- 费率（千分之，20 = 2%）
  status     INTEGER NOT NULL DEFAULT 1,     -- 1 启用 0 停用
  notify_url TEXT    NOT NULL DEFAULT '',    -- 默认异步通知地址
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_apps_user ON apps(user_id);

CREATE TABLE IF NOT EXISTS orders (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  trade_no       TEXT    NOT NULL UNIQUE,    -- 平台订单号
  out_trade_no   TEXT    NOT NULL,           -- 商户订单号
  app_id         INTEGER NOT NULL,           -- apps.id
  user_id        INTEGER NOT NULL,           -- 冗余商户 id，便于查询
  channel        TEXT    NOT NULL,           -- alipay / wxpay / qqpay
  subject        TEXT    NOT NULL,           -- 商品名称
  money          INTEGER NOT NULL,           -- 订单金额（分）
  fee            INTEGER NOT NULL DEFAULT 0, -- 手续费（分）
  status         INTEGER NOT NULL DEFAULT 0, -- 0 待支付 1 已支付 2 已退款 3 已关闭
  notify_url     TEXT    NOT NULL DEFAULT '',
  return_url     TEXT    NOT NULL DEFAULT '',
  client_ip      TEXT    NOT NULL DEFAULT '',
  notified       INTEGER NOT NULL DEFAULT 0, -- 0 未通知 1 通知成功
  notify_attempts INTEGER NOT NULL DEFAULT 0,
  paid_at        INTEGER NOT NULL DEFAULT 0,
  expired_at     INTEGER NOT NULL DEFAULT 0,
  created_at     INTEGER NOT NULL,
  updated_at     INTEGER NOT NULL,
  upstream_trade_no TEXT NOT NULL DEFAULT '',  -- 上游平台订单号
  goods_key      TEXT    NOT NULL DEFAULT '',  -- 绑定的上游商品 key
  quantity       INTEGER NOT NULL DEFAULT 0,   -- 上游购买数量
  unit_price     INTEGER NOT NULL DEFAULT 0,   -- 上游商品单价（分）
  payurl         TEXT    NOT NULL DEFAULT '',  -- 上游收银台地址
  qrcode         TEXT    NOT NULL DEFAULT '',  -- 上游支付二维码内容（weixin:// 串）
  buyer_contact  TEXT    NOT NULL DEFAULT '',  -- 上游下单用的随机联系邮箱
  query_pwd      TEXT    NOT NULL DEFAULT '',  -- 上游订单查询密码
  cards          TEXT    NOT NULL DEFAULT '',  -- 支付成功后上游交付的卡密（JSON）
  UNIQUE(app_id, out_trade_no)
);
CREATE INDEX IF NOT EXISTS idx_orders_user ON orders(user_id, id);
CREATE INDEX IF NOT EXISTS idx_orders_status_time ON orders(status, created_at);

CREATE TABLE IF NOT EXISTS settle_requests (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL,
  amount     INTEGER NOT NULL,               -- 提现金额（分）
  fee        INTEGER NOT NULL DEFAULT 0,     -- 提现手续费（分）
  account    TEXT    NOT NULL,               -- 收款账号
  pay_type   TEXT    NOT NULL DEFAULT 'alipay', -- alipay / bank
  status     INTEGER NOT NULL DEFAULT 0,     -- 0 待审核 1 已打款 2 已驳回
  remark     TEXT    NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  handled_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_settles_user ON settle_requests(user_id, id);

CREATE TABLE IF NOT EXISTS balance_logs (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id       INTEGER NOT NULL,
  type          INTEGER NOT NULL,            -- 0 订单收入 1 提现扣款 2 驳回返还 3 管理员调整 4 订单退款
  amount        INTEGER NOT NULL,            -- 正负（分）
  balance_after INTEGER NOT NULL,
  ref_id        INTEGER NOT NULL DEFAULT 0,  -- 关联 orders.id / settle_requests.id
  note          TEXT    NOT NULL DEFAULT '',
  created_at    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_balance_logs_user ON balance_logs(user_id, id);

CREATE TABLE IF NOT EXISTS sessions (
  token      TEXT    PRIMARY KEY,
  user_id    INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  rotated_to TEXT    NOT NULL DEFAULT ''  -- 已轮换到的新令牌；宽限期内重复刷新返回同一令牌
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);

CREATE TABLE IF NOT EXISTS settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
