package search

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"shazam/internal/audio"
	"shazam/internal/db"
	"shazam/internal/fingerprint"
	"sort"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MatchedSongOptimized struct {
	SongID string
	Score  float64
	Offset float64
}

const (
	MIN_MATCH_THRESHOLD = 5
	OFFSET_BIN_SIZE_MS  = 32
	TOP_N_RESULTS       = 3

	// New constants for secondary validation tolerances
	// FreqTolerance: Max allowed difference in Hz for AnchorFreq and TargetFreq
	// Example: Allowing up to 2 Hz difference.
	FREQ_TOLERANCE = 2.0 // Hz

	// TimeDeltaTolerance: Max allowed difference in seconds for TimeDelta
	// Example: Allowing up to 20 milliseconds (0.02 seconds) difference.
	TIME_DELTA_TOLERANCE = 0.02 // Seconds (equivalent to 20ms)
)

func MatchHashes(queryFingerprints []db.Fingerprint, DB *gorm.DB) ([]MatchedSongOptimized, error) {

	if len(queryFingerprints) == 0 {
		return nil, nil
	}

	sampleFingerprintMap := make(map[uint32]float64)
	hashes := make([]uint32, 0, len(queryFingerprints))
	for _, fp := range queryFingerprints {
		sampleFingerprintMap[fp.Hash] = fp.AnchorTime
		hashes = append(hashes, fp.Hash)
	}
	var matchedFingerprints []db.Fingerprint
	// Group by song_id, count hashes, and filter in SQL
	subQuery := DB.Model(&db.Fingerprint{}).
		Select("song_id").
		Where("hash IN ?", hashes).
		Group("song_id").
		Having("COUNT(*) > ?", float64(len(hashes))*0.2)

	if err := DB.Where("hash IN ? and song_id IN (?)", hashes, subQuery).
		Find(&matchedFingerprints).Error; err != nil {
		return nil, fmt.Errorf("failed DB query: %w", err)

	}
	fmt.Println("LENGTH OF MATCHED FINGERPRINTS", len(matchedFingerprints))
	matches := make(map[string][][2]uint32)
	timestamps := make(map[string]float64)
	targetZones := make(map[string]map[float64]int)

	for _, dbFp := range matchedFingerprints {
		sampleTime, ok := sampleFingerprintMap[dbFp.Hash]
		if !ok {
			continue
		}

		matches[dbFp.SongID] = append(matches[dbFp.SongID], [2]uint32{uint32(sampleTime), uint32(dbFp.AnchorTime)})

		if _, ok := timestamps[dbFp.SongID]; !ok || dbFp.AnchorTime < timestamps[dbFp.SongID] {
			timestamps[dbFp.SongID] = dbFp.AnchorTime
		}

		if _, ok := targetZones[dbFp.SongID]; !ok {
			targetZones[dbFp.SongID] = make(map[float64]int)
		}
		targetZones[dbFp.SongID][dbFp.AnchorTime]++
	}

	scores := analyzeRelativeTiming(matches)
	var result []MatchedSongOptimized

	for songID, score := range scores {

		match := MatchedSongOptimized{
			SongID: songID,
			Offset: timestamps[songID],
			Score:  score,
		}
		result = append(result, match)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})
	if len(result) > fingerprint.FAN_OUT {
		return result[:fingerprint.FAN_OUT], nil
	} else {

		return result, nil
	}
}

func analyzeRelativeTiming(matches map[string][][2]uint32) map[string]float64 {
	scores := make(map[string]float64)
	for songID, times := range matches {
		count := 0
		for i := 0; i < len(times); i++ {
			for j := i + 1; j < len(times); j++ {
				sampleDiff := math.Abs(float64(times[i][0] - times[j][0]))
				dbDiff := math.Abs(float64(times[i][1] - times[j][1]))
				if math.Abs(sampleDiff-dbDiff) < 50 {
					count++
				}

			}
		}
		scores[songID] = float64(count)
	}
	return scores
}

func RecogniseSong(c *gin.Context) {
	defer os.Remove("temp.wav")
	MultipartForm, err := c.MultipartForm()
	if err != nil {
		c.JSON(400, gin.H{"error": "Failed to read multipart form"})
		return
	}

	if MultipartForm == nil {
		c.JSON(400, gin.H{"error": "No audio file provided"})
		return
	}
	audios := MultipartForm.File["audio"]

	if len(audios) == 0 {
		c.JSON(400, gin.H{"error": "No audio file provided"})
		return
	}

	audioData := audios[0]
	fmt.Println(audioData.Filename)

	file1, err := audioData.Open()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return

	}
	defer file1.Close()
	temp, err := os.Create("temp.wav")
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer temp.Close()

	// Write the uploaded audio file to temp.wav
	_, err = file1.Seek(0, 0)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	_, err = temp.ReadFrom(file1)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return

	}
	file2, err := os.Open("temp.wav")
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer file2.Close()
	samples, err := audio.DownSamplingAudio(file2, "temp.wav")
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
	}
	fingerPrints := fingerprint.Fingerprint(samples, "song")
	_data, _ := MatchHashes(fingerPrints, db.DB)

	fmt.Println("SONG", _data)
	c.JSON(200, _data)
	runtime.GC()

}
