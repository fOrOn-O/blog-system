package config

import (
	"log"

	"github.com/spf13/viper"
)

// Config 应用配置结构体
type Config struct {
	App      App      `mapstructure:"app"`
	Database Database `mapstructure:"database"`
	Redis    Redis    `mapstructure:"redis"`
	JWT      JWT      `mapstructure:"jwt"`
}

// App 应用基础配置
type App struct {
	Name string `mapstructure:"name"`
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"` // debug, release, test
}

// Database 数据库配置
type Database struct {
	Driver string `mapstructure:"driver"` // sqlite, mysql
	DSN    string `mapstructure:"dsn"`
}

// Redis Redis配置
type Redis struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// JWT JWT配置
type JWT struct {
	Secret     string `mapstructure:"secret"`
	Expiration int    `mapstructure:"expiration"` // 小时
}

// AppConfig 全局配置实例
var AppConfig *Config

// InitConfig 初始化配置
func InitConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")

	// 设置默认值
	setDefaults()

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("未找到配置文件，使用默认配置")
		} else {
			log.Fatalf("读取配置文件失败: %v", err)
		}
	}

	AppConfig = &Config{}
	if err := viper.Unmarshal(AppConfig); err != nil {
		log.Fatalf("解析配置文件失败: %v", err)
	}

	log.Println("配置加载成功")
}

// setDefaults 设置默认配置值
func setDefaults() {
	// App
	viper.SetDefault("app.name", "blog-system")
	viper.SetDefault("app.port", ":8080")
	viper.SetDefault("app.mode", "debug")

	// Database
	viper.SetDefault("database.driver", "sqlite")
	viper.SetDefault("database.dsn", "blog.db")

	// Redis
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", "6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)

	// JWT
	viper.SetDefault("jwt.secret", "blog-system-secret-key")
	viper.SetDefault("jwt.expiration", 24)
}
