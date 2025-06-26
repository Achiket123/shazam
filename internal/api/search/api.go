package search

import (
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"shazam/internal/db"
	"shazam/internal/fingerprint"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/go-audio/wav"
	"gorm.io/gorm"
)

// Constants (ensure these are defined as they were in your original code)
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

// MatchedSongOptimized represents a potential song match with its score, confidence, and time offset.
type MatchedSongOptimized struct {
	SongID      string
	Score       int // Number of hash matches that align at a common time offset
	MatchCount  int // Score / SecondBestScore (or Score if no second best)
	MatchOffset int // The most common time offset in milliseconds for the song
}

func MatchHashes(queryFingerprints []db.Fingerprint, DB *gorm.DB) ([]MatchedSongOptimized, error) {
	queryLength := len(queryFingerprints)
	thresholdForQuery := (queryLength)
	if queryLength == 0 {
		return nil, nil
	}

	const freqThreshold = 20.0
	const timeDeltaThreshold = 20.0
	const timeDeltaWeight = 0.3
	const countWeight = 0.7

	histogram := make(map[string]map[int]int)
	timedeltaHistogram := make(map[string]map[int]int)
	queryHashMap := make(map[string][]db.Fingerprint)
	sliceOfHash := make([]string, 0, len(queryFingerprints))

	for _, qfp := range queryFingerprints {
		hashHex := qfp.Hash
		sliceOfHash = append(sliceOfHash, hashHex)
		queryHashMap[hashHex] = append(queryHashMap[hashHex], qfp)
	}

	allFingerPrints := []db.Fingerprint{}

	type SongCount struct {
		SongID string
		Count  int
	}

	var qualifiedSongs []SongCount
	err := DB.
		Table("fingerprints").
		Select("song_id, COUNT(*) as count").
		Where("hash IN ?", sliceOfHash).
		Group("song_id").
		Having("COUNT(*) >= ?", thresholdForQuery).
		Scan(&qualifiedSongs).Error
	if err != nil {
		return nil, err
	}
	fmt.Println("Qualified songs: ", len(qualifiedSongs))

	songIDs := make([]string, 0, len(qualifiedSongs))
	for _, entry := range qualifiedSongs {
		songIDs = append(songIDs, entry.SongID)
	}

	if len(songIDs) == 0 {
		return []MatchedSongOptimized{}, nil
	}

	results := DB.
		Where("hash IN ?", sliceOfHash).
		Find(&allFingerPrints)
	if results.Error != nil {
		return nil, results.Error
	}

	for _, afp := range allFingerPrints {
		qfps := queryHashMap[afp.Hash]
		for _, qfp := range qfps {
			freqDiffQuery := math.Abs(qfp.AnchorFreq - qfp.TargetFreq)
			freqDiffDB := math.Abs(afp.AnchorFreq - afp.TargetFreq)
			if math.Abs(freqDiffQuery-freqDiffDB) <= freqThreshold {
				if math.Abs(afp.TimeDelta-qfp.TimeDelta) <= timeDeltaThreshold {
					offset := int(afp.AnchorTime - qfp.AnchorTime)
					timedelta := int(afp.TimeDelta - qfp.TimeDelta)
					if _, ok := histogram[afp.SongID]; !ok {
						histogram[afp.SongID] = make(map[int]int)
					}
					histogram[afp.SongID][offset]++

					if _, ok := timedeltaHistogram[afp.SongID]; !ok {
						timedeltaHistogram[afp.SongID] = make(map[int]int)
					}
					timedeltaHistogram[afp.SongID][timedelta]++
				}
			}
		}
	}

	finalMatches := []MatchedSongOptimized{}
	for songID, offsetMap := range histogram {
		bestOffset := 0
		maxCount := 0
		for offset, count := range offsetMap {
			if count > maxCount {
				maxCount = count
				bestOffset = offset
			}
		}

		bestTimedelta := 0
		maxTDCount := 0
		if tdMap, exists := timedeltaHistogram[songID]; exists {
			for td, count := range tdMap {
				if count > maxTDCount {
					maxTDCount = count
					bestTimedelta = td
				}
			}
		}
		fmt.Println(bestTimedelta)
		score := int(float64(maxCount)*countWeight + float64(maxTDCount)*timeDeltaWeight)
		match := MatchedSongOptimized{
			SongID:      songID,
			MatchOffset: bestOffset,
			MatchCount:  maxCount,
			Score:       score,
		}
		finalMatches = append(finalMatches, match)
	}

	sort.Slice(finalMatches, func(i, j int) bool {
		return finalMatches[i].Score > finalMatches[j].Score
	})

	return finalMatches, nil
}

func RecogniseSong(c *gin.Context) {

	fileHeader, err := c.FormFile("audio")
	if err != nil {
		c.JSON(400, gin.H{"error": "Could not get file from form: " + err.Error()})
		return
	}

	uploadedFile, err := fileHeader.Open()
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to open uploaded file"})
		return
	}
	defer uploadedFile.Close()

	tempM4AFile, err := os.CreateTemp("", "upload-*.m4a")
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to create temp m4a file"})
		return
	}
	defer os.Remove(tempM4AFile.Name())
	defer tempM4AFile.Close()

	_, err = io.Copy(tempM4AFile, uploadedFile)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to save uploaded file"})
		return
	}
	tempM4AFile.Close()

	tempWAVFilePath := tempM4AFile.Name() + ".wav"
	cmd := exec.Command("ffmpeg", "-i", tempM4AFile.Name(), tempWAVFilePath)
	if err := cmd.Run(); err != nil {
		c.JSON(500, gin.H{"error": "Failed to convert audio to WAV: " + err.Error()})
		return
	}
	defer os.Remove(tempWAVFilePath)

	wavFile, err := os.Open(tempWAVFilePath)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to open converted WAV file"})
		return
	}
	defer wavFile.Close()

	d := wav.NewDecoder(wavFile)
	buf, err := d.FullPCMBuffer()
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to read PCM buffer from WAV: " + err.Error()})
		return
	}

	samples := buf.AsFloatBuffer().Data

	fingerPrints := fingerprint.Fingerprint(&samples, "song")

	hashes, _ := MatchHashes(fingerPrints, db.DB)
	if len(hashes) == 0 {
		c.JSON(200, gin.H{"message": "No matches found"})
	} else {
		c.JSON(200, hashes)
	}
}
