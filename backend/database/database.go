// Package database 持有 SQLite 连接与 sqlc 查询对象（fun.Wired 单例）。
package database

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"

	"epay/config"
	"epay/db"
	"epay/domain"
)

//go:embed schema.sql
var schemaSQL string

// Database 数据库单例：Q 为 sqlc 生成的类型安全查询集。
// 依赖经 fun:"auto" 标签注入（Config 须先于 Database Wired）。
type Database struct {
	Cfg *config.Config `fun:"auto"`
	DB  *sql.DB
	Q   *db.Queries
}

// New 由 fun.Wired 调用：打开 SQLite、建表、播种初始数据。
// 注意：New 在 Wired 锁内执行，禁止再调用 fun.Wired（不可重入会死锁）。
func (d *Database) New() error {
	cfg := d.Cfg
	dsn := "file:" + cfg.DBPath +
		"?_pragma=busy_timeout(10000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=synchronous(NORMAL)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite 单写者：串行化连接可规避 SQLITE_BUSY，本项目规模下足够
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetConnMaxIdleTime(0)
	if _, err := sqlDB.Exec(schemaSQL); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}
	if err := migrateSessions(sqlDB); err != nil {
		return fmt.Errorf("migrate sessions: %w", err)
	}
	if err := migrateOrders(sqlDB); err != nil {
		return fmt.Errorf("migrate orders: %w", err)
	}
	d.DB = sqlDB
	d.Q = db.New(sqlDB)

	if err := d.seed(cfg); err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	go d.cleanupLoop()
	return nil
}

