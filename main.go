package main

import (
	"fmt"
	"os"
	"runtime"
	"shazam/internal/api/search"
	"shazam/internal/db"
	"shazam/internal/fingerprint"
	"strings"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"gorm.io/gorm"
)

func main() {
	DB := db.EstablishConn()
	DB.AutoMigrate(&db.Fingerprint{})
	// FingerPrint()
	searchSong()
	// r := gin.Default()
	// r.Use(cors.New(cors.Config{
	// 	AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
	// 	AllowHeaders: []string{"Origin", "Content-Type", "Accept"},
	// 	AllowOrigins: []string{"*"},
	// }))
	// searchSong()
	// r.POST("/search", search.RecogniseSong)
	// r.Run("192.168.0.104:8081")

	runtime.GC()
	// }

}

func CreateHash(hashes []db.Fingerprint, DB *gorm.DB) {
	if err := DB.CreateInBatches(&hashes, 10000).Error; err != nil {
		panic(err)
	}
}

func searchSong() {
	file, err := os.Open("output.wav")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	wav.NewDecoder(file)
	d := wav.NewDecoder(file)
	d.FwdToPCM()
	buf := audio.IntBuffer{
		Data: make([]int, d.PCMChunk.Size/2),
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  44100,
		},
	}

	_, err = d.PCMBuffer(&buf)
	if err != nil {
		panic(err)
	}
	samples := buf.AsFloatBuffer().Data
	fingerPrints := fingerprint.Fingerprint(&samples, "song") // Assuming fingerprint function takes []float64
	data, _ := search.MatchHashes(fingerPrints, db.DB)
	fmt.Println("data", data)

}

func FingerPrint() {
	files, err := os.ReadDir("assets/audio")
	if err != nil {
		panic(err)
	}
	i := 0

	for _, file := range files {

		splitData := strings.Split(file.Name(), ".")
		var fileName string
		if len(splitData) > 2 {
			fileName = strings.Join(splitData[:len(splitData)-1], ".")

		} else {
			fileName = splitData[0]

		}
		fmt.Printf("Processing file: %s\n", fileName)
		fileName = "assets/audio/" + fileName + ".wav"
		file, err := os.Open(fileName)
		if err != nil {
			panic(err)
		}
		defer file.Close()

		wav.NewDecoder(file)
		d := wav.NewDecoder(file)
		d.FwdToPCM()
		buf := audio.IntBuffer{
			Data: make([]int, d.PCMChunk.Size/2),
			Format: &audio.Format{
				NumChannels: 1,
				SampleRate:  44100,
			},
		}

		_, err = d.PCMBuffer(&buf)
		if err != nil {
			panic(err)
		}
		samples := buf.AsFloatBuffer().Data
		fingerPrints := fingerprint.Fingerprint(&samples, fileName)
		CreateHash(fingerPrints, db.DB)
		i++
		if i == 30 {
			break
		}
	}
}
