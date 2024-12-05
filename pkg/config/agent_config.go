package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// AgentConfig 代理配置结构
type AgentConfig struct {
	// 节点标识
	NodeID int    `yaml:"node_id"`
	Token  string `yaml:"token"`

	// 服务端连接信息
	Server struct {
		Address     string `yaml:"address"`      // HTTP API地址
		GRPCAddress string `yaml:"grpc_address"` // gRPC服务地址
		TLS         struct {
			Enabled bool   `yaml:"enabled"`
			CACert  string `yaml:"ca_cert"`
		} `yaml:"tls"`
	} `yaml:"server"`

	// WireGuard配置
	WireGuard struct {
		ConfigPath string `yaml:"config_path"` // WireGuard配置文件路径
		Prefix     string `yaml:"prefix"`      // WireGuard配置文件前缀
	} `yaml:"wireguard"`

	// Babeld配置
	Babel struct {
		ConfigPath string `yaml:"config_path"` // Babeld配置文件路径
		BinPath    string `yaml:"bin_path"`    // babeld命令路径
	} `yaml:"babel"`

	// 运行时配置
	Runtime struct {
		LogPath     string `yaml:"log_path"`     // 日志文件路径
		LogLevel    string `yaml:"log_level"`    // 日志级别
		DryRun      bool   `yaml:"dry_run"`      // 调试模式
		MetricsPort int    `yaml:"metrics_port"` // 指标监控端口
	} `yaml:"runtime"`
}

// LoadAgentConfig 加载客户端配置
func LoadAgentConfig(path string, workspaceRoot string) (*AgentConfig, error) {
	// 读取配置文件
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// 解析配置
	cfg := &AgentConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// 验证必要字段
	if cfg.NodeID == 0 {
		return nil, fmt.Errorf("node_id is required")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("token is required")
	}
	if cfg.Server.Address == "" {
		return nil, fmt.Errorf("server.address is required")
	}
	if cfg.Server.GRPCAddress == "" {
		return nil, fmt.Errorf("server.grpc_address is required")
	}

	return cfg, nil
}

// DefaultAgentConfig 返回默认配置
func DefaultAgentConfig() *AgentConfig {
	cfg := &AgentConfig{}
	cfg.Server.Address = "http://localhost:8080"
	cfg.Server.GRPCAddress = "localhost:9090"
	cfg.Runtime.LogLevel = "info"
	cfg.Runtime.MetricsPort = 9100
	return cfg
}
