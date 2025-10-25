package config

import (
	"flag"
	"log"
	"strings"

	"github.com/spf13/viper"
)

type ServerCfg struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}
type MysqlCfg struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Database     string `mapstructure:"database"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	Charset      string `mapstructure:"charset"`
	MaxIdleConns int    `mapstructure:"maxIdleConns"`
	MaxOpenConns int    `mapstructure:"maxOpenConns"`
}
type RabbitProducerCfg struct {
	Name         string `mapstructure:"name"`
	Exchange     string `mapstructure:"exchange"`
	ExchangeType string `mapstructure:"exchange_type"`
	RoutingKey   string `mapstructure:"routing_key"`
}

type RabbitConsumerCfg struct {
	Name         string `mapstructure:"name"`
	Queue        string `mapstructure:"queue"`
	Exchange     string `mapstructure:"exchange"`
	ExchangeType string `mapstructure:"exchange_type"`
	RoutingKey   string `mapstructure:"routing_key"`
	Durable      bool   `mapstructure:"durable"`
	AutoDelete   bool   `mapstructure:"auto_delete"`
	Exclusive    bool   `mapstructure:"exclusive"`
	NoWait       bool   `mapstructure:"no_wait"`
}

type RabbitCfg struct {
	Host          string              `mapstructure:"host"`
	Port          int                 `mapstructure:"port"`
	Username      string              `mapstructure:"username"`
	Password      string              `mapstructure:"password"`
	VirtualHost   string              `mapstructure:"virtual_host"`
	PrefetchCount int                 `mapstructure:"prefetch_count"`
	Producers     []RabbitProducerCfg `mapstructure:"producers"`
	Consumers     []RabbitConsumerCfg `mapstructure:"consumers"`
}

type RedisCfg struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}
type SecurityCfg struct {
	HMACSecret string `mapstructure:"hmacSecret"`
}
type OrderCfg struct {
	ShardsPerMonth   int `mapstructure:"shardsPerMonth"`
	CreateTimeoutSec int `mapstructure:"createTimeoutSec"`
}

type UpstreamCfg struct {
	ReceiveApiUrl string `mapstructure:"receiveApiUrl"`
	PayoutApiUrl  string `mapstructure:"payoutApiUrl"`
	AuthToken     string `mapstructure:"authToken"`
}

type ProjectCfg struct {
	Name      string `mapstructure:"name"`
	Version   string `mapstructure:"version"`
	Copyright string `mapstructure:"copyright"`
}

type Root struct {
	Server     ServerCfg   `mapstructure:"server"`
	MysqlMain  MysqlCfg    `mapstructure:"mysql_main"`
	MysqlOrder MysqlCfg    `mapstructure:"mysql_order"`
	RabbitMQ   RabbitCfg   `mapstructure:"rabbitmq"` // ✅ 替换原来的简化版
	Redis      RedisCfg    `mapstructure:"redis"`
	Security   SecurityCfg `mapstructure:"security"`
	Order      OrderCfg    `mapstructure:"order"`
	Upstream   UpstreamCfg `mapstructure:"upstream"`
	Project    ProjectCfg  `mapstructure:"project"`
}

var C Root

func Init() {
	env := flag.String("env", "dev", "config env: dev|prod")
	log.Printf("启动环境: %+v", *env)
	flag.Parse()

	v := viper.New()
	v.SetConfigFile("config/config." + *env + ".yaml")
	if err := v.ReadInConfig(); err != nil {
		log.Fatalf("read config file failed: %v", err)
	}
	if err := v.Unmarshal(&C); err != nil {
		log.Fatalf("unmarshal config failed: %v", err)
	}

	// sane defaults
	if strings.TrimSpace(C.Server.Port) == "" {
		C.Server.Port = "8080"
	}
	if C.Order.ShardsPerMonth <= 0 {
		C.Order.ShardsPerMonth = 4
	}
	if C.Order.CreateTimeoutSec <= 0 {
		C.Order.CreateTimeoutSec = 3
	}
}
