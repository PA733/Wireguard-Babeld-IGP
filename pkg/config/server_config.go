package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// ServerConfig 服务端配置
type ServerConfig struct {
	// 服务器配置
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
		TLS  struct {
			Enabled bool   `yaml:"enabled"`
			Cert    string `yaml:"cert"`
			Key     string `yaml:"key"`
		} `yaml:"tls"`
	} `yaml:"server"`

	// 网络配置
	Network struct {
		BasePort          int    `yaml:"base_port"`
		IPv4Range         string `yaml:"ipv4_range"`
		IPv4Template      string `yaml:"ipv4_template"`
		IPv4NodeTemplate  string `yaml:"ipv4_node_template"`
		IPv6Range         string `yaml:"ipv6_range"`
		IPv6Template      string `yaml:"ipv6_template"`
		IPv6NodeTemplate  string `yaml:"ipv6_node_template"`
		LinkLocalTemplate string `yaml:"link_local_template"`
		LinkLocalNet      string `yaml:"link_local_net"`
		BabelMulticast    string `yaml:"babel_multicast"`
		BabelPort         int    `yaml:"babel_port"`
	} `yaml:"network"`

	// 日志配置
	Log struct {
		Debug bool   `yaml:"debug"`
		File  string `yaml:"file"`
	} `yaml:"log"`

	// 存储配置
	Storage struct {
		Type   string `yaml:"type"`
		SQLite struct {
			Path string `yaml:"path"`
		} `yaml:"sqlite"`
		Postgres struct {
			Host     string `yaml:"host"`
			Port     int    `yaml:"port"`
			User     string `yaml:"user"`
			Password string `yaml:"password"`
			DBName   string `yaml:"dbname"`
			SSLMode  string `yaml:"sslmode"`
		} `yaml:"postgres"`
	} `yaml:"storage"`
}

// LoadServerConfig 加载服务端配置
func LoadServerConfig(path string, workspaceRoot string) (*ServerConfig, error) {
	cfg := &ServerConfig{}
	if err := LoadConfig(path, cfg); err != nil {
		return nil, err
	}

	// 处理相对路径
	if err := cfg.resolveRelativePaths(workspaceRoot); err != nil {
		return nil, fmt.Errorf("resolving paths: %w", err)
	}

	return cfg, nil
}

// Validate 实现Config接口
func (c *ServerConfig) Validate() error {
	if c.Server.Host == "" {
		return fmt.Errorf("server.host is required")
	}
	if c.Server.Port <= 0 {
		return fmt.Errorf("invalid server.port: %d", c.Server.Port)
	}
	if c.Network.BasePort <= 0 {
		return fmt.Errorf("invalid network.base_port: %d", c.Network.BasePort)
	}
	if c.Network.IPv4Range == "" {
		return fmt.Errorf("network.ipv4_range is required")
	}
	if c.Network.IPv6Range == "" {
		return fmt.Errorf("network.ipv6_range is required")
	}
	if c.Storage.Type == "" {
		return fmt.Errorf("storage.type is required")
	}
	return nil
}

// resolveRelativePaths 处理相对路径
func (c *ServerConfig) resolveRelativePaths(baseDir string) error {
	// 处理日志文件路径
	if c.Log.File != "" && !filepath.IsAbs(c.Log.File) {
		c.Log.File = filepath.Join(baseDir, c.Log.File)
	}

	// 处理SQLite数据库路径
	if c.Storage.Type == "sqlite" && !filepath.IsAbs(c.Storage.SQLite.Path) {
		c.Storage.SQLite.Path = filepath.Join(baseDir, c.Storage.SQLite.Path)
		// 确保数据库目录存在
		if err := os.MkdirAll(filepath.Dir(c.Storage.SQLite.Path), 0755); err != nil {
			return fmt.Errorf("creating sqlite directory: %w", err)
		}
	}

	return nil
}

// DefaultServerConfig 返回默认服务端配置
func DefaultServerConfig() *ServerConfig {
	cfg := &ServerConfig{}

	// 服务器配置
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 8080

	// 网络配置
	cfg.Network.BasePort = 36420
	cfg.Network.IPv4Range = "10.42.0.0/16"
	cfg.Network.IPv4Template = "10.42.0.0/16"
	cfg.Network.IPv4NodeTemplate = "10.42.0.0/16"
	cfg.Network.IPv6Range = "2a13:a5c7:21ff::/48"
	cfg.Network.IPv6Template = "2a13:a5c7:21ff::/48"
	cfg.Network.IPv6NodeTemplate = "2a13:a5c7:21ff::/48"
	cfg.Network.LinkLocalTemplate = "fe80::/64"
	cfg.Network.LinkLocalNet = "fe80::/64"
	cfg.Network.BabelMulticast = "ff02::1:6/128"
	cfg.Network.BabelPort = 6696

	// 日志配置
	cfg.Log.Debug = false
	cfg.Log.File = "data/mesh-server.log"

	// 存储配置
	cfg.Storage.Type = "sqlite"
	cfg.Storage.SQLite.Path = "data/mesh.db"

	return cfg
}
