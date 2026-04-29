package fingerprint

import (
	"math"
	"math/cmplx"
)

// Spectrogram computes a short-time Fourier transform (STFT) of the input signal.
// Returns only the positive-frequency bins (first WindowSize/2+1 bins).
func Spectrogram(data []float64) [][]complex128 {
	spectrogram := make([][]complex128, 0)
	length := len(data)
	numBins := WindowSize/2 + 1 // only positive frequencies

	for start := 0; start+WindowSize <= length; start += HopSize {
		frame := data[start : start+WindowSize]
		windowed := applyHanningWindow(frame)
		spectrum := FFT(windowed)
		// Keep only positive-frequency bins
		bin := make([]complex128, numBins)
		copy(bin, spectrum[:numBins])
		spectrogram = append(spectrogram, bin)
	}

	return spectrogram
}

// applyHanningWindow applies a Hann window to reduce spectral leakage.
func applyHanningWindow(frame []float64) []complex128 {
	N := len(frame)
	windowed := make([]complex128, N)
	for i := 0; i < N; i++ {
		w := 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(N-1)))
		windowed[i] = complex(frame[i]*w, 0)
	}
	return windowed
}

// getMagnitudesDB converts complex spectrogram to dB-scale magnitudes.
func getMagnitudesDB(spectrogram [][]complex128) [][]float64 {
	numFrames := len(spectrogram)
	if numFrames == 0 {
		return nil
	}
	numBins := len(spectrogram[0])
	magnitudes := make([][]float64, numFrames)
	for t := 0; t < numFrames; t++ {
		magnitudes[t] = make([]float64, numBins)
		for f := 0; f < numBins; f++ {
			mag := cmplx.Abs(spectrogram[t][f])
			if mag < 1e-10 {
				magnitudes[t][f] = -100.0
			} else {
				magnitudes[t][f] = 20 * math.Log10(mag)
			}
		}
	}
	return magnitudes
}

// freqToBin converts a frequency in Hz to the nearest FFT bin index.
func freqToBin(hz float64) int {
	bin := int(math.Round(hz * float64(WindowSize) / float64(SampleRate)))
	maxBin := WindowSize / 2
	if bin > maxBin {
		return maxBin
	}
	return bin
}

// binToFreq converts an FFT bin index to its centre frequency in Hz.
func binToFreq(bin int) float64 {
	return float64(bin) * float64(SampleRate) / float64(WindowSize)
}
