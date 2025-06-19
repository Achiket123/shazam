package fingerprint

import (
	"log"
	"math"
	"math/cmplx"
	"shazam/internal/db"
)

// GetPeaks returns the indices of local maxima (peaks) in the spectrogram.
// A peak is a point that is greater than its 8 neighbors.
func ExtractAdaptivePeaks(spectrogram [][]complex128, songID string) []Peak {
	if len(spectrogram) == 0 || len(spectrogram[0]) == 0 {
		return nil
	}

	numFrames := len(spectrogram)
	numBins := len(spectrogram[0])
	finalPeaks := make([]Peak, 0)

	// Step 1: Convert the entire spectrogram to dB magnitudes first.
	magnitudesDB := make([][]float64, numFrames)
	for t, frame := range spectrogram {
		magnitudesDB[t] = make([]float64, numBins)
		for f, c := range frame {
			mag := cmplx.Abs(c)
			if mag < 1e-9 { // Avoid log(0)
				magnitudesDB[t][f] = -180.0 // A very low dB value
			} else {
				magnitudesDB[t][f] = 20 * math.Log10(mag)
			}
		}
	}

	// Helper to convert Hz to a frequency bin index
	hzToBin := func(freq float64) int {
		return int(freq * float64(WindowSize) / float64(SampleRate))
	}

	// Step 2: Iterate through each time frame to find peaks adaptively.
	for t := 0; t < numFrames; t++ {

		// Find the single strongest peak within each frequency band for this time frame.
		bandPeaks := make([]Peak, 0, len(FREQ_BANDS))
		var sumOfAmps float64 = 0

		for _, band := range FREQ_BANDS {
			startBin := hzToBin(band[0])
			endBin := hzToBin(band[1])

			// Ensure bins are within the valid range
			if startBin < 0 {
				startBin = 0
			}
			if endBin >= numBins {
				endBin = numBins - 1
			}
			if startBin >= endBin {
				continue
			}

			maxAmp := -1e9
			maxBinIndex := -1

			for f := startBin; f <= endBin; f++ {
				if magnitudesDB[t][f] > maxAmp {
					maxAmp = magnitudesDB[t][f]
					maxBinIndex = f
				}
			}

			if maxBinIndex != -1 {
				peak := Peak{
					Amp: maxAmp,
					// We will fill in Time and Freq later if this peak is chosen
					// This saves a bit of computation.
					Time: float64(t),           // Store frame index for now
					Freq: float64(maxBinIndex), // Store bin index for now
				}
				bandPeaks = append(bandPeaks, peak)
				sumOfAmps += maxAmp
			}
		}

		if len(bandPeaks) == 0 {
			continue
		}

		// Step 3: Calculate the dynamic threshold for this time frame.
		dynamicThreshold := sumOfAmps / float64(len(bandPeaks))

		// Step 4: Filter the band peaks, keeping only those above the dynamic average.
		for _, peak := range bandPeaks {
			if peak.Amp > dynamicThreshold {
				// Now convert frame/bin indices to real-world units for the final peaks.
				finalPeak := Peak{
					Time: float64(peak.Time*float64(HopSize)) / float64(SampleRate),
					Freq: float64(peak.Freq*float64(SampleRate)) / float64(WindowSize),
					Amp:  peak.Amp,
				}
				finalPeaks = append(finalPeaks, finalPeak)
			}
		}
	}

	log.Printf("Found %d adaptive peaks for song '%s'.\n", len(finalPeaks), songID)
	return finalPeaks
}

func FindPeakRelationships(peaks []Peak, songID string) []db.Fingerprint {
	if len(peaks) == 0 {
		return nil
	}

	fingerprints := []db.Fingerprint{}

	// Sort peaks by time to ensure chronological processing
	// (Although ExtractAdaptivePeaks already produces them in order, this is good practice)
	// sort.Slice(peaks, func(i, j int) bool { return peaks[i].Time < peaks[j].Time })

	for i, anchorPeak := range peaks {
		// Define the time window for the target zone
		minTime := anchorPeak.Time + DeltaTMin
		maxTime := anchorPeak.Time + DeltaTMax

		// Iterate through subsequent peaks to find potential targets
		for j := i + 1; j < len(peaks); j++ {
			targetPeak := peaks[j]

			// If the target peak is too early, skip it
			if targetPeak.Time < minTime {
				continue
			}

			// If the target peak is too late, we can stop searching for this anchor
			if targetPeak.Time > maxTime {
				break
			}

			// Check if the frequency difference is within the allowed range
			deltaFreq := math.Abs(targetPeak.Freq - anchorPeak.Freq)
			if deltaFreq <= DeltaFMax {
				// This is a valid pair! Create a fingerprint.
				deltaTime := targetPeak.Time - anchorPeak.Time

				// A simple and effective hash.
				// NOTE: For a production system, you might want a more robust hash
				// that is less prone to collisions.
				hash := int64(anchorPeak.Freq)&0xFFF<<20 | int64(targetPeak.Freq)&0xFFF<<8 | int64(deltaTime*100)&0xFF

				fingerprint := db.Fingerprint{
					AnchorFreq: anchorPeak.Freq,
					TargetFreq: targetPeak.Freq,
					TimeDelta:  deltaTime,
					AnchorTime: anchorPeak.Time,
					Hash:       hash, // You would use this hash as a key in your database
					SongID:     songID,
				}
				fingerprints = append(fingerprints, fingerprint)
			}
		}
	}
	log.Printf("Created %d fingerprints for song '%s'.\n", len(fingerprints), songID)
	return fingerprints
}
