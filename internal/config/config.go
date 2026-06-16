// Package config 负责应用配置的加载和管理
// 使用 Viper 实现多来源配置合并，优先级：命令行参数 > 环境变量(AIM_*) > yaml > 默认值
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config 应用全局配置结构体
type Config struct {
	Port   int    `mapstructure:"port"`   // HTTP 服务监听端口
	DBPath string `mapstructure:"db_path"` // SQLite 数据库文件路径
}

// 默认配置模板，包含中文注释说明每个配置项的用途
const defaultConfigTemplate = `# AiResourceManager 配置文件
# 首次启动时自动生成，可根据需要修改

# 服务监听端口（默认 3678）
port: 3678

# 数据库文件路径（默认在 ~/.aiManager/data/aimanager.db）
# 支持相对路径（相对于 ~/.aiManager/）和绝对路径
db_path: "~/.aiManager/data/aimanager.db"
`

// Load 加载配置文件并返回配置结构体
// 参数 cfgPath: 指定配置文件路径，为空时使用默认路径 ~/.aiManager/config.yaml
// 返回: 解析后的 Config 指针和可能的错误
func Load(cfgPath string) (*Config, error) {
	// 确定配置目录
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户主目录失败: %w", err)
	}
	configDir := filepath.Join(homeDir, ".aiManager")

	// 确保配置目录存在
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("创建配置目录失败: %w", err)
	}

	defaultCfgPath := filepath.Join(configDir, "config.yaml")

	// 如果默认配置文件不存在，生成带中文注释的配置文件
	if _, err := os.Stat(defaultCfgPath); os.IsNotExist(err) {
		if writeErr := os.WriteFile(defaultCfgPath, []byte(defaultConfigTemplate), 0644); writeErr != nil {
			return nil, fmt.Errorf("生成默认配置文件失败: %w", writeErr)
		}
	}

	// 设置 Viper 默认值
	viper.SetDefault("port", 3678)
	viper.SetDefault("db_path", filepath.Join(configDir, "data", "aimanager.db"))

	// 配置文件路径
	if cfgPath != "" {
		viper.SetConfigFile(cfgPath)
	} else {
		viper.SetConfigFile(defaultCfgPath)
	}

	// 环境变量绑定，前缀 AIM_
	viper.SetEnvPrefix("AIM")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		// 配置文件不存在时使用默认值，不视为错误
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
	}

	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 展开 db_path 中的 ~ 为实际主目录
	if strings.HasPrefix(cfg.DBPath, "~") {
		cfg.DBPath = filepath.Join(homeDir, cfg.DBPath[1:])
	}

	// 确保数据库目录存在
	dbDir := filepath.Dir(cfg.DBPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}

	return cfg, nil
}
