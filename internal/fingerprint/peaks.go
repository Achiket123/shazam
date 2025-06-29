package fingerprint

import (
	"math"
	"math/cmplx"
	"shazam/internal/db"
	"sort"
)

const (
	FAN_OUT = 5

	TARGET_ZONE_DT_MAX_SECONDS = 2.0
	TARGET_ZONE_FREQ_BAND_HZ   = 300.0

	// Ensure SampleRate, WindowSize, HopSize are defined and accessible.
	// Example values:

	// FREQ_BIN_RESOLUTION should be consistent with SampleRate and WindowSize
	FREQ_BIN_RESOLUTION = float64(SampleRate) / float64(WindowSize)
)

type Peak struct {
	Time      float64
	Frequency float64
	Magnitude float64
}

// Assumed helper functions (from your previous code)
func meanStd(data []float64) (float64, float64) {
	if len(data) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	mean := sum / float64(len(data))

	sumSqDiff := 0.0
	for _, v := range data {
		sumSqDiff += (v - mean) * (v - mean)
	}
	std := math.Sqrt(sumSqDiff / float64(len(data)))
	return mean, std
}

func getMagnitudes(spectrogram [][]complex128) [][]float64 {
	numFrames := len(spectrogram)
	if numFrames == 0 {
		return [][]float64{}
	}
	numBins := len(spectrogram[0])
	magnitudes := make([][]float64, numFrames)
	for t := 0; t < numFrames; t++ {
		magnitudes[t] = make([]float64, numBins)
		for f := 0; f < numBins; f++ {
			mags := cmplx.Abs(spectrogram[t][f]) // Use cmplx.Abs if available, or .Abs() if complex128 has it
			if mags == 0 {
				magnitudes[t][f] = -120.0 // dB floor for silence
			} else {
				magnitudes[t][f] = 20 * math.Log10(mags)
			}
		}
	}
	return magnitudes
}

func calculateFrequencyFromBin(binIndex int) float64 {
	return float64(binIndex) * (float64(SampleRate) / float64(WindowSize))
}

// ExtractPeaks identifies significant peaks from the spectrogram.
// It has been adjusted to potentially yield more fingerprints.
func ExtractPeaks(spectrogram [][]complex128) []Peak {
	if len(spectrogram) < 1 || len(spectrogram[0]) < 1 {
		return []Peak{}
	}

	magnitudes := getMagnitudes(spectrogram)
	numFrames := len(magnitudes)
	numBins := len(magnitudes[0])
	peaks := []Peak{}

	// Neighborhood size for local maximum check (5x5 grid)
	neighborhoodSizeTime := 2 // +/- 2 frames
	neighborhoodSizeFreq := 2 // +/- 2 frequency bins

	// Adjusted Z-score threshold to be less strict
	// Lowering this value will increase the number of peaks detected.
	zScoreThreshold := 1.8 // Changed from 3.0 to 1.8 (experiment with this value)

	// Adjusted max peaks per frame to allow more peaks
	// Increasing this value will increase the number of fingerprints.
	maxPeaksPerFrame := 20 // Changed from 10 to 20 (experiment with this value)

	// Z-score comparison neighborhood size (21x21 window for background contrast)
	// Consider if this fixed size is appropriate for all frequency ranges.
	zScoreComparisonWindow := 10 // +/- 10 frames/bins

	for i := 0; i < numFrames; i++ {
		currentFramePeaks := []Peak{}
		for j := 0; j < numBins; j++ {
			currentMagnitude := magnitudes[i][j]

			// Skip very low magnitude points, as they are likely noise or silence
			if currentMagnitude <= -100.0 { // -100 dB is a common low-energy threshold
				continue
			}

			// --- Local Maximum Check ---
			isLocalMaximum := true
			for di := -neighborhoodSizeTime; di <= neighborhoodSizeTime; di++ {
				for dj := -neighborhoodSizeFreq; dj <= neighborhoodSizeFreq; dj++ {
					ni, nj := i+di, j+dj

					// Ensure neighborhood indices are within bounds and not the current point itself
					if ni >= 0 && ni < numFrames && nj >= 0 && nj < numBins && (di != 0 || dj != 0) {
						if magnitudes[ni][nj] > currentMagnitude {
							isLocalMaximum = false
							break
						}
					}
				}
				if !isLocalMaximum {
					break
				}
			}

			if isLocalMaximum {
				// --- Z-score Calculation for Contrastive Peaks ---
				zScoreNeighborhood := []float64{}
				// Collect magnitudes from the larger comparison window
				for di := -zScoreComparisonWindow; di <= zScoreComparisonWindow; di++ {
					for dj := -zScoreComparisonWindow; dj <= zScoreComparisonWindow; dj++ {
						ni, nj := i+di, j+dj
						if ni >= 0 && ni < numFrames && nj >= 0 && nj < numBins {
							// Optionally, exclude the current peak itself from the Z-score mean/std calculation
							// if (di != 0 || dj != 0) { ... }
							zScoreNeighborhood = append(zScoreNeighborhood, magnitudes[ni][nj])
						}
					}
				}

				mean, std := meanStd(zScoreNeighborhood)
				if std == 0 { // Avoid division by zero if all values in neighborhood are the same
					continue
				}
				zScore := (currentMagnitude - mean) / std

				// If the peak's Z-score meets the threshold, add it
				if zScore >= zScoreThreshold {
					peakTime := float64(i*HopSize) / float64(SampleRate)
					peakFrequency := calculateFrequencyFromBin(j)
					currentFramePeaks = append(currentFramePeaks, Peak{
						Time:      peakTime,
						Frequency: peakFrequency,
						Magnitude: currentMagnitude,
					})
				}
			}
		}

		// --- Limit and Sort Peaks Per Frame ---
		// If more peaks are found than maxPeaksPerFrame, sort by magnitude and take the top N
		if len(currentFramePeaks) > maxPeaksPerFrame {
			// Sort peaks in the current frame by Magnitude in descending order
			sort.Slice(currentFramePeaks, func(a, b int) bool {
				return currentFramePeaks[a].Magnitude > currentFramePeaks[b].Magnitude
			})
			// Append only the top 'maxPeaksPerFrame' peaks
			peaks = append(peaks, currentFramePeaks[:maxPeaksPerFrame]...)
		} else {
			// Append all found peaks if they are within the limit
			peaks = append(peaks, currentFramePeaks...)
		}
	}

	return peaks
}

