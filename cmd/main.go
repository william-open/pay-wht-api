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
	// 设置可信代理 IP（如本地或内网）
	r.SetTrustedProxies([]string{"127.0.0.1", "192.168.0.0/16"})
	r.Use(middleware.Recover())

	v1 := r.Group("/api/v1")
	{
		oh := handler.NewOrderHandler()
		v1.POST("/orders", middleware.AuthHMAC(), oh.Create)
		v1.GET("/orders/:id", oh.Get)
		v1.GET("/orders", oh.List)
	}

	addr := ":" + config.C.Server.Port
	log.Printf("listening %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
