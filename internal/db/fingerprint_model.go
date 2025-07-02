package db

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Fingerprint struct {

	Hash       uint32  `json:"hash"`
	AnchorTime float64 `json:"anchor_time"`
	SongID     string
}

var DB *gorm.DB

func EstablishConn() *gorm.DB {

	godotenv.Load()
	HOST, _ := os.LookupEnv("HOST")
	DBNAME, _ := os.LookupEnv("DBNAME")
	USER, _ := os.LookupEnv("USER")
	PASS, _ := os.LookupEnv("PASS")
	SSLMODE, _ := os.LookupEnv("SSLMODE")
	PORT, _ := os.LookupEnv("PORT")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=Asia/Shanghai", HOST, USER, PASS, DBNAME, PORT, SSLMODE)
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
