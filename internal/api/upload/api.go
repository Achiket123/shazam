package upload

import (
	"fmt"
	"log"
	"shazam/internal/audio"
	"shazam/internal/fingerprint"

	"github.com/gin-gonic/gin"
)

func InitUpload() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.POST("/upload", UploadSong)
	return r
}

func UploadSong(c *gin.Context) {
	type Songs struct {
		Songs []string `form:"songs" binding:"required"`
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	var songs Songs
	err = c.ShouldBind(&songs)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	for _, song := range songs.Songs {
		err = c.SaveUploadedFile(file, "../../../assets/"+song)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		audio.ConvertToWAV("../../../assets/"+song, "../../../assets/"+song+".wav")

		// 1. Load WAV
		samples, rate, err := audio.ReadWavFile("../../../assets/" + song + ".wav")
		if err != nil {
			log.Fatal("Error reading wav:", err)
		}
		fmt.Printf("Read WAV @ %d Hz, %d samples\n", rate, len(samples))

		// 2. Extract fingerprints
		fp := fingerprint.ExtractFingerprints(samples, 2048, 512, 6, 5)
		fmt.Printf("Generated %d fingerprints\n", len(fp))

		// Print a few
		for i := 0; i < 5 && i < len(fp); i++ {
			fmt.Printf("[%02d] Hash: %s | Offset: %d\n", i, fp[i].Hash, fp[i].TimeOffset)
		}
	}

	c.JSON(200, gin.H{"message": "success"})

}
