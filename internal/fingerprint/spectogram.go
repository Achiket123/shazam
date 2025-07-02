package fingerprint

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"math/cmplx"
	"os"
)

func Spectrogram(data []float64) [][]complex128 {
	spectrogram := make([][]complex128, 0)
	Length := len(data)
	for start := 0; start+WindowSize <= Length; start += HopSize {

		frame := data[start : start+WindowSize]
		window := ApplyHanningWindow(frame)

		spectrum := FFT(window)

		spectrogram = append(spectrogram, spectrum[:HopSize])
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

	// --- Step 1: Calculate optimal width and height for 16:9 aspect ---
	scaleFactor := 4 // Increase this for higher resolution
	width := numFrames * scaleFactor
	height := (width * 9) / 16 // Maintain 16:9 aspect ratio

	if height > numBins*scaleFactor {
		height = numBins * scaleFactor
		width = (height * 16) / 9
	}

	// --- Step 2: Prepare magnitude data ---
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

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for x := 0; x < width; x++ {
		spectroX := (x * numFrames) / width
		for y := 0; y < height; y++ {
			spectroY := (y * numBins) / height
			val := (magnitudes[spectroX][spectroY] - minDB) / (maxMagnitude - minDB)
			val = math.Max(0.0, math.Min(1.0, val))
			c := MapToColor(val)

			img.Set(x, height-1-y, c) // Flip vertically
		}
	}

	// --- Step 4: Save image ---
	file, err := os.Create("spectrogram_hd.png")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	png.Encode(file, img)

	return img
}

func MapToColor(val float64) color.Color {

	if val < 0 {
		val = 0
	}
	if val > 1 {
		val = 1
	}

	var r, g, b uint8

	switch {
	case val < 0.25:

		r = 0
		g = uint8(255 * (val / 0.25))
		b = 255
	case val < 0.5:

		r = 0
		g = 255
		b = uint8(255 * (1 - ((val - 0.25) / 0.25)))
	case val < 0.75:

		r = uint8(255 * ((val - 0.5) / 0.25))
		g = 255
		b = 0
	default:

		r = 255
		g = uint8(255 * (1 - ((val - 0.75) / 0.25)))
		b = 0
	}

	return color.RGBA{R: r, G: g, B: b, A: 255}
}
