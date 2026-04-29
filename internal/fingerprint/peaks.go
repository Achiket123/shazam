package fingerprint

import (
	"math"
	"shazam/internal/db"
	"sort"
)

const (
	FAN_OUT = 15 // more pairings per anchor = more fingerprints = better recall under noise

	// Target zone for pairing: anchor peak searches forward in time within this window
	TARGET_ZONE_DT_MAX_SECONDS = 2.0

	// How many top peaks to keep per band per frame
	MAX_PEAKS_PER_BAND = 3

	// Z-score threshold — peaks must stand out from their local neighbourhood by this much.
	// Per-band extraction means we can be more selective (higher threshold) without
	// suppressing musically important peaks in quieter bands.
	Z_SCORE_THRESHOLD = 2.5

	// Neighbourhood radius (frames × bins) used when computing the local mean/std
	// for Z-score. Kept small so computation stays manageable.
	Z_NEIGHBORHOOD = 7

	// Minimum time gap between anchor and target peaks.
	// Pairs that are too close in time tend to come from the same transient and
	// add noise without improving discriminability.
	MIN_DELTA_T = 0.05 // seconds
)

// Peak represents a spectral peak: a point in time-frequency space with high local energy.
type Peak struct {
	Time      float64
	Frequency float64
	Magnitude float64
}

// ExtractPeaks finds perceptually significant peaks across all frequency bands.
//
// Key improvements over a naive global approach:
//  1. The spectrogram is split into log-spaced frequency bands (see FreqBands in fingerprinting.go).
//     This prevents loud low-frequency content (bass, kick drum) from suppressing peaks in
//     quieter but harmonically important high-frequency bands.
//  2. Local maximum detection uses a 3×3 neighbourhood (time × freq) inside each band.
//  3. Z-score thresholding is computed within the band, not globally, so the threshold
//     adapts to the local noise floor of each frequency region.
//  4. Only the top MAX_PEAKS_PER_BAND peaks (by magnitude) are kept per band per frame,
//     bounding the total fingerprint count while maximising quality.
func ExtractPeaks(spectrogram [][]complex128) []Peak {
	if len(spectrogram) == 0 || len(spectrogram[0]) == 0 {
		return nil
	}

	magnitudes := getMagnitudesDB(spectrogram)
	numFrames := len(magnitudes)
	numBins := len(magnitudes[0])

	var allPeaks []Peak

	for _, band := range FreqBands {
		minBin := freqToBin(band.MinHz)
		maxBin := freqToBin(band.MaxHz)
		if maxBin >= numBins {
			maxBin = numBins - 1
		}
		if minBin >= maxBin {
			continue
		}

		for i := 0; i < numFrames; i++ {
			framePeaks := extractBandPeaks(magnitudes, i, minBin, maxBin, numFrames, numBins)
			allPeaks = append(allPeaks, framePeaks...)
		}
	}

	return allPeaks
}

// extractBandPeaks finds the best peaks within a single frequency band for one frame.
func extractBandPeaks(
	magnitudes [][]float64,
	frame, minBin, maxBin, numFrames, numBins int,
) []Peak {
	type candidate struct {
		bin int
		mag float64
	}
	var candidates []candidate

	for j := minBin; j <= maxBin; j++ {
		mag := magnitudes[frame][j]
		if mag <= -90.0 {
			continue // silence / below noise floor
		}

		// --- 3×3 local maximum check ---
		if !isLocalMax(magnitudes, frame, j, 1, 1, numFrames, numBins) {
			continue
		}

		// --- Z-score within neighbourhood ---
		zScore := computeZScore(magnitudes, frame, j, Z_NEIGHBORHOOD, numFrames, numBins)
		if zScore < Z_SCORE_THRESHOLD {
			continue
		}

		candidates = append(candidates, candidate{j, mag})
	}

	if len(candidates) == 0 {
		return nil
	}

	// Keep only the strongest MAX_PEAKS_PER_BAND candidates
	sort.Slice(candidates, func(a, b int) bool {
		return candidates[a].mag > candidates[b].mag
	})
	if len(candidates) > MAX_PEAKS_PER_BAND {
		candidates = candidates[:MAX_PEAKS_PER_BAND]
	}

	peaks := make([]Peak, 0, len(candidates))
	for _, c := range candidates {
		peaks = append(peaks, Peak{
			Time:      float64(frame*HopSize) / float64(SampleRate),
			Frequency: binToFreq(c.bin),
			Magnitude: c.mag,
		})
	}
	return peaks
}

