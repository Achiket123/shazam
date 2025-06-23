package main

import (
	"fmt"
	"os"
	"runtime"
	"shazam/internal/api/search"
	"shazam/internal/audio"
	"shazam/internal/db"
	"shazam/internal/fingerprint"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func main() {
	DB := db.EstablishConn()
	DB.AutoMigrate(&db.Fingerprint{})

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept"},
		AllowOrigins: []string{"*"},
	}))
	r.POST("/search", search.RecogniseSong)
	r.Run("192.168.0.104:8081")

	// files, err := os.ReadDir("samples")
	// if err != nil {
	// 	panic(err)
	// }

	// for _, file := range files {
	// 	splitData := strings.Split(file.Name(), ".")
	// 	var fileName string
	// 	if len(splitData) > 2 {
	// 		fileName = strings.Join(splitData[:len(splitData)-1], ".")

	// 	} else {
	// 		fileName = splitData[0]

	// 	}
	// 	fmt.Printf("Processing file: %s\n", fileName)

	fileName := "output.wav"
	file, err := os.Open(fileName)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	samples, err := audio.DownSamplingAudio(file)
	if err != nil {
		panic(err)
	}
	hashes := fingerprint.Fingerprint(samples, fileName)
	fmt.Println(len(hashes))
	matches := search.MatchHashes(hashes, DB)
	fmt.Println(len(matches))
	if len(matches) > 0 {
		fmt.Println(matches)
	}
	// CreateHash(hashes, DB)
	runtime.GC()
	// }

}

func CreateHash(hashes []db.Fingerprint, DB *gorm.DB) {
	if err := DB.CreateInBatches(&hashes, 10000).Error; err != nil {
		panic(err)
	}
}
