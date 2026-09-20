

-- name: CreateUser :one
INSERT INTO users (username, password_hash, email, role, status, balance, created_at, updated_at)
VALUES (@username, @password_hash, @email, @role, 1, 0,
        @now, @now)
RETURNING id;

-- name: GetUser :one
SELECT * FROM users WHERE id = @id;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = @username;

-- name: CountUsers :one
SELECT COUNT(*) AS cnt FROM users;

-- name: UpdatePassword :exec
UPDATE users SET password_hash = @password_hash, updated_at = @now
WHERE id = @id;

-- name: UpdateUserStatus :execrows
UPDATE users SET status = @status, updated_at = @now WHERE id = @id;

-- name: AddUserBalance :one
UPDATE users SET balance = balance + @delta, updated_at = @now
WHERE id = @id
RETURNING balance;

-- name: ListUsers :many
SELECT id, username, email, role, status, balance, created_at
FROM users
WHERE (@kw IS NULL OR username LIKE '%' || @kw || '%'
       OR email LIKE '%' || @kw || '%')
ORDER BY id DESC
LIMIT @lim OFFSET @off;

-- name: CountListUsers :one
SELECT COUNT(*) AS cnt
FROM users
WHERE (@kw IS NULL OR username LIKE '%' || @kw || '%'
       OR email LIKE '%' || @kw || '%');


-- name: CreateSession :exec
INSERT INTO sessions (token, user_id, expires_at, created_at)
VALUES (@token, @user_id, @expires_at, @now);

-- name: GetSessionUser :one
SELECT u.id, u.username, u.password_hash, u.email, u.role, u.status, u.balance, u.created_at, u.updated_at
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token = @token AND s.expires_at > @now AND u.status = 1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token = @token;

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = @user_id;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < @now;


-- name: CreateApp :one
INSERT INTO apps (user_id, name, app_id, app_key, rate, status, notify_url, created_at, updated_at)
VALUES (@user_id, @name, @app_id, @app_key, @rate, 1,
        @notify_url, @now, @now)
RETURNING id;

-- name: MaxAppId :one
SELECT COALESCE(MAX(app_id), 1000) AS max_id FROM apps;

-- name: GetApp :one
SELECT * FROM apps WHERE id = @id;

-- name: GetAppByAppId :one
SELECT * FROM apps WHERE app_id = @app_id;

-- name: ListAppsByUser :many
SELECT * FROM apps WHERE user_id = @user_id ORDER BY id DESC;

-- name: UpdateApp :execrows
UPDATE apps
SET name = @name, notify_url = @notify_url, rate = @rate, status = @status, updated_at = @now
WHERE id = @id AND user_id = @user_id;

-- name: ResetAppKey :exec
UPDATE apps SET app_key = @app_key, updated_at = @now WHERE id = @id;

-- name: DeleteApp :execrows
DELETE FROM apps WHERE id = @id AND user_id = @user_id;


-- name: CreateOrder :one
INSERT INTO orders (trade_no, out_trade_no, app_id, user_id, channel, subject, money, fee,
                    status, notify_url, return_url, client_ip, expired_at, created_at, updated_at)
VALUES (@trade_no, @out_trade_no, @app_id, @user_id,
        @channel, @subject, @money, @fee, 0,
        @notify_url, @return_url, @client_ip, @expired_at,
        @now, @now)
RETURNING id;

-- name: GetOrder :one
SELECT * FROM orders WHERE id = @id;

-- name: GetOrderByTradeNo :one
SELECT * FROM orders WHERE trade_no = @trade_no;

-- name: GetOrderForOut :one
SELECT * FROM orders WHERE app_id = @app_id AND out_trade_no = @out_trade_no;

-- name: MarkOrderPaid :execrows
UPDATE orders SET status = 1, paid_at = @paid_at, updated_at = @now
WHERE id = @id AND status = 0;

-- name: MarkUpstreamOrderPaid :execrows
UPDATE orders SET status = 1, paid_at = @paid_at, updated_at = @now
WHERE id = @id AND status IN (0, 3) AND upstream_trade_no <> '';

-- name: MarkOrderRefunded :execrows
UPDATE orders SET status = 2, updated_at = @now
WHERE id = @id AND status = 1;

-- name: SetOrderStatus :execrows
UPDATE orders SET status = @status, updated_at = @now
WHERE id = @id;

