package fingerprint

import (
	"fmt"
	"math"
	"shazam/internal/db"
	"time"
)

const (
	SampleRate = 8000

	// Larger window = better frequency resolution (7.8 Hz/bin at 8kHz)
	// More overlap = better time resolution and smoother peak detection
	WindowSize = 2048
	HopSize    = 256 // 87.5% overlap — Shazam-level time resolution

	// Pre-emphasis coefficient — boosts high frequencies before FFT
	// so high-freq peaks aren't drowned out by low-freq energy
	PreEmphasisCoeff = 0.97

	DeltaTMin = 0.05 // minimum 50ms between anchor and target
	DeltaTMax = 2.0
	DeltaFMax = 1000.0

	window, threshold, maxPeaks = 11, 80, 20
)

// Log-spaced frequency bands (Hz), mimicking mel-scale perception.
// Peaks in each band are evaluated independently so low-freq dominance
// doesn't suppress musically important high-freq content.
var FreqBands = []struct {
	MinHz, MaxHz float64
}{
	{0, 500},
	{500, 1000},
	{1000, 2000},
	{2000, 4000}, // upper limit at Nyquist for 8kHz sample rate
}

// Fingerprint is the main entry point: audio samples → []db.Fingerprint
func Fingerprint(data *[]float64, fileName string) []db.Fingerprint {
	start := time.Now().UnixNano()
	fmt.Printf("[fp] samples: %d\n", len(*data))

	// Step 1: Pre-emphasis filter — y[n] = x[n] - α*x[n-1]
	emphasized := preEmphasis(*data, PreEmphasisCoeff)

	spectrogram := Spectrogram(emphasized)
	fmt.Printf("[fp] frames: %d\n", len(spectrogram))

	peaks := ExtractPeaks(spectrogram)
	fmt.Printf("[fp] peaks: %d\n", len(peaks))

	pairs := FindPeakRelationships(peaks, fileName)
	fmt.Printf("[fp] fingerprints: %d\n", len(pairs))
	fmt.Printf("[fp] time: %d ms\n", (time.Now().UnixNano()-start)/1_000_000)

	return pairs
}

// preEmphasis boosts high-frequency content before FFT,
// compensating for the natural roll-off of audio signals.
func preEmphasis(samples []float64, coeff float64) []float64 {
	out := make([]float64, len(samples))
	if len(samples) == 0 {
		return out
	}
	out[0] = samples[0]
	for i := 1; i < len(samples); i++ {
		out[i] = samples[i] - coeff*samples[i-1]
	}
	return out
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
	if len(sample) == 0 {
		return res
	}
	res[0] = a * sample[0]
	for i := 1; i < len(sample); i++ {
		res[i] = a*sample[i] + (1-a)*sample[i-1]
	}
	return res
}