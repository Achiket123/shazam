package search

import (
	"fmt"
	"shazam/internal/db"
	"sort"

	"gorm.io/gorm"
)

type MatchedSong struct {
	SongID     string
	Score      int // Number of aligned fingerprint matches
	TimeOffset int // The time offset (anchorPeak.Time - matchedDbFingerprint.Time)
}

const MIN_MATCH_THRESHOLD = 3

// func MatchAPI(c *gin.Context) {
// 	song, err := c.FormFile("song")
// 	if err != nil {
// 		panic(err)
// 	}
// 	file, err := song.Open()
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer file.Close()
// 	_, err = audio.DownSamplingAudio(file, song.Filename)
// 	if err != nil {
// 		panic(err)
// 	}
// 	outPut, err := os.Open("output.wav")
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer outPut.Close()
// 	fingerprint.ComputeFFT(outPut, song.Filename)
// 	hashes := fingerprint.ComputeFFT(outPut, song.Filename)
// 	_, name := MatchHashes(hashes, db.DB)
// 	c.JSON(200, gin.H{
// 		"name": name,
// 	})

// }

func MatchHashes(queryFingerprints []db.Fingerprint, DB *gorm.DB) []MatchedSong {
	if len(queryFingerprints) == 0 {
		return nil
	}

	// === Step 1: Prepare query data for efficient lookup ===
	// Map hashes to their anchor times in the query sample. Times are in milliseconds.
	queryHashMap := make(map[int64][]int)
	for _, qfp := range queryFingerprints {
		// Convert anchor time (seconds) to milliseconds for integer-based offsets
		anchorTimeMs := int(qfp.AnchorTime * 1000)
		queryHashMap[qfp.Hash] = append(queryHashMap[qfp.Hash], anchorTimeMs)
	}

	uniqueQueryHashes := make([]int64, 0, len(queryHashMap))
	for hashVal := range queryHashMap {
		uniqueQueryHashes = append(uniqueQueryHashes, hashVal)
	}
	var dbMatches []db.Fingerprint
	// Assuming your GORM model for Fingerprint has a field `Hash` that maps to the "hash" column.
	result := DB.Where("hash IN ?", uniqueQueryHashes).Find(&dbMatches)
	if result.Error != nil {
		fmt.Printf("Database query error: %v\n", result.Error)
		return nil
	}

	// === Step 3: Build the Time Offset Histogram ===
	// This map stores: songID -> offset -> count
	songTimeOffsets := make(map[string]map[int]int)

	for _, dbMatch := range dbMatches {
		// Check if the hash from the DB match exists in our query hashes
		if queryAnchorTimes, ok := queryHashMap[dbMatch.Hash]; ok {
			// A single DB hash could match multiple times in the query if the audio pattern repeats
			for _, queryAnchorTime := range queryAnchorTimes {
				// Calculate the time offset. This is the core of the algorithm.
				// We also convert the DB anchor time to milliseconds.
				offset := queryAnchorTime - int(dbMatch.AnchorTime*1000)

				// Initialize map for the song if it's the first time we see it
				if _, exists := songTimeOffsets[dbMatch.SongID]; !exists {
					songTimeOffsets[dbMatch.SongID] = make(map[int]int)
				}
				// Increment the count for this specific offset for this song.
				songTimeOffsets[dbMatch.SongID][offset]++
			}
		}
	}

	// === Step 4: Find the Best Score for Each Song from the Histogram ===
	var matchedSongs []MatchedSong

	for songID, offsetsMap := range songTimeOffsets {
		bestScore := 0
		bestOffset := 0

		// Find the offset with the highest number of matches (the peak of the histogram)
		for offset, count := range offsetsMap {
			if count > bestScore {
				bestScore = count
				bestOffset = offset
			}
		}

		// Only consider songs that meet a minimum threshold of matches.
		// This filters out songs with just a few random, coincidental hash matches.
		if bestScore >= MIN_MATCH_THRESHOLD {
			matchedSongs = append(matchedSongs, MatchedSong{
				SongID:     songID,
				Score:      bestScore,
				TimeOffset: bestOffset,
			})
		}
	}

	// === Step 5: Sort Results and Return the Best Matches ===
	sort.Slice(matchedSongs, func(i, j int) bool {
		return matchedSongs[i].Score > matchedSongs[j].Score
	})

	// You can return all matches or just the top N
	const topN = 5
	if len(matchedSongs) > topN {
		return matchedSongs[:topN]
	}
	return matchedSongs
}
