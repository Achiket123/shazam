package search

import (
	"fmt"
	"math"
	"os"
	"shazam/internal/audio"
	"shazam/internal/db"
	"shazam/internal/fingerprint"
	"sort"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// MatchedSong is the result returned to the client for each candidate match.
type MatchedSong struct {
	SongID     string  `json:"song_id"`
	Score      float64 `json:"score"`
	Offset     float64 `json:"offset_seconds"`
	Confidence string  `json:"confidence"` // "high" | "medium" | "low"
}

const (
	// Minimum number of hash matches for a candidate to be considered at all.
	MIN_HASH_MATCHES = 5

	// Offset histogram bin size in milliseconds.
	// Matches within the same bin are considered temporally coherent.
	// 20ms is tight enough to reject coincidental matches, loose enough to
	// tolerate minor timing jitter from recording latency or resampling.
	OFFSET_BIN_MS = 20

	// A candidate must have at least this fraction of matches in its best
	// offset bin to be returned. Filters out songs with scattered (random) matches.
	MIN_COHERENCE_RATIO = 0.05

	TOP_N_RESULTS = 5

	// Score thresholds for the confidence label
	HIGH_CONFIDENCE_SCORE   = 25
	MEDIUM_CONFIDENCE_SCORE = 10
)

// MatchHashes is the core recognition function.
//
// Algorithm:
//  1. Query the DB for all fingerprints whose hash appears in the query clip.
//  2. For each DB match, compute the time offset (dbAnchorTime - queryAnchorTime).
//  3. Bin those offsets into a histogram. If a song truly matches, all its
//     matching fingerprints will have nearly the same offset (the position of
//     the clip within the song). This creates a sharp histogram spike.
//  4. Score each song by the height of its tallest offset bin.
//  5. Apply coherence filtering and return the top N results with confidence labels.
func MatchHashes(queryFingerprints []db.Fingerprint, DB *gorm.DB) ([]MatchedSong, error) {
	if len(queryFingerprints) == 0 {
		return nil, nil
	}

	// Build hash → []anchorTime map (multimap — same hash can appear multiple times)
	queryMap := make(map[uint64][]float64, len(queryFingerprints))
	hashes := make([]uint64, 0, len(queryFingerprints))
	seen := make(map[uint64]bool)
	for _, fp := range queryFingerprints {
		queryMap[fp.Hash] = append(queryMap[fp.Hash], fp.AnchorTime)
		if !seen[fp.Hash] {
			hashes = append(hashes, fp.Hash)
			seen[fp.Hash] = true
		}
	}

	// Pre-filter: only fetch songs that have enough hash hits to be worth scoring.
	// The 10% threshold avoids loading thousands of rows for songs with 1-2 lucky collisions.
	subQuery := DB.Model(&db.Fingerprint{}).
		Select("song_id").
		Where("hash IN ?", hashes).
		Group("song_id").
		Having("COUNT(*) >= ?", math.Max(float64(MIN_HASH_MATCHES), float64(len(hashes))*0.05))

	var dbMatches []db.Fingerprint
	if err := DB.
		Where("hash IN ? AND song_id IN (?)", hashes, subQuery).
		Find(&dbMatches).Error; err != nil {
		return nil, fmt.Errorf("DB query failed: %w", err)
	}
	fmt.Printf("[search] db candidates: %d\n", len(dbMatches))

	// --- Offset histogram ---
	// key: (songID, offsetBin) → count of coherent matches
	type offsetKey struct {
		songID string
		bin    int64
	}
	histogram := make(map[offsetKey]int)
	totalMatches := make(map[string]int) // total raw matches per song

	for _, dbFp := range dbMatches {
		queryTimes, ok := queryMap[dbFp.Hash]
		if !ok {
			continue
		}
		totalMatches[dbFp.SongID]++
		for _, qt := range queryTimes {
			// Offset = where in the DB song does this query clip start?
			offsetMs := (dbFp.AnchorTime - qt) * 1000
			bin := int64(math.Floor(offsetMs / OFFSET_BIN_MS))
			histogram[offsetKey{dbFp.SongID, bin}]++
		}
	}

	// Find the best offset bin score per song
	bestBinScore := make(map[string]int)
	bestOffset := make(map[string]float64)
	for key, count := range histogram {
		if count > bestBinScore[key.songID] {
			bestBinScore[key.songID] = count
			bestOffset[key.songID] = float64(key.bin) * OFFSET_BIN_MS / 1000.0
		}
	}

	// Build result list, applying coherence filter
	var results []MatchedSong
	for songID, score := range bestBinScore {
		total := totalMatches[songID]
		coherenceRatio := float64(score) / float64(total)
		if coherenceRatio < MIN_COHERENCE_RATIO {
			continue // matches are scattered — likely false positive
		}

		results = append(results, MatchedSong{
			SongID:     songID,
			Score:      float64(score),
			Offset:     bestOffset[songID],
			Confidence: confidenceLabel(score),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > TOP_N_RESULTS {
		results = results[:TOP_N_RESULTS]
	}
	return results, nil
}

func confidenceLabel(score int) string {
	switch {
	case score >= HIGH_CONFIDENCE_SCORE:
		return "high"
	case score >= MEDIUM_CONFIDENCE_SCORE:
		return "medium"
	default:
		return "low"
	}
}

func RecogniseSong(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil || form == nil {
		c.JSON(400, gin.H{"error": "failed to read multipart form"})
		return
	}

	files := form.File["audio"]
	if len(files) == 0 {
		c.JSON(400, gin.H{"error": "no audio file provided (field: 'audio')"})
		return
	}

	audioData := files[0]
	fmt.Println("[search] received:", audioData.Filename)

	src, err := audioData.Open()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer src.Close()

	// Use a temp file with a unique name — avoids corruption under concurrent requests
	tmp, err := os.CreateTemp("", "shazam-query-*.wav")
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if _, err = tmp.ReadFrom(src); err != nil {
		c.JSON(500, gin.H{"error": "failed to buffer audio: " + err.Error()})
		return
	}

	// Reopen for reading (ReadFrom leaves write cursor at end of file)
	readFile, err := os.Open(tmp.Name())
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer readFile.Close()

	samples, err := audio.DownSamplingAudio(readFile, audioData.Filename)
	if err != nil {
		c.JSON(422, gin.H{"error": "audio processing failed: " + err.Error()})
		return
	}

	fps := fingerprint.Fingerprint(samples, "query")
	results, err := MatchHashes(fps, db.DB)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	fmt.Println("[search] results:", results)
	c.JSON(200, results)
}
