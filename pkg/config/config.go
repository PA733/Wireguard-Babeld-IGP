package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Network NetworkConfig `yaml:"network"`
	Log     LogConfig     `yaml:"log"`
	Storage StorageConfig `yaml:"storage"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type NetworkConfig struct {
	BasePort       int    `yaml:"base_port"`
	IPv4Range      string `yaml:"ipv4_range"`
	IPv6Range      string `yaml:"ipv6_range"`
	LinkLocalNet   string `yaml:"link_local_net"`
	BabelMulticast string `yaml:"babel_multicast"`
	BabelPort      int    `yaml:"babel_port"`
}

type LogConfig struct {
	Debug bool   `yaml:"debug"`
	File  string `yaml:"file"`
}

type StorageConfig struct {
	Type     string         `yaml:"type"`
	SQLite   SQLiteConfig   `yaml:"sqlite"`
	Postgres PostgresConfig `yaml:"postgres"`
}

type SQLiteConfig struct {
	Path string `yaml:"path"`
}

type PostgresConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	config := &Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return config, nil
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Network: NetworkConfig{
			BasePort:       36420,
			IPv4Range:      "10.42.0.0/16",
			IPv6Range:      "2a13:a5c7:21ff::/48",
			LinkLocalNet:   "fe80::/64",
			BabelMulticast: "ff02::1:6/128",
			BabelPort:      6696,
		},
		Log: LogConfig{
			Debug: false,
			File:  "mesh-backend.log",
		},
		Storage: StorageConfig{
			Type: "memory",
		},
	}
}
