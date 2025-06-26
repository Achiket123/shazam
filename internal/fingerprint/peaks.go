package fingerprint

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"log"
	"math"
	"math/cmplx"
	"shazam/internal/db"
	"sort"
)

const (
	FAN_OUT                = 4
	PEAK_TARGET_DENSITY    = 30
	SECONDS_PER_CHUNK      = 1.0
	PEAK_NEIGHBORHOOD_SIZE = 5
)

func ExtractRobustPeaks(spectrogram [][]complex128, songID string) []Peak {
	if len(spectrogram) == 0 || len(spectrogram[0]) == 0 {
		return nil
	}

	numFrames := len(spectrogram)
	numBins := len(spectrogram[0])
	magnitudes := getMagnitudes(spectrogram)

	half := PEAK_NEIGHBORHOOD_SIZE / 2
	candidatePeaks := []Peak{}

	// Loop through each time-frequency point
	for t := half; t < numFrames-half; t++ {
		for f := half; f < numBins-half; f++ {
			currentAmp := magnitudes[t][f]
			if isLocalMax(magnitudes, t, f, half, currentAmp) {
				candidatePeaks = append(candidatePeaks, Peak{
					Time: float64(t),
					Freq: float64(f),
					Amp:  currentAmp,
				})
			}
		}
	}

	// Chunk-wise filtering
	finalPeaks := make([]Peak, 0)
	framesPerChunk := float64(SampleRate) * SECONDS_PER_CHUNK / float64(HopSize)
	peaksPerChunk := int(PEAK_TARGET_DENSITY * SECONDS_PER_CHUNK)

	for chunkStart := 0; chunkStart < len(candidatePeaks); {
		chunkEnd := chunkStart
		startTime := candidatePeaks[chunkStart].Time
		for chunkEnd < len(candidatePeaks) && candidatePeaks[chunkEnd].Time < startTime+framesPerChunk {
			chunkEnd++
		}

		chunk := candidatePeaks[chunkStart:chunkEnd]

		// Sort chunk by amplitude and pick top-N
		sort.Slice(chunk, func(i, j int) bool {
			return chunk[i].Amp > chunk[j].Amp
		})
		limit := min(len(chunk), peaksPerChunk)
		for _, peak := range chunk[:limit] {
			finalPeaks = append(finalPeaks, Peak{
				Time: float64(peak.Time*float64(HopSize)) / float64(SampleRate),
				Freq: float64(peak.Freq*float64(SampleRate)) / float64(WindowSize),
				Amp:  peak.Amp,
			})
		}

		chunkStart = chunkEnd
	}

	sort.Slice(finalPeaks, func(i, j int) bool {
		return finalPeaks[i].Time < finalPeaks[j].Time
	})

	log.Printf("Found %d robust peaks for song '%s'.\n", len(finalPeaks), songID)
	return finalPeaks
}

func isLocalMax(magnitudes [][]float64, t, f, half int, currentAmp float64) bool {
	for dt := -half; dt <= half; dt++ {
		for df := -half; df <= half; df++ {
			if dt == 0 && df == 0 {
				continue
			}
			if magnitudes[t+dt][f+df] >= currentAmp {
				return false
			}
		}
	}
	return true
}

func getMagnitudes(spectrogram [][]complex128) [][]float64 {
	numFrames := len(spectrogram)
	numBins := len(spectrogram[0])
	mags := make([][]float64, numFrames)
	for t := 0; t < numFrames; t++ {
		mags[t] = make([]float64, numBins)
		for f := 0; f < numBins; f++ {
			mags[t][f] = cmplx.Abs(spectrogram[t][f])
		}
	}
	return mags
}

func FindPeakRelationships(peaks []Peak, songID string) []db.Fingerprint {
	if len(peaks) == 0 {
		return nil
	}

	fingerprints := []db.Fingerprint{}

	// Use a bytes.Buffer to efficiently build the data to be hashed for each fingerprint.
	// This avoids repeated memory allocations for byte slices.
	buf := new(bytes.Buffer)

	for i, anchorPeak := range peaks {
		minTime := anchorPeak.Time + DeltaTMin
		maxTime := anchorPeak.Time + DeltaTMax

		pairCount := 0

		for j := i + 1; j < len(peaks); j++ {
			targetPeak := peaks[j]

			// Optimization: Peaks are assumed to be sorted by time.
			// If targetPeak.Time is already less than minTime, continue.
			if targetPeak.Time < minTime {
				continue
			}

			// If targetPeak.Time exceeds maxTime, no further peaks for this anchor will be valid.
			if targetPeak.Time > maxTime {
				break
			}

			// Apply FAN_OUT limit
			if pairCount >= FAN_OUT {
				break
			}

			deltaTime := targetPeak.Time - anchorPeak.Time

			buf.Reset()

			const freqScale = 100.0       // 0.01 Hz precision
			const timeDeltaScale = 1000.0 // 1 ms precision

			quantizedAnchorFreq := int32(math.Round(anchorPeak.Freq * freqScale))
			quantizedTargetFreq := int32(math.Round(targetPeak.Freq * freqScale))
			quantizedDeltaTime := int32(math.Round(deltaTime * timeDeltaScale))

			// Write the quantized integer values to the buffer.
			// Using BigEndian for consistent byte order.
			binary.Write(buf, binary.BigEndian, quantizedAnchorFreq)
			binary.Write(buf, binary.BigEndian, quantizedTargetFreq)
			binary.Write(buf, binary.BigEndian, quantizedDeltaTime)

			// 3. Compute the SHA-1 hash.
			hasher := sha1.New()
			hasher.Write(buf.Bytes())   // Feed the buffer's content to the hasher
			sha1Hash := hasher.Sum(nil) // Get the 20-byte hash sum

			// --- End SHA-1 Hashing Logic ---

			fingerprint := db.Fingerprint{
				AnchorTime: anchorPeak.Time,
				TargetFreq: targetPeak.Freq,
				AnchorFreq: anchorPeak.Freq,
				TimeDelta:  deltaTime,
				Hash:       hex.EncodeToString(sha1Hash), // Assign the computed SHA-1 hash (20 bytes)
				SongID:     songID,
			}
			fingerprints = append(fingerprints, fingerprint)
			pairCount++
		}
	}
	log.Printf("Created %d fingerprints for song '%s'.\n", len(fingerprints), songID)
	return fingerprints
}
