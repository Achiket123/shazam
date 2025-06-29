package upload

import (
	"shazam/internal/audio"
	"shazam/internal/db"
	"shazam/internal/fingerprint"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func FingerprintAPI(c *gin.Context) {
	song, err := c.FormFile("song")
	if err != nil {
		panic(err)
	}

	songFile, err := song.Open()
	if err != nil {
		panic(err)
	}
	defer songFile.Close()
	samples, err := audio.DownSamplingAudio(songFile, "songs")
	if err != nil {
		panic(err)
	}
	hashes := fingerprint.Fingerprint(samples, song.Filename)
	CreateHash(hashes, db.DB)
}

func CreateHash(hashes []db.Fingerprint, DB *gorm.DB) {
	if err := DB.CreateInBatches(&hashes, 4000).Error; err != nil {
		panic(err)
	}
}
