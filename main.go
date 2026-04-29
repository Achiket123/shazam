package main

import (
	"shazam/internal/api/search"
	"shazam/internal/api/upload"
	"shazam/internal/db"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	database := db.EstablishConn()
	// AutoMigrate creates/updates the fingerprints table.
	// After the uint32→uint64 hash upgrade, drop the old table first:
	//   database.Migrator().DropTable(&db.Fingerprint{})
	database.AutoMigrate(&db.Fingerprint{})

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowMethods: []string{"GET", "POST"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept"},
		AllowOrigins: []string{"*"},
	}))

	r.POST("/upload", upload.FingerprintAPI) // index a song
	r.POST("/search", search.RecogniseSong)  // recognise a clip

	r.Run(":8080")

}

