package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 通用配置接口
type Config interface {
	Validate() error
}

// LoadConfig 从文件加载配置
func LoadConfig(path string, cfg Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validating config: %w", err)
	}

	return nil
}
