package dal

import (
	"fmt"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"time"

	"wht-order-api/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var MainDB *gorm.DB

func InitMainDB() {
	c := config.C.MysqlMain
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		c.Username, c.Password, c.Host, c.Port, c.Database, c.Charset)
	// 配置日志输出
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // 日志输出位置（这里输出到控制台）
		logger.Config{
			SlowThreshold:             time.Millisecond, // 慢 SQL 阈值
			LogLevel:                  logger.Info,      // 日志级别，设置为 Info 才会全部打印
			IgnoreRecordNotFoundError: false,            // 是否忽略 ErrRecordNotFound 错误
			Colorful:                  true,             // 彩色打印
		},
	)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		log.Fatalf("connect main db failed: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxIdleConns(c.MaxIdleConns)
	sqlDB.SetMaxOpenConns(c.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(2 * time.Hour)
	MainDB = db
}