// isLocalMax returns true if magnitudes[t][f] is greater than all neighbours
// within ±dt frames and ±df bins.
func isLocalMax(mag [][]float64, t, f, dt, df, numFrames, numBins int) bool {
	cur := mag[t][f]
	for di := -dt; di <= dt; di++ {
		for dj := -df; dj <= df; dj++ {
			if di == 0 && dj == 0 {
				continue
			}
			ni, nj := t+di, f+dj
			if ni >= 0 && ni < numFrames && nj >= 0 && nj < numBins {
				if mag[ni][nj] >= cur {
					return false
				}
			}
		}
	}
	return true
}

// computeZScore measures how much the peak at (t,f) stands out from its
// local neighbourhood (radius r in both time and frequency dimensions).
func computeZScore(mag [][]float64, t, f, r, numFrames, numBins int) float64 {
	var vals []float64
	for di := -r; di <= r; di++ {
		for dj := -r; dj <= r; dj++ {
			ni, nj := t+di, f+dj
			if ni >= 0 && ni < numFrames && nj >= 0 && nj < numBins {
				vals = append(vals, mag[ni][nj])
			}
		}
	}
	if len(vals) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(len(vals))

	sumSq := 0.0
	for _, v := range vals {
		d := v - mean
		sumSq += d * d
	}
	std := math.Sqrt(sumSq / float64(len(vals)))
	if std == 0 {
		return 0
	}
	return (mag[t][f] - mean) / std
}

// FindPeakRelationships pairs each anchor peak with up to FAN_OUT target peaks
// that fall within the target zone (time window + same frequency band tolerance).
//
// The hash packs (anchorFreqBin, targetFreqBin, deltaMs) into a uint64.
// Bit layout:  anchorFreq[10] | targetFreq[10] | deltaMs[20]  (40 bits used)
// This gives:
//   - freq resolution: ~7.8 Hz / bin at 8 kHz with 1024-bin FFT
//   - deltaT resolution: 1 ms, max 1048 seconds (well beyond TARGET_ZONE_DT_MAX_SECONDS)
func FindPeakRelationships(peaks []Peak, songID string) []db.Fingerprint {
	if len(peaks) == 0 {
		return nil
	}

	// Must be sorted by time so the early-exit break in the inner loop is valid
	sort.Slice(peaks, func(i, j int) bool {
		return peaks[i].Time < peaks[j].Time
	})

	const (
		FREQ_BITS    = 10
		DELTA_T_BITS = 20
	)
	freqBinRes := float64(SampleRate) / float64(WindowSize)

	fingerprints := make([]db.Fingerprint, 0, len(peaks)*FAN_OUT)

	for i, anchor := range peaks {
		maxTime := anchor.Time + TARGET_ZONE_DT_MAX_SECONDS
		fansFound := 0

		for j := i + 1; j < len(peaks) && fansFound < FAN_OUT; j++ {
			target := peaks[j]

			if target.Time > maxTime {
				break // peaks are sorted — nothing later can match
			}

			deltaT := target.Time - anchor.Time
			if deltaT < MIN_DELTA_T {
				continue // too close in time — likely same transient
			}

			anchorBin := uint64(math.Round(anchor.Frequency / freqBinRes))
			targetBin := uint64(math.Round(target.Frequency / freqBinRes))
			deltaMs := uint64(math.Round(deltaT * 1000))

			maskedAnchor := anchorBin & ((1 << FREQ_BITS) - 1)
			maskedTarget := targetBin & ((1 << FREQ_BITS) - 1)
			maskedDelta := deltaMs & ((1 << DELTA_T_BITS) - 1)

			hash := (maskedAnchor << (FREQ_BITS + DELTA_T_BITS)) |
				(maskedTarget << DELTA_T_BITS) |
				maskedDelta

			fingerprints = append(fingerprints, db.Fingerprint{
				SongID:     songID,
				Hash:       hash,
				AnchorTime: anchor.Time,
			})
			fansFound++
		}
	}

	return fingerprints
}
