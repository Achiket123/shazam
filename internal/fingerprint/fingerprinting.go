package fingerprint

import (
	"os"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/mjibson/go-dsp/fft"
)

func ComputeFFT(filepath string) {
	file, err := os.Open(filepath)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	decoder := wav.NewDecoder(file)
	samples := audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  44100},
		SourceBitDepth: 16,
		Data:           make([]int, 4096),
	}
	decoder.PCMBuffer(&samples)

	data := NormalizeInt16(samples.Data)
	fft.FFTReal(data) 

}

func NormalizeInt16(samples []int) []float64 {
	normalized := make([]float64, len(samples))
	const maxInt16 = 32768.0

	for i, sample := range samples {
		normalized[i] = float64(sample) / maxInt16
	}

	return normalized
}
