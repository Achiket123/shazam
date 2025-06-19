package fingerprint

import (
	"image"
	"image/color"
	"math"
	"math/cmplx"
)

const (
	frameSize  = 4096 // Adjust as needed
	hopSize    = 2058 // Adjust as needed
	windowSize = frameSize
	scaleX     = 4
	scaleY     = 1
)

func Spectrogram(data []float64) [][]complex128 {
	spectrogram := make([][]complex128, 0)
	Length := len(data)
	for start := 0; start+windowSize <= Length; start += hopSize {

		frame := data[start : start+windowSize]
		window := ApplyHanningWindow(frame)

		// Compute FFT
		spectrum := FFT(window)
		// Take first half (positive frequencies)
		spectrogram = append(spectrogram, spectrum[:hopSize])
	}

	return spectrogram
}

func ApplyHanningWindow(frame []float64) []complex128 {
	N := len(frame)
	windowed := make([]complex128, N)
	for i := 0; i < N; i++ {
		windowed[i] = complex(frame[i]*0.5*(1-math.Cos(2*math.Pi*float64(i)/float64(N-1))), 0)
	}
	return windowed
}
func createSpectrogramImage(spectrogram [][]complex128) image.Image {
	if len(spectrogram) == 0 {
		return image.NewRGBA(image.Rect(0, 0, 0, 0))
	}

	numFrames := len(spectrogram)
	numBins := len(spectrogram[0])

	// 1. Calculate Magnitudes and convert to dB scale
	magnitudes := make([][]float64, numFrames)
	maxMagnitude := -1e9 // Use a very small number to start

	for i, frame := range spectrogram {
		magnitudes[i] = make([]float64, numBins)
		for j, c := range frame {
			// Magnitude of the complex number
			mag := cmplx.Abs(c)

			// Convert to decibels (dB). Add a small epsilon to avoid log(0).
			// A reference value of 1.0 is common.
			db := 20 * math.Log10(mag+1e-9)
			magnitudes[i][j] = db

			if db > maxMagnitude {
				maxMagnitude = db
			}
		}
	}

	// 2. Normalize magnitudes to the [0, 1] range for color mapping.
	// We'll use a dynamic range of 80 dB below the max.
	minDB := maxMagnitude - 80.0
	img := image.NewRGBA(image.Rect(0, 0, numFrames, numBins))

	for x := 0; x < numFrames; x++ {
		for y := 0; y < numBins; y++ {
			val := (magnitudes[x][y] - minDB) / (maxMagnitude - minDB)
			val = max(0.0, min(1.0, val))

			c := mapToColor(val)

			// Apply scaling: fill the pixel block
			if x%scaleX == 0 && y%scaleY == 0 {
				img.Set(x/scaleX, (numBins-1-y)/scaleY, c)
			}
		}
	}

	return img
}

// mapToColor maps a value from 0.0 (cold) to 1.0 (hot) to a color.
// This is a simple "viridis-like" colormap.
func mapToColor(value float64) color.Color {
	// Clamp value to [0, 1]
	value = math.Max(0, math.Min(1, value))

	var r, g, b uint8
	if value < 0.25 {
		// Blue to Cyan
		r = 0
		g = uint8(4 * value * 255)
		b = 255
	} else if value < 0.5 {
		// Cyan to Green
		r = 0
		g = 255
		b = uint8(255 * (1 - 4*(value-0.25)))
	} else if value < 0.75 {
		// Green to Yellow
		r = uint8(255 * 4 * (value - 0.5))
		g = 255
		b = 0
	} else {
		// Yellow to Red
		r = 255
		g = uint8(255 * (1 - 4*(value-0.75)))
		b = 0
	}
	return color.RGBA{R: r, G: g, B: b, A: 255}
}