// Tx 在事务中执行 fn，fn 内的 Queries 绑定到同一事务。
func (d *Database) Tx(ctx context.Context, fn func(q *db.Queries) error) error {
	tx, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(d.Q.WithTx(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// SettingInt 读取整型设置，缺省返回 def。
func (d *Database) SettingInt(key string, def int64) int64 {
	v, err := d.Q.GetSetting(context.Background(), key)
	if err != nil {
		return def
	}
	var n int64
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return def
	}
	return n
}

// SettingStr 读取字符串设置，缺省返回 def。
func (d *Database) SettingStr(key, def string) string {
	v, err := d.Q.GetSetting(context.Background(), key)
	if err != nil || v == "" {
		return def
	}
	return v
}

// SetSetting 写入设置项。
func (d *Database) SetSetting(key, value string) error {
	return d.Q.UpsertSetting(context.Background(), db.UpsertSettingParams{Key: key, Value: value})
}

// ErrSessionInvalid 会话不存在或已过期（轮换入口据此判定需重新登录）。
var ErrSessionInvalid = errors.New("session invalid or expired")

// RefreshSession 显式轮换会话令牌：签发新令牌（有效期 24 小时），
// 旧令牌保留 5 分钟宽限期供在途请求完成；宽限期内重复刷新返回同一枚新令牌（幂等）。
func (d *Database) RefreshSession(oldToken, newToken string, now int64) (string, error) {
	tx, err := d.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var userID, expiresAt int64
	var rotatedTo string
	err = tx.QueryRow(
		`SELECT user_id, expires_at, COALESCE(rotated_to, '') FROM sessions WHERE token = ?`,
		oldToken,
	).Scan(&userID, &expiresAt, &rotatedTo)
	if err == sql.ErrNoRows || (err == nil && expiresAt <= now) {
		return "", ErrSessionInvalid
	}
	if err != nil {
		return "", err
	}
	if rotatedTo != "" {
		// 仅当目标令牌仍存在且有效时返回，避免注销后通过旧令牌拿到死 token。
		var targetExpires int64
		if err := tx.QueryRow(`SELECT expires_at FROM sessions WHERE token = ?`, rotatedTo).Scan(&targetExpires); err != nil || targetExpires <= now {
			return "", ErrSessionInvalid
		}
		if err := tx.Commit(); err != nil {
			return "", err
		}
		return rotatedTo, nil
	}
	if _, err := tx.Exec(
		`INSERT INTO sessions (token, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		newToken, userID, now+24*60*60, now,
	); err != nil {
		return "", err
	}
	if _, err := tx.Exec(
		`UPDATE sessions SET rotated_to = ?, expires_at = MIN(expires_at, ?) WHERE token = ?`,
		newToken, now+5*60, oldToken,
	); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return newToken, nil
}

func (d *Database) seed(cfg *config.Config) error {
	ctx := context.Background()
	now := time.Now().Unix()
	defaults := []db.UpsertSettingParams{
		{Key: "site_name", Value: cfg.SiteName},
		{Key: "order_expire_minutes", Value: "10"},
		{Key: "min_settle_fen", Value: "1000"},    // 最低提现 10 元
		{Key: "up_stock_amount", Value: "100000"}, // 上游库存目标（分），默认 ¥1000
	}
	for _, s := range defaults {
		if _, err := d.Q.GetSetting(ctx, s.Key); err != nil {
			if err := d.Q.UpsertSetting(ctx, s); err != nil {
				return err
			}
		}
	}

	// 首次启动创建管理员
	cnt, err := d.Q.CountUsers(ctx)
	if err != nil {
		return err
	}
	if cnt == 0 {
		hash, err := domain.HashPassword(cfg.AdminPassword)
		if err != nil {
			return err
		}
		if _, err := d.Q.CreateUser(ctx, db.CreateUserParams{
			Username: cfg.AdminUsername, PasswordHash: hash, Email: "", Role: 1, Now: now,
		}); err != nil {
			return err
		}
		log.Printf("[seed] 已创建管理员账号 %s（请尽快修改密码）", cfg.AdminUsername)
	}
	return nil
}

// migrateSessions 为已存在的旧库补齐 sessions.rotated_to 列（新库由 schema.sql 建好）。
func migrateSessions(db *sql.DB) error {
	var cnt int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('sessions') WHERE name = 'rotated_to'`,
	).Scan(&cnt); err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}
	_, err := db.Exec(`ALTER TABLE sessions ADD COLUMN rotated_to TEXT NOT NULL DEFAULT ''`)
	return err
}

// migrateOrders 为旧库补齐 orders 表的上游对接字段（新库由 schema.sql 建好）。
func migrateOrders(db *sql.DB) error {
	cols := map[string]string{
		"upstream_trade_no": "TEXT NOT NULL DEFAULT ''",
		"goods_key":         "TEXT NOT NULL DEFAULT ''",
		"quantity":          "INTEGER NOT NULL DEFAULT 0",
		"unit_price":        "INTEGER NOT NULL DEFAULT 0",
		"payurl":            "TEXT NOT NULL DEFAULT ''",
		"qrcode":            "TEXT NOT NULL DEFAULT ''",
		"buyer_contact":     "TEXT NOT NULL DEFAULT ''",
		"query_pwd":         "TEXT NOT NULL DEFAULT ''",
		"cards":             "TEXT NOT NULL DEFAULT ''",
	}
	for name, def := range cols {
		var cnt int
		if err := db.QueryRow(
			`SELECT COUNT(*) FROM pragma_table_info('orders') WHERE name = ?`, name,
		).Scan(&cnt); err != nil {
			return err
		}
		if cnt > 0 {
			continue
		}
		if _, err := db.Exec(`ALTER TABLE orders ADD COLUMN ` + name + ` ` + def); err != nil {
			return err
		}
	}
	return nil
}

func (d *Database) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		now := time.Now().Unix()
		if err := d.Q.DeleteExpiredSessions(ctx, now); err != nil {
			log.Printf("[cleanup] sessions: %v", err)
		}
		if _, err := d.Q.CloseExpiredOrders(ctx, now); err != nil {
			log.Printf("[cleanup] orders: %v", err)
		}
		cancel()
	}
}
