package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// AgentConfig 代理配置结构
type AgentConfig struct {
	// 节点标识
	NodeID int    `yaml:"node_id"`
	Token  string `yaml:"token"`

	// 服务端连接信息
	Server struct {
		Address string `yaml:"address"` // 服务器地址
		TLS     struct {
			Enabled bool   `yaml:"enabled"`
			CACert  string `yaml:"ca_cert"`
		} `yaml:"tls"`
	} `yaml:"server"`

	// 本地服务配置
	WireGuard struct {
		ConfigPath string `yaml:"config_path"` // WireGuard配置文件路径
		BinPath    string `yaml:"bin_path"`    // wg命令路径
	} `yaml:"wireguard"`

	Babel struct {
		ConfigPath string `yaml:"config_path"` // Babeld配置文件路径
		BinPath    string `yaml:"bin_path"`    // babeld命令路径
	} `yaml:"babel"`

	// 运行时配置
	Runtime struct {
		LogPath     string `yaml:"log_path"`     // 日志文件路径
		LogLevel    string `yaml:"log_level"`    // 日志级别
		DryRun      bool   `yaml:"dry_run"`      // 是否为调试模式
		MetricsPort int    `yaml:"metrics_port"` // 指标监控端口
	} `yaml:"runtime"`
}

// LoadAgentConfig 加载客户端配置
func LoadAgentConfig(path string, workspaceRoot string) (*AgentConfig, error) {
	cfg := &AgentConfig{}
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
func (c *AgentConfig) Validate() error {
	if c.NodeID <= 0 {
		return fmt.Errorf("invalid node_id: %d", c.NodeID)
	}
	if c.Token == "" {
		return fmt.Errorf("token is required")
	}
	if c.Server.Address == "" {
		return fmt.Errorf("server.address is required")
	}
	return nil
}

// resolveRelativePaths 处理相对路径
func (c *AgentConfig) resolveRelativePaths(baseDir string) error {
	resolvePath := func(path *string) {
		if *path != "" && !filepath.IsAbs(*path) {
			*path = filepath.Join(baseDir, *path)
			// 确保目录存在
			if dir := filepath.Dir(*path); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					// 这里我们只记录错误，不中断执行
					fmt.Printf("Warning: failed to create directory %s: %v\n", dir, err)
				}
			}
		}
	}

	resolvePath(&c.WireGuard.ConfigPath)
	resolvePath(&c.WireGuard.BinPath)
	resolvePath(&c.Babel.ConfigPath)
	resolvePath(&c.Babel.BinPath)
	resolvePath(&c.Runtime.LogPath)

	return nil
}

// DefaultAgentConfig 返回默认客户端配置
func DefaultAgentConfig() *AgentConfig {
	return &AgentConfig{
		Server: struct {
			Address string `yaml:"address"`
			TLS     struct {
				Enabled bool   `yaml:"enabled"`
				CACert  string `yaml:"ca_cert"`
			} `yaml:"tls"`
		}{
			Address: "http://localhost:8080",
			TLS: struct {
				Enabled bool   `yaml:"enabled"`
				CACert  string `yaml:"ca_cert"`
			}{
				Enabled: false,
				CACert:  "",
			},
		},
		Runtime: struct {
			LogPath     string `yaml:"log_path"`
			LogLevel    string `yaml:"log_level"`
			DryRun      bool   `yaml:"dry_run"`
			MetricsPort int    `yaml:"metrics_port"`
		}{
			LogPath:     "data/agent.log",
			LogLevel:    "info",
			DryRun:      false,
			MetricsPort: 9100,
		},
	}
}