-- name: MarkOrderNotified :exec
UPDATE orders SET notified = 1, notify_attempts = @attempts, updated_at = @now
WHERE id = @id;

-- name: IncNotifyAttempts :exec
UPDATE orders SET notify_attempts = notify_attempts + 1, updated_at = @now
WHERE id = @id;

-- name: ResetOrderNotify :exec
UPDATE orders SET notified = 0, notify_attempts = 0, updated_at = @now
WHERE id = @id;

-- name: CloseExpiredOrders :execrows
UPDATE orders SET status = 3, updated_at = @now
WHERE status = 0 AND expired_at > 0 AND expired_at < @now;

-- name: SetOrderUpstream :exec
UPDATE orders SET upstream_trade_no = @up_trade_no, goods_key = @goods_key, quantity = @quantity,
       unit_price = @unit_price, payurl = @payurl, qrcode = @qrcode, buyer_contact = @buyer_contact,
       query_pwd = @query_pwd, updated_at = @now
WHERE id = @id;

-- name: SetOrderQrcode :exec
UPDATE orders SET qrcode = @qrcode, updated_at = @now
WHERE id = @id;

-- name: ListPendingUpstreamOrders :many
SELECT * FROM orders
WHERE status IN (0, 3) AND upstream_trade_no <> '' AND created_at >= @min_created_at
ORDER BY id ASC
LIMIT @lim;

-- name: SetOrderCards :exec
UPDATE orders SET cards = @cards, updated_at = @now WHERE id = @id;

-- name: ListOrdersByUser :many
SELECT o.id, o.trade_no, o.out_trade_no, o.app_id, o.user_id, o.channel, o.subject, o.money,
       o.fee, o.status, o.notify_url, o.return_url, o.notified, o.notify_attempts, o.paid_at, o.expired_at,
       o.created_at, a.name AS app_name, a.app_id AS pid
FROM orders o JOIN apps a ON a.id = o.app_id
WHERE o.user_id = @user_id
  AND (@status IS NULL OR o.status = @status)
  AND (@channel IS NULL OR o.channel = @channel)
  AND (@kw IS NULL OR o.trade_no LIKE '%' || @kw || '%'
       OR o.out_trade_no LIKE '%' || @kw || '%')
  AND (@t_from IS NULL OR o.created_at >= @t_from)
  AND (@t_to IS NULL OR o.created_at < @t_to)
ORDER BY o.id DESC
LIMIT @lim OFFSET @off;

-- name: CountOrdersByUser :one
SELECT COUNT(*) AS cnt
FROM orders
WHERE user_id = @user_id
  AND (@status IS NULL OR status = @status)
  AND (@channel IS NULL OR channel = @channel)
  AND (@kw IS NULL OR trade_no LIKE '%' || @kw || '%'
       OR out_trade_no LIKE '%' || @kw || '%')
  AND (@t_from IS NULL OR created_at >= @t_from)
  AND (@t_to IS NULL OR created_at < @t_to);

-- name: ListOrdersAll :many
SELECT o.id, o.trade_no, o.out_trade_no, o.app_id, o.user_id, o.channel, o.subject, o.money,
       o.fee, o.status, o.notify_url, o.return_url, o.notified, o.paid_at, o.expired_at,
       o.created_at, a.name AS app_name, a.app_id AS pid, u.username AS merchant
FROM orders o
JOIN apps a ON a.id = o.app_id
JOIN users u ON u.id = o.user_id
WHERE (@status IS NULL OR o.status = @status)
  AND (@channel IS NULL OR o.channel = @channel)
  AND (@kw IS NULL OR o.trade_no LIKE '%' || @kw || '%'
       OR o.out_trade_no LIKE '%' || @kw || '%' OR u.username LIKE '%' || @kw || '%')
  AND (@t_from IS NULL OR o.created_at >= @t_from)
  AND (@t_to IS NULL OR o.created_at < @t_to)
ORDER BY o.id DESC
LIMIT @lim OFFSET @off;

-- name: CountOrdersAll :one
SELECT COUNT(*) AS cnt
FROM orders o JOIN users u ON u.id = o.user_id
WHERE (@status IS NULL OR o.status = @status)
  AND (@channel IS NULL OR o.channel = @channel)
  AND (@kw IS NULL OR o.trade_no LIKE '%' || @kw || '%'
       OR o.out_trade_no LIKE '%' || @kw || '%' OR u.username LIKE '%' || @kw || '%')
  AND (@t_from IS NULL OR o.created_at >= @t_from)
  AND (@t_to IS NULL OR o.created_at < @t_to);

