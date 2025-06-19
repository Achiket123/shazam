package db

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Fingerprint struct {
	AnchorFreq float64
	TargetFreq float64
	TimeDelta  float64
	AnchorTime float64 // The absolute time of the anchor peak
	Hash       int64
	SongID     string
}

var DB *gorm.DB

var dsn = "host=localhost user=achiket password=8759 dbname=achiket port=5432 sslmode=disable TimeZone=Asia/Shanghai"

func EstablishConn() *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic("failed to connect database")
	}
	fmt.Println("connected to database")
	DB = db
	return db
}
