package dal

import (
	"fmt"
	"log"
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
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("connect main db failed: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxIdleConns(c.MaxIdleConns)
	sqlDB.SetMaxOpenConns(c.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(2 * time.Hour)
	MainDB = db
}
