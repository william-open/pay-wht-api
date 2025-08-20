package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"wht-order-api/internal/config"
	"wht-order-api/internal/dal"
	"wht-order-api/internal/handler"
	"wht-order-api/internal/idgen"
	"wht-order-api/internal/middleware"
	"wht-order-api/internal/mq"
)

func main() {
	gin.SetMode(gin.DebugMode) // 默认就是 debug，可显式设置
	// load config env
	config.Init()

	// init infra
	dal.InitMainDB()
	dal.InitOrderDB()
	dal.InitRedis()
	dal.InitRabbitMQ()

	// idgen
	idgen.Init(1)

	// start consumers
	go mq.StartConsumers()

	// http server
	if config.C.Server.Mode != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(middleware.RequestLogger())
	r.Use(gin.Recovery()) //注册 gin.Recovery() 中间件（捕获 panic）
	// 设置可信代理 IP（如本地或内网）
	err := r.SetTrustedProxies([]string{"127.0.0.1", "192.168.0.0/16"})
	if err != nil {
		log.Fatal(err)
	}
	r.Use(middleware.Recover())

	v1 := r.Group("/api/v1")
	{
		oh := handler.NewOrderHandler()
		v1.POST("/order/create", middleware.AuthMD5Sign(), oh.Create)
		v1.GET("/order/:id", oh.Get)
	}

	addr := ":" + config.C.Server.Port
	log.Printf("listening %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
