package audio

import (
	"fmt"
	"io"
	"math"
	"shazam/internal/fingerprint"
	"strings"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	go_mp3 "github.com/hajimehoshi/go-mp3"
)

// DownSamplingAudio converts any supported input audio file to WAV format with 44100Hz, 16-bit, mono.

func DownSamplingAudio(file io.ReadSeeker, fileName string) (*[]float64, error) { // Changed return type
	splitName := strings.Split(fileName, ".")
	format := splitName[len(splitName)-1]
	fmt.Println(format)

	var pcmSamples []float64
	var originalSampleRate int

	if format == "wav" {
		fmt.Println("wav")
		decoder := wav.NewDecoder(file)
		if !decoder.IsValidFile() {
			return nil, fmt.Errorf("invalid WAV file")
		}
		err := decoder.FwdToPCM()
		if err != nil {
			return nil, err
		}
		// Read all PCM data into an IntBuffer
		buf := audio.IntBuffer{
			Format: &audio.Format{
				NumChannels: int(decoder.NumChans),
				SampleRate:  int(decoder.SampleRate)},
			Data: make([]int, decoder.PCMLen()),
		}
		if _, err := decoder.PCMBuffer(&buf); err != nil { // Reads all PCM data
			return nil, fmt.Errorf("failed to decode WAV to PCM: %w", err)
		}

		originalSampleRate = buf.Format.SampleRate

		// Convert to mono if necessary and then to float64
		if buf.Format.NumChannels > 1 {
			monoData := make([]int, len(buf.Data)/buf.Format.NumChannels)
			for i := 0; i < len(monoData); i++ {
				sum := 0
				for c := 0; c < buf.Format.NumChannels; c++ {
					sum += buf.Data[i*buf.Format.NumChannels+c]
				}
				monoData[i] = sum / buf.Format.NumChannels
			}
			pcmSamples = make([]float64, len(monoData))
			for i, v := range monoData {
				pcmSamples[i] = float64(v)
			}
		} else {
			lpData := fingerprint.LowpassFilter(buf.AsFloatBuffer().Data, 5512.5, float64(decoder.SampleRate))
			pcmSamples = lpData // Already mono, convert to float64
		}

	} else if format == "mp3" { // Changed 'else' to 'else if' for clarity
		decoder, err := go_mp3.NewDecoder(file) // Check for error here
		if err != nil {
			return nil, fmt.Errorf("failed to create MP3 decoder: %w", err)
		}

		originalSampleRate = decoder.SampleRate()

		// Use a buffer for decoding MP3 directly into float64 or int samples
		// The go-mp3 decoder typically decodes to 16-bit PCM.
		// You might need to adapt this part based on how go-mp3 provides its output.
		// A common pattern is to read into a byte slice and then convert to audio samples.

		var rawPCMBytes []byte       // Stores decoded 16-bit PCM bytes
		tmpBuf := make([]byte, 4096) // Small buffer for reading
		for {
			n, err := decoder.Read(tmpBuf) // Read into byte buffer
			if err != nil && err != io.EOF {
				return nil, fmt.Errorf("error reading MP3: %w", err)
			}
			if n == 0 && err == io.EOF {
				break
			}
			rawPCMBytes = append(rawPCMBytes, tmpBuf[:n]...)
		}

		// Convert rawPCMBytes (interleaved 16-bit) to mono float64 samples
		// Assuming 16-bit signed little-endian PCM from go-mp3
		pcmSamples = make([]float64, 0, len(rawPCMBytes)/2) // Pre-allocate assuming mono
		for i := 0; i < len(rawPCMBytes); i += 4 {          // For stereo 16-bit: 4 bytes per frame (2 channels * 2 bytes/sample)
			// Read 1st channel sample
			sample1 := float64(int16(rawPCMBytes[i]) | int16(rawPCMBytes[i+1])<<8)
			// Read 2nd channel sample
			sample2 := float64(int16(rawPCMBytes[i+2]) | int16(rawPCMBytes[i+3])<<8)

			// Average for mono
			pcmSamples = append(pcmSamples, (sample1+sample2)/2.0)
		}
		lpData := fingerprint.LowpassFilter(pcmSamples, 5512.5, float64(decoder.SampleRate()))
		pcmSamples = lpData

	} else {
		return nil, fmt.Errorf("unsupported audio format: %s", format)
	}

	downSampled := DownSampling(pcmSamples, originalSampleRate, fingerprint.SampleRate)

	return &downSampled, nil
}
func DownSampling(pcm []float64, sampleRate int, targetSampleRate int) []float64 {
	if targetSampleRate >= sampleRate || targetSampleRate <= 0 {
		return pcm
	}

	sampleRateFactor := float64(sampleRate) / float64(targetSampleRate)
	newLength := int(float64(len(pcm)) / sampleRateFactor)

	downsampled := make([]float64, 0, newLength)

	for i := 0; i < newLength; i++ {
		// Use round to reduce jitter
		originalIndex := int(math.Round(float64(i) * sampleRateFactor))
		if originalIndex < len(pcm) {
			downsampled = append(downsampled, pcm[originalIndex])
		}
	}

	return downsampled
}
