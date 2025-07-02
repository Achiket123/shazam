package fingerprint

import (
	"fmt"
	"math"
	"shazam/internal/db"
	"time"
)

// frameSize: Number of audio samples per analysis frame. Larger values improve frequency resolution, but reduce time resolution.
// hopSize: Number of samples to advance for each frame (overlap = frameSize - hopSize).
// sampleRate: Expected audio sample rate (Hz).
// window: Window size for local peak detection in the spectrogram.
// threshold: Minimum dB value for a point to be considered as a peak.
// maxPeaks: Maximum number of peaks to detect per frame.
// fanout: Number of target peaks to pair with each anchor peak for fingerprint generation.
// maxDeltaT: Maximum time difference (in frames) between anchor and target peaks for fingerprinting.
const (
	SampleRate                  = 8000
	window, threshold, maxPeaks = 11, 80, 20
	WindowSize                  = 1024
	HopSize                     = 512
	DeltaTMin                   = 0.1
	DeltaTMax                   = 2.0
	DeltaFMax                   = 1000.0
)

var FREQ_BANDS = []struct{ min, max int }{{0, 10}, {10, 20}, {20, 40}, {40, 80}, {80, 160}, {160, 512}}

func Fingerprint(data *[]float64, fileName string) []db.Fingerprint {
	start := time.Now().Nanosecond()

	fmt.Printf("LENGTH OF DATA : %v\n", len(*data))

	spectrogram := Spectrogram(*data)
	fmt.Printf("LENGTH OF SPECTROGRAM : %v\n", len(spectrogram))

	peaks := ExtractPeaks(spectrogram)
	fmt.Printf("LENGTH OF ROBUST PEAKS : %v\n", len(peaks))

	pairs := FindPeakRelationships(peaks, fileName)
	fmt.Printf("LENGTH OF PEAK PAIRS %v\n", len(pairs))
	end := time.Now().Nanosecond()

	timeto := end - start
	fmt.Printf("END : %v\n", end)
	fmt.Printf("START : %v\n", start)

	fmt.Printf("TIME TO READ : %v\n", timeto)

	return pairs

}
func NormalizeInt16Array(samples []int) []float64 {
	normalized := make([]float64, len(samples))

	for i, sample := range samples {
		normalized[i] = float64(sample)
	}

	return normalized
}
func NormalizeInt16(sample int) float64 {

	return float64(sample)
}

func LowpassFilter(sample []float64, cutoffFreq, samplerate float64) []float64 {
	res := make([]float64, len(sample))
	rc := 1 / (2 * math.Pi * cutoffFreq)
	dt := 1 / samplerate
	a := dt / (rc + dt)
	res[0] = a * sample[0]
	for i := 1; i < len(sample); i++ {
		res[i] = a*sample[i] + (1-a)*sample[i-1]

	}

	return res

}
