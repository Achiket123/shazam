package db

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Fingerprint is the core DB record.
// Hash is uint64 (40 bits used) — far fewer collisions than the old uint32.
// ⚠️  Breaking schema change from uint32: drop and re-create the fingerprints table,
//
//	then re-fingerprint all songs.
type Fingerprint struct {
	Hash       uint64  `json:"hash"        gorm:"index;not null"`
	AnchorTime float64 `json:"anchor_time" gorm:"not null"`
	SongID     string  `json:"song_id"     gorm:"index;not null"`
}

var DB *gorm.DB

func EstablishConn() *gorm.DB {
	godotenv.Load(".env")

	host, _ := os.LookupEnv("HOST")
	dbname, _ := os.LookupEnv("DBNAME")
	user, _ := os.LookupEnv("USER")
	pass, _ := os.LookupEnv("PASS")
	sslmode, _ := os.LookupEnv("SSLMODE")
	port, _ := os.LookupEnv("DBPORT")

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=Asia/Kolkata",
		host, user, pass, dbname, port, sslmode,
	)
	log.Default().Println(dsn)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		panic(fmt.Errorf("failed to connect to database: %w", err))
	}

	sqlDB, err := db.DB()
	if err != nil {
		panic(err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	fmt.Println("[db] connected")
	DB = db
	return db
}
