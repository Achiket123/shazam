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
	AnchorTime float64
	Hash       int64 `json:"hash"`
	SongID     string
}

var DB *gorm.DB

var dsn = "host=localhost user=postgres password=8759 dbname=achiket port=5432 sslmode=disable TimeZone=Asia/Shanghai"

func EstablishConn() *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("connected to database")
	DB = db
	return db
}