-- name: ListRecentOrdersByUser :many
SELECT o.id, o.trade_no, o.out_trade_no, o.app_id, o.user_id, o.channel, o.subject, o.money,
       o.fee, o.status, o.notify_url, o.return_url, o.notified, o.paid_at, o.expired_at,
       o.created_at, a.name AS app_name, a.app_id AS pid
FROM orders o JOIN apps a ON a.id = o.app_id
WHERE o.user_id = @user_id
ORDER BY o.id DESC
LIMIT @lim;

-- name: ListRecentOrdersAll :many
SELECT o.id, o.trade_no, o.out_trade_no, o.app_id, o.user_id, o.channel, o.subject, o.money,
       o.fee, o.status, o.notify_url, o.return_url, o.notified, o.paid_at, o.expired_at,
       o.created_at, a.name AS app_name, a.app_id AS pid, u.username AS merchant
FROM orders o
JOIN apps a ON a.id = o.app_id
JOIN users u ON u.id = o.user_id
ORDER BY o.id DESC
LIMIT @lim;

-- name: OrderStatsByUser :one
SELECT COUNT(*) AS cnt, COALESCE(SUM(money), 0) AS total_money, COALESCE(SUM(fee), 0) AS total_fee
FROM orders
WHERE user_id = @user_id AND status = 1
  AND created_at >= @t_from AND created_at < @t_to;

-- name: OrderStatsAll :one
SELECT COUNT(*) AS cnt, COALESCE(SUM(money), 0) AS total_money, COALESCE(SUM(fee), 0) AS total_fee
FROM orders
WHERE status = 1 AND created_at >= @t_from AND created_at < @t_to;

-- name: GlobalTotals :one
SELECT (SELECT COUNT(*) FROM users) AS user_cnt,
       (SELECT COUNT(*) FROM orders WHERE status = 1) AS paid_cnt,
       (SELECT COALESCE(SUM(money), 0) FROM orders WHERE status = 1) AS total_money,
       (SELECT COUNT(*) FROM settle_requests WHERE status = 0) AS pending_settle_cnt;


-- name: CreateSettle :one
INSERT INTO settle_requests (user_id, amount, fee, account, pay_type, status, created_at)
VALUES (@user_id, @amount, @fee, @account, @pay_type,
        0, @now)
RETURNING id;

-- name: GetSettle :one
SELECT * FROM settle_requests WHERE id = @id;

-- name: HandleSettle :execrows
UPDATE settle_requests
SET status = @status, remark = @remark, handled_at = @handled_at
WHERE id = @id AND status = 0;

-- name: ListSettlesByUser :many
SELECT * FROM settle_requests WHERE user_id = @user_id
ORDER BY id DESC LIMIT @lim OFFSET @off;

-- name: CountSettlesByUser :one
SELECT COUNT(*) AS cnt FROM settle_requests WHERE user_id = @user_id;

-- name: ListSettlesAll :many
SELECT s.id, s.user_id, s.amount, s.fee, s.account, s.pay_type, s.status, s.remark,
       s.created_at, s.handled_at, u.username AS merchant
FROM settle_requests s JOIN users u ON u.id = s.user_id
WHERE (@status IS NULL OR s.status = @status)
ORDER BY s.id DESC LIMIT @lim OFFSET @off;

-- name: CountSettlesAll :one
SELECT COUNT(*) AS cnt
FROM settle_requests
WHERE (@status IS NULL OR status = @status);


-- name: CreateBalanceLog :exec
INSERT INTO balance_logs (user_id, type, amount, balance_after, ref_id, note, created_at)
VALUES (@user_id, @type, @amount, @balance_after,
        @ref_id, @note, @now);

-- name: ListBalanceLogsByUser :many
SELECT * FROM balance_logs WHERE user_id = @user_id
ORDER BY id DESC LIMIT @lim OFFSET @off;

-- name: CountBalanceLogsByUser :one
SELECT COUNT(*) AS cnt FROM balance_logs WHERE user_id = @user_id;


-- name: GetSetting :one
SELECT value FROM settings WHERE key = @key;

-- name: UpsertSetting :exec
INSERT INTO settings (key, value) VALUES (@key, @value)
ON CONFLICT(key) DO UPDATE SET value = excluded.value;

-- name: ListSettings :many
SELECT key, value FROM settings;
