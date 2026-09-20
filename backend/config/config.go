// Package config 提供 Fast Pay 配置的单例装配（fun.Wired）。
package config

import (
	"encoding/json"
	"log"
	"os"
)

// Config 应用配置。首次启动若无 config.json 会写出默认值。
type Config struct {
	Port          uint16 `json:"port"`           // 监听端口
	DBPath        string `json:"db_path"`        // SQLite 文件路径
	SiteName      string `json:"site_name"`      // 站点名称
	FrontendURL   string `json:"frontend_url"`   // 前端地址（收银台跳转用）
	AdminUsername string `json:"admin_username"` // 初始管理员账号
	AdminPassword string `json:"admin_password"` // 初始管理员密码
	CORSOrigins   string `json:"cors_origins"`   // 跨域白名单，逗号分隔；开发环境走 Vite 代理可不填
}

// New 由 fun.Wired 调用：加载 config.json，缺省则生成默认配置文件。
func (c *Config) New() error {
	path := os.Getenv("EPAY_CONFIG")
	if path == "" {
		path = "config.json"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			*c = defaultConfig()
			if raw, mErr := json.MarshalIndent(c, "", "  "); mErr == nil {
				_ = os.WriteFile(path, raw, 0o644)
				log.Printf("[config] 未找到 %s，已生成默认配置", path)
			}
			return nil
		}
		return err
	}
	if err := json.Unmarshal(data, c); err != nil {
		return err
	}
	// 兜底：关键字段为空时回落默认值
	d := defaultConfig()
	if c.Port == 0 {
		c.Port = d.Port
	}
	if c.DBPath == "" {
		c.DBPath = d.DBPath
	}
	if c.FrontendURL == "" {
		c.FrontendURL = d.FrontendURL
	}
	if c.AdminUsername == "" || c.AdminPassword == "" {
		c.AdminUsername, c.AdminPassword = d.AdminUsername, d.AdminPassword
	}
	return nil
}

func defaultConfig() Config {
	return Config{
		Port:          9200,
		DBPath:        "funpay.db",
		SiteName:      "Fast Pay",
		FrontendURL:   "http://localhost:5173",
		AdminUsername: "admin",
		AdminPassword: "admin123",
		CORSOrigins:   "http://localhost:5173,http://127.0.0.1:5173",
	}
}
