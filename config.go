package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("获取家目录失败:", err)
		return "", err
	}
	return filepath.Join(home, ".yd.conf"), nil
}

func loadConfig() (*Config, error) {
	cfg, err := loadStoredConfig()
	if err != nil {
		return nil, err
	}
	if cfg.Session == nil || cfg.Session.Key == "" || cfg.Session.Secret == "" {
		return nil, fmt.Errorf("配置文件中缺少有效的 session.sessionKey / session.sessionSecret")
	}
	return cfg, nil
}

func loadStoredConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开配置文件失败 %s: %w\n请先执行 yd login", path, err)
	}
	defer f.Close()

	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	if cfg.WorkDir != "" && !strings.HasPrefix(cfg.WorkDir, "/") {
		return nil, fmt.Errorf("配置文件中的 workDir 必须是绝对路径，例如 /同步盘")
	}
	if cfg.WorkDir != "" {
		cfg.WorkDir = cleanCloudPath(cfg.WorkDir)
	}
	return &cfg, nil
}

func saveConfig(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(cfg)
}
