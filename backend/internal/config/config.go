package config

import (
	"log"
	"os"
	"strings"

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
	viper.AddConfigPath("./internal/config")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")

	// 设置默认值
	setDefaults()

	// 启用环境变量支持
	viper.AutomaticEnv()

	// 读取配置文件（可选，如果文件不存在则使用环境变量和默认值）
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("未找到配置文件，使用环境变量和默认配置")
		} else {
			log.Printf("警告: 读取配置文件失败: %v", err)
		}
	}

	AppConfig = &Config{}
	if err := viper.Unmarshal(AppConfig); err != nil {
		log.Fatalf("解析配置失败: %v", err)
	}

	// 环境变量覆盖（优先级最高）
	overrideFromEnv()

	log.Println("配置加载成功")
}

// overrideFromEnv 从环境变量覆盖敏感配置
func overrideFromEnv() {
	if secret := os.Getenv("BLOG_JWT_SECRET"); secret != "" {
		AppConfig.JWT.Secret = secret
		log.Println("JWT Secret 已从环境变量加载")
	}

	if password := os.Getenv("BLOG_ADMIN_PASSWORD"); password != "" {
		// 这个值会在 main.go 中使用
		log.Println("管理员密码已从环境变量加载")
	}

	if driver := strings.TrimSpace(os.Getenv("BLOG_DATABASE_DRIVER")); driver != "" {
		AppConfig.Database.Driver = strings.ToLower(driver)
		log.Println("数据库驱动已从环境变量加载")
	}

	if dsn := os.Getenv("BLOG_DATABASE_DSN"); dsn != "" {
		AppConfig.Database.DSN = dsn
		log.Println("数据库 DSN 已从环境变量加载")
	}

	if redisPassword := os.Getenv("BLOG_REDIS_PASSWORD"); redisPassword != "" {
		AppConfig.Redis.Password = redisPassword
		log.Println("Redis 密码已从环境变量加载")
	}
}

// GetEnv 获取环境变量，如果不存在则返回默认值
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
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
