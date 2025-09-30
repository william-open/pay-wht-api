package main

import (
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"log"
	"wht-order-api/internal/config"
	"wht-order-api/internal/dal"
	"wht-order-api/internal/handler"
	"wht-order-api/internal/idgen"
	"wht-order-api/internal/logger"
	"wht-order-api/internal/middleware"
	"wht-order-api/internal/mq"
	"wht-order-api/internal/shard"
	"wht-order-api/internal/system"
)

func main() {
	_ = godotenv.Load(".env")  // 自动加载环境变量
	gin.SetMode(gin.DebugMode) // 默认就是 debug，可显式设置
	// load config env
	config.Init()

	// init infra
	dal.InitMainDB()
	dal.InitOrderDB()
	dal.InitRedis()
	mqErr := dal.InitRabbitMQ()
	if mqErr != nil {
		log.Fatalf("启动消息队列失败,错误信息: %v", mqErr)
		return
	}

	// idgen
	// 初始化 ID 生成器
	idgen.InitFromEnv()
	// 启动时间回拨检测
	go idgen.CheckSystemClock()
	// 初始化分片引擎
	shard.InitShardEngines()
	logger.InitLogger()
	// 初始化一些系统配置参数
	system.Config()
	// start MQ receive consumer
	go mq.StartReceiveConsumer()
	// start MQ payout consumer
	go mq.StartPayoutConsumer()
	// 2. 初始化全局 Publisher

	// http server
	if config.C.Server.Mode != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// 1. 初始化 trace_id、请求体、IP、UA、开始时间
	r.Use(middleware.TraceAuditMiddleware())

	// 2. 捕获 panic，避免中断日志写入
	r.Use(gin.Recovery())

	// 3. 其他中间件（如权限、认证）
	r.Use(middleware.RequestLogger()) // 如果你还需要这个

	// 4. 注册 gin.Recovery() 中间件（捕获 panic）
	r.Use(middleware.Recover())

	// 设置可信代理 IP（如本地或内网）
	err := r.SetTrustedProxies([]string{"127.0.0.1", "192.168.0.0/16"})
	if err != nil {
		log.Fatal(err)
	}

	v1 := r.Group("/api/v1")
	{
		receive := handler.NewReceiveOrderHandler()
		payout := handler.NewPayoutOrderHandler()
		account := handler.NewAccountHandler()
		// 代收网关
		v1.POST("/order/receive/create", middleware.ReceiveCreateAuth(), receive.ReceiveOrderCreate)
		v1.POST("/order/receive/query", middleware.ReceiveQueryAuth(), receive.ReceiveOrderQuery)
		// 代付网关
		v1.POST("/order/payout/create", middleware.PayoutCreateAuth(), payout.PayoutOrderCreate)
		v1.POST("/order/payout/query", middleware.PayoutQueryAuth(), payout.PayoutOrderQuery)
		// 查询商户账户信息
		v1.POST("/query/account/balance", middleware.AccountAuth(), account.Query)
	}

	addr := ":" + config.C.Server.Port
	log.Printf("listening %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
