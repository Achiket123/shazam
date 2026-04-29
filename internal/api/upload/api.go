package upload

import (
	"fmt"
	"net/http"
	"shazam/internal/audio"
	"shazam/internal/db"
	"shazam/internal/fingerprint"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func FingerprintAPI(c *gin.Context) {
	song, err := c.FormFile("song")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing 'song' field: " + err.Error()})
		return
	}

	songFile, err := song.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer songFile.Close()

	samples, err := audio.DownSamplingAudio(songFile, song.Filename)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "audio processing failed: " + err.Error()})
		return
	}

	hashes := fingerprint.Fingerprint(samples, song.Filename)
	if err := createHash(hashes, db.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store fingerprints: " + err.Error()})
		return
	}

	fmt.Printf("[upload] stored %d fingerprints for %s\n", len(hashes), song.Filename)
	c.JSON(http.StatusOK, gin.H{
		"song":         song.Filename,
		"fingerprints": len(hashes),
	})
}

func createHash(hashes []db.Fingerprint, DB *gorm.DB) error {
	if len(hashes) == 0 {
		return nil
	}
	return DB.CreateInBatches(&hashes, 4000).Error
}