func FindPeakRelationships(peaks []Peak, songID string) []db.Fingerprint {
	if len(peaks) == 0 {
		return nil
	}

	fingerprints := []db.Fingerprint{}

	for i, anchor := range peaks {

		targetZoneMinTime := anchor.Time
		targetZoneMaxTime := anchor.Time + TARGET_ZONE_DT_MAX_SECONDS
		targetZoneMinFreq := anchor.Frequency - TARGET_ZONE_FREQ_BAND_HZ
		targetZoneMaxFreq := anchor.Frequency + TARGET_ZONE_FREQ_BAND_HZ

		potentialTargets := []Peak{}
		for j := i + 1; j < len(peaks); j++ {
			target := peaks[j]

			if target.Time >= targetZoneMinTime && target.Time <= targetZoneMaxTime &&
				target.Frequency >= targetZoneMinFreq && target.Frequency <= targetZoneMaxFreq {
				potentialTargets = append(potentialTargets, target)
			}

			if target.Time > targetZoneMaxTime {
				break
			}
		}

		numTargetsToConsider := int(math.Min(float64(len(potentialTargets)), float64(FAN_OUT)))

		for k := 0; k < numTargetsToConsider; k++ {
			target := potentialTargets[k]

			deltaT := target.Time - anchor.Time

			if deltaT <= 0 {
				continue
			}

			anchorFreqBin := int(anchor.Frequency / FREQ_BIN_RESOLUTION)
			targetFreqBin := int(target.Frequency / FREQ_BIN_RESOLUTION)

			deltaMs := uint32(math.Round(deltaT * 1000))

			const (
				FREQ_BITS    = 9
				DELTA_T_BITS = 14
			)

			maskedAnchorFreq := uint32(anchorFreqBin) & ((1 << FREQ_BITS) - 1)
			maskedTargetFreq := uint32(targetFreqBin) & ((1 << FREQ_BITS) - 1)
			maskedDeltaMs := deltaMs & ((1 << DELTA_T_BITS) - 1)

			address := (maskedAnchorFreq << (FREQ_BITS + DELTA_T_BITS)) |
				(maskedTargetFreq << DELTA_T_BITS) |
				maskedDeltaMs

			fingerprint := db.Fingerprint{
				SongID:     songID,
				Hash:       address,
				AnchorTime: anchor.Time,
			}
			fingerprints = append(fingerprints, fingerprint)
		}
	}
	return fingerprints
}
