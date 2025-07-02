package main

import (
	"runtime"
	"shazam/internal/api/search"
	"shazam/internal/db"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	DB := db.EstablishConn()
	DB.AutoMigrate(&db.Fingerprint{})

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept"},
		AllowOrigins: []string{"*"},
	}))

	r.POST("/search", search.RecogniseSong)
	r.Run()

	runtime.GC()

}

// func FingerPrint() {
// 	files, err := os.ReadDir("assets/audio")
// 	if err != nil {
// 		panic(err)
// 	}
// 	i := 0

// 	for _, file := range files {

// 		splitData := strings.Split(file.Name(), ".")
// 		var fileName string
// 		if len(splitData) > 2 {
// 			fileName = strings.Join(splitData[:len(splitData)-1], ".")

// 		} else {
// 			fileName = splitData[0]

// 		}
// 		fmt.Printf("Processing file: %s\n", fileName)
// 		fileName = "assets/audio/" + fileName + ".wav"
// 		file, err := os.Open(fileName)
// 		if err != nil {
// 			panic(err)
// 		}
// 		defer file.Close()

// 		wav.NewDecoder(file)
// 		d := wav.NewDecoder(file)
// 		d.FwdToPCM()
// 		buf := audio.IntBuffer{
// 			Data: make([]int, d.PCMChunk.Size/2),
// 			Format: &audio.Format{
// 				NumChannels: 1,
// 				SampleRate:  44100,
// 			},
// 		}

// 		_, err = d.PCMBuffer(&buf)
// 		if err != nil {
// 			panic(err)
// 		}
// 		samples := buf.AsFloatBuffer().Data
// 		fingerPrints := fingerprint.Fingerprint(&samples, fileName)
// 		CreateHash(fingerPrints, db.DB)
// 		i++
// 		if i == 30 {
// 			break
// 		}
// 	}
// }

// func searchSong() {
// 	file, err := os.Open("temp.wav")
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer file.Close()
// 	samples, err := audio.DownSamplingAudio(file, "temp.wav")
// 	if err != nil {
// 		panic(err)
// 	}

// 	fingerPrints := fingerprint.Fingerprint(samples, "song")
// 	// Assuming fingerprint function takes []float64

// 	_data, _ := search.MatchHashes(fingerPrints, db.DB)
// 	fmt.Println("data", _data)

// }

// func FingerPrint() {
// 	files, err := os.ReadDir("assets")
// 	if err != nil {
// 		panic(err)
// 	}

// 	for _, file := range files {

// 		splitData := strings.Split(file.Name(), ".")
// 		var fileName string
// 		if len(splitData) > 2 {
// 			fileName = strings.Join(splitData[:len(splitData)-1], ".")

// 		} else {
// 			fileName = splitData[0]

// 		}
// 		fmt.Printf("Processing file: %s\n", fileName)
// 		openfileName := "assets/" + fileName + ".wav"
// 		file, err := os.Open(openfileName)
// 		if err != nil {
// 			panic(err)
// 		}
// 		defer file.Close()

// 		samples, err := audio.DownSamplingAudio(file, "song.wav")
// 		if err != nil {
// 			panic(err)
// 		}
// 		fingerPrints := fingerprint.Fingerprint(samples, fileName)
// 		CreateHash(fingerPrints, db.DB)

// 	}
// }
