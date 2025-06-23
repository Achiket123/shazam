package main

import (
	"runtime"
	"shazam/internal/api/search"
	"shazam/internal/db"

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
	r.Run()

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

	// CreateHash(hashes, DB)
	runtime.GC()
	// }

}

func CreateHash(hashes []db.Fingerprint, DB *gorm.DB) {
	if err := DB.CreateInBatches(&hashes, 10000).Error; err != nil {
		panic(err)
	}
}
