package fingerprint

import (
	"log"
	"math/cmplx"
	"shazam/internal/db"
	"sort"
)

const FAN_OUT = 4
const PEAK_TARGET_DENSITY = 30
const SECONDS_PER_CHUNK = 1.0

func ExtractRobustPeaks(spectrogram [][]complex128, songID string) []Peak {
	if len(spectrogram) == 0 || len(spectrogram[0]) == 0 {
		return nil
	}

	numFrames := len(spectrogram)
	numBins := len(spectrogram[0])
	magnitudes := getMagnitudes(spectrogram)

	candidatePeaks := []Peak{}
	for t := 1; t < numFrames-1; t++ {
		for f := 1; f < numBins-1; f++ {
			amp := magnitudes[t][f]

			if amp > magnitudes[t-1][f-1] && amp > magnitudes[t-1][f] && amp > magnitudes[t-1][f+1] &&
				amp > magnitudes[t][f-1] && amp > magnitudes[t][f+1] &&
				amp > magnitudes[t+1][f-1] && amp > magnitudes[t+1][f] && amp > magnitudes[t+1][f+1] {

				candidatePeaks = append(candidatePeaks, Peak{
					Time: float64(t),
					Freq: float64(f),
					Amp:  amp,
				})
			}
		}
	}

	finalPeaks := make([]Peak, 0)
	framesPerChunk := float64(SECONDS_PER_CHUNK) * float64(SampleRate) / float64(HopSize)
	peaksPerChunk := int(PEAK_TARGET_DENSITY * SECONDS_PER_CHUNK)

	for chunkStart := 0; chunkStart < len(candidatePeaks); {
		chunkEnd := chunkStart

		startTime := candidatePeaks[chunkStart].Time
		for chunkEnd < len(candidatePeaks) && (candidatePeaks[chunkEnd].Time < startTime+float64(framesPerChunk)) {
			chunkEnd++
		}

		chunk := candidatePeaks[chunkStart:chunkEnd]

		if len(chunk) > 0 {

			sort.Slice(chunk, func(i, j int) bool {
				return chunk[i].Amp > chunk[j].Amp
			})

			limit := peaksPerChunk
			if len(chunk) < limit {
				limit = len(chunk)
			}

			for _, peak := range chunk[:limit] {

				finalPeaks = append(finalPeaks, Peak{
					Time: float64(peak.Time*float64(HopSize)) / float64(SampleRate),
					Freq: float64(peak.Freq*float64(SampleRate)) / float64(WindowSize),
					Amp:  peak.Amp,
				})
			}
		}

		chunkStart = chunkEnd
	}

	sort.Slice(finalPeaks, func(i, j int) bool {
		return finalPeaks[i].Time < finalPeaks[j].Time
	})

	log.Printf("Found %d robust peaks for song '%s'.\n", len(finalPeaks), songID)
	return finalPeaks
}

func getMagnitudes(spectrogram [][]complex128) [][]float64 {
	numFrames := len(spectrogram)
	numBins := len(spectrogram[0])
	magnitudes := make([][]float64, numFrames)
	for t := 0; t < numFrames; t++ {
		magnitudes[t] = make([]float64, numBins)
		for f := 0; f < numBins; f++ {
			magnitudes[t][f] = cmplx.Abs(spectrogram[t][f])
		}
	}
	return magnitudes
}

func FindPeakRelationships(peaks []Peak, songID string) []db.Fingerprint {
	if len(peaks) == 0 {
		return nil
	}

	fingerprints := []db.Fingerprint{}

	for i, anchorPeak := range peaks {

		minTime := anchorPeak.Time + DeltaTMin
		maxTime := anchorPeak.Time + DeltaTMax

		pairCount := 0

		for j := i + 1; j < len(peaks); j++ {
			targetPeak := peaks[j]

			if targetPeak.Time < minTime {
				continue
			}

			if targetPeak.Time > maxTime {
				break
			}

			if pairCount >= FAN_OUT {
				break
			}

			deltaTime := targetPeak.Time - anchorPeak.Time
			hash := int64(anchorPeak.Freq)&0xFFF<<20 | int64(targetPeak.Freq)&0xFFF<<8 | int64(deltaTime*100)&0xFF

			fingerprint := db.Fingerprint{

				Hash:   hash,
				SongID: songID,
			}
			fingerprints = append(fingerprints, fingerprint)
			pairCount++
		}
	}
	log.Printf("Created %d fingerprints for song '%s'.\n", len(fingerprints), songID)
	return fingerprints
}
