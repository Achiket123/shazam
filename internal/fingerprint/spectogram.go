package fingerprint

import (
	"image"
	"image/color"
	"math"
	"math/cmplx"
)

const (
	frameSize  = 4096
	hopSize    = 2058
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

		spectrum := FFT(window)

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

	magnitudes := make([][]float64, numFrames)
	maxMagnitude := -1e9

	for i, frame := range spectrogram {
		magnitudes[i] = make([]float64, numBins)
		for j, c := range frame {

			mag := cmplx.Abs(c)

			db := 20 * math.Log10(mag+1e-9)
			magnitudes[i][j] = db

			if db > maxMagnitude {
				maxMagnitude = db
			}
		}
	}

	minDB := maxMagnitude - 80.0
	img := image.NewRGBA(image.Rect(0, 0, numFrames, numBins))

	for x := 0; x < numFrames; x++ {
		for y := 0; y < numBins; y++ {
			val := (magnitudes[x][y] - minDB) / (maxMagnitude - minDB)
			val = max(0.0, min(1.0, val))

			c := mapToColor(val)

			if x%scaleX == 0 && y%scaleY == 0 {
				img.Set(x/scaleX, (numBins-1-y)/scaleY, c)
			}
		}
	}

	return img
}

func mapToColor(value float64) color.Color {

	value = math.Max(0, math.Min(1, value))

	var r, g, b uint8
	if value < 0.25 {

		r = 0
		g = uint8(4 * value * 255)
		b = 255
	} else if value < 0.5 {

		r = 0
		g = 255
		b = uint8(255 * (1 - 4*(value-0.25)))
	} else if value < 0.75 {

		r = uint8(255 * 4 * (value - 0.5))
		g = 255
		b = 0
	} else {

		r = 255
		g = uint8(255 * (1 - 4*(value-0.75)))
		b = 0
	}
	return color.RGBA{R: r, G: g, B: b, A: 255}
}
