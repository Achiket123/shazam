package audio

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	go_mp3 "github.com/hajimehoshi/go-mp3"
)

const targetDownSampleRate = 23000

// DownSamplingAudio converts any supported input audio file to WAV format with 44100Hz, 16-bit, mono.
func DownSamplingAudio(file *os.File) (*[]float64, error) {
	fileName := file.Name()
	splitName := strings.Split(fileName, ".")
	format := splitName[len(splitName)-1]
	if format == "wav" {
		fmt.Println("wav")
		decoder := wav.NewDecoder(file)
		if !decoder.IsValidFile() {
			return nil, fmt.Errorf("invalid WAV file")
		}

		err := decoder.FwdToPCM()
		if err != nil {
			panic(err)
		}

		length := decoder.PCMSize

		fmt.Println(length)

		buf := audio.IntBuffer{Data: make([]int, length/2), Format: &audio.Format{NumChannels: 1, SampleRate: targetDownSampleRate}}

		_, err = decoder.PCMBuffer(&buf)
		if err != nil {
			panic(err)
		}
		fmt.Println(buf.Data[length/2-1])
		downSampled := DownSampling(buf.AsFloatBuffer().Data, buf.Format.SampleRate, targetDownSampleRate)

		return &downSampled, nil
	}
	decoder, _ := go_mp3.NewDecoder(file)
	fmt.Println(decoder.Length())
	pcm := make([]float64, 0)
	tmp := make([]byte, decoder.Length())

	for {
		n, err := decoder.Read(tmp)

		if err != nil && err != io.EOF {
			log.Fatalf("Error reading MP3: %v", err)
			break
		}
		if n == 0 {
			break
		}
		for i := 0; i < n; i += 4 {
			if i+1 >= len(tmp) {
				break
			}
			sample := float64(int(tmp[i]) | int(tmp[i+1])<<8)
			pcm = append(pcm, sample)
		}
	}
	downSampled := DownSampling(pcm, decoder.SampleRate(), targetDownSampleRate)

	return &downSampled, nil

}

func DownSampling(pcm []float64, SampleRate int, targetSampleRate int) []float64 {

	sampleRateFactor := SampleRate / targetSampleRate
	downsampled := make([]float64, len(pcm)/sampleRateFactor)
	for i := 0; i < len(pcm); i += sampleRateFactor {
		if pcm[i] == 0 {
			continue
		}
		downsampled = append(downsampled, pcm[i])
	}
	return downsampled

}
func UpSampling(pcm []float64, originalSampleRate int, targetSampleRate int) []float64 {
	if originalSampleRate >= targetSampleRate || originalSampleRate <= 0 || targetSampleRate <= 0 {
		return pcm
	}
	ratio := float64(targetSampleRate) / float64(originalSampleRate)
	upsampledLen := int(float64(len(pcm)) * ratio)
	upsampled := make([]float64, upsampledLen)

	for i := 0; i < upsampledLen; i++ {
		srcIdx := float64(i) / ratio
		idx := int(srcIdx)
		if idx >= len(pcm)-1 {
			upsampled[i] = pcm[len(pcm)-1]
		} else {
			// Linear interpolation
			frac := srcIdx - float64(idx)
			upsampled[i] = pcm[idx]*(1-frac) + pcm[idx+1]*frac
		}
	}
	return upsampled
}
