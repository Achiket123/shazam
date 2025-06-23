package search

import (
	"fmt"
	"shazam/internal/db"
	"sort"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MatchedSongOptimized struct {
	SongID     string
	Score      int
	Confidence float64
	TimeOffset int
}

const (
	MIN_MATCH_THRESHOLD = 5

	OFFSET_BIN_SIZE_MS = 32

	TOP_N_RESULTS = 3
)

func MatchHashes(queryFingerprints []db.Fingerprint, DB *gorm.DB) []MatchedSongOptimized {
	if len(queryFingerprints) == 0 {
		return nil
	}

	queryHashMap := make(map[int64][]int)
	for _, qfp := range queryFingerprints {
		anchorTimeMs := int(qfp.AnchorTime * 1000)
		queryHashMap[qfp.Hash] = append(queryHashMap[qfp.Hash], anchorTimeMs)
	}

	uniqueQueryHashes := make([]int64, 0, len(queryHashMap))
	for hashVal := range queryHashMap {
		uniqueQueryHashes = append(uniqueQueryHashes, hashVal)
	}

	var dbMatches []db.Fingerprint
	result := DB.Where("hash IN ?", uniqueQueryHashes).Find(&dbMatches)
	if result.Error != nil {
		fmt.Printf("Database query error: %v\n", result.Error)
		return nil
	}

	histogram := make(map[string]map[int]int)

	for _, dbMatch := range dbMatches {
		if queryAnchorTimes, ok := queryHashMap[dbMatch.Hash]; ok {
			dbAnchorTimeMs := int(dbMatch.AnchorTime * 1000)
			for _, queryAnchorTime := range queryAnchorTimes {

				offset := queryAnchorTime - dbAnchorTimeMs
				binnedOffset := (offset / OFFSET_BIN_SIZE_MS) * OFFSET_BIN_SIZE_MS

				if _, exists := histogram[dbMatch.SongID]; !exists {
					histogram[dbMatch.SongID] = make(map[int]int)
				}
				histogram[dbMatch.SongID][binnedOffset]++
			}
		}
	}

	var potentialMatches []MatchedSongOptimized

	for songID, offsetsMap := range histogram {
		bestScore := 0
		secondBestScore := 0
		bestOffset := 0

		for offset, count := range offsetsMap {
			if count > bestScore {
				secondBestScore = bestScore
				bestScore = count
				bestOffset = offset
			} else if count > secondBestScore {
				secondBestScore = count
			}
		}

		if bestScore >= MIN_MATCH_THRESHOLD {

			var confidence float64
			if secondBestScore > 0 {
				confidence = float64(bestScore) / float64(secondBestScore)
			} else {
				confidence = float64(bestScore)
			}

			potentialMatches = append(potentialMatches, MatchedSongOptimized{
				SongID:     songID,
				Score:      bestScore,
				Confidence: confidence,
				TimeOffset: bestOffset,
			})
		}
	}

	sort.Slice(potentialMatches, func(i, j int) bool {

		if potentialMatches[i].Score != potentialMatches[j].Score {
			return potentialMatches[i].Score > potentialMatches[j].Score
		}
		return potentialMatches[i].Confidence > potentialMatches[j].Confidence
	})

	if len(potentialMatches) > TOP_N_RESULTS {
		return potentialMatches[:TOP_N_RESULTS]
	}
	return potentialMatches
}

func RecogniseSong(c *gin.Context) {
	var requestHashes []db.Fingerprint
	if err := c.ShouldBindJSON(&requestHashes); err != nil {
		c.JSON(500, gin.H{"error": "Invalid request format"})
		return
	}
	hashes := MatchHashes(requestHashes, db.DB)
	if len(hashes) == 0 {
		c.JSON(200, gin.H{"message": "No matches found"})
	} else {
		c.JSON(200, hashes)
	}
}
