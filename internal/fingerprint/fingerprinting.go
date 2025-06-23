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
	SampleRate                  = 44100
	window, threshold, maxPeaks = 11, 80, 20
	WindowSize                  = 4096
	HopSize                     = 2048
	DeltaTMin                   = 0.1
	DeltaTMax                   = 2.0
	DeltaFMax                   = 1000.0
)

var FREQ_BANDS = [][]float64{
	{30, 100},    // Low bass
	{100, 250},   // Upper bass
	{250, 500},   // Low mids
	{500, 1000},  // Mids
	{1000, 2500}, // High mids
	{2500, 5000}, // Presence
}

type Peak struct {
	Time float64
	Freq float64
	Amp  float64
}

func Fingerprint(data *[]float64, fileName string) []db.Fingerprint {
	start := time.Now().Nanosecond()

	var PEAKS []Peak

	fmt.Printf("LENGTH OF DATA : %v\n", len(*data))
	lpData := LowpassFilter(*data, 1000, SampleRate)
	spectrogram := Spectrogram(lpData)
	fmt.Printf("LENGTH OF SPECTROGRAM : %v\n", len(spectrogram))

	peaks := ExtractRobustPeaks(spectrogram, fileName)

	PEAKS = append(PEAKS, peaks...)
	fmt.Printf("LENGTH OF PEAKS : %v\n", len(PEAKS))

	pairs := FindPeakRelationships(PEAKS, fileName)
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
