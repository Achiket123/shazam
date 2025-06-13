package db

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Fingerprint struct {
	gorm.Model
	Frequency1 int
	Frequency2 int
	Hash       string `gorm:"uniqueIndex:idx_hash_time_offset"`
	TimeOffset int    `gorm:"uniqueIndex:idx_hash_time_offset"`
	SongID     string
}

var dsn = "host=localhost user=achiket password=8759 dbname=achiket port=5432 sslmode=disable TimeZone=Asia/Shanghai"

func EstablishConn() *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	err = db.AutoMigrate(&Fingerprint{})
	if err != nil {
		fmt.Println("failed to migrate database", err)
		panic("failed to migrate database")
	}
	return db
}
