package main

import (
	"log"
	"net/http"
	"shazam/internal/api/search"
	"shazam/internal/api/upload"
	"shazam/internal/db"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	database := db.EstablishConn()
	// AutoMigrate creates/updates the fingerprints table.
	// After the uint32→uint64 hash upgrade, drop the old table first:
	//   database.Migrator().DropTable(&db.Fingerprint{})
	err := database.AutoMigrate(&db.Fingerprint{})
	if err != nil {
		log.Default().Fatal(err)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowMethods: []string{"GET", "POST"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept"},
		AllowOrigins: []string{"*"},
	}))
	r.Use(func(c *gin.Context) {
		log.Default().Println("LOG:")
		log.Default().Println(c.Request.RequestURI)
		log.Default().Println(c.Request.RequestURI)

		c.Next()
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"Status":    "OK",
			"TimeStamp": time.Now(),
		})

	})
	r.POST("/upload", upload.FingerprintAPI) // index a song
	r.POST("/search", search.RecogniseSong)  // recognise a clip
	r.GET("/stream", gin.WrapF(func(w http.ResponseWriter, req *http.Request) {
		search.StreamSearch(w, req)
	}))
	log.Default().Println("Listening...")
	r.Run("0.0.0.0:8080")

}
