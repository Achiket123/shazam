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

// DownSamplingAudio decodes any supported audio format to mono float64 PCM
// at fingerprint.SampleRate (8 kHz), ready for fingerprinting.
//
// Processing chain:
//   format decode → stereo-to-mono mix → lowpass anti-aliasing → downsample
//
// The lowpass cutoff is set to Nyquist/2 of the TARGET rate (4 kHz → 3.8 kHz)
// rather than the source rate, ensuring no aliasing after downsampling.
func DownSamplingAudio(file io.ReadSeeker, fileName string) (*[]float64, error) {
	parts := strings.Split(fileName, ".")
	format := strings.ToLower(parts[len(parts)-1])

	var pcmSamples []float64
	var originalSampleRate int

	switch format {
	case "wav":
		var err error
		pcmSamples, originalSampleRate, err = decodeWAV(file)
		if err != nil {
			return nil, err
		}

	case "mp3":
		var err error
		pcmSamples, originalSampleRate, err = decodeMP3(file)
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unsupported audio format: %s", format)
	}

	// Anti-aliasing lowpass before downsampling.
	// Cutoff = 0.95 × Nyquist of target rate to leave a small guard band.
	targetNyquist := float64(fingerprint.SampleRate) / 2.0
	cutoff := targetNyquist * 0.95
	filtered := fingerprint.LowpassFilter(pcmSamples, cutoff, float64(originalSampleRate))

	downSampled := downsample(filtered, originalSampleRate, fingerprint.SampleRate)
	return &downSampled, nil
}

func decodeWAV(file io.ReadSeeker) ([]float64, int, error) {
	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		return nil, 0, fmt.Errorf("invalid WAV file")
	}
	if err := decoder.FwdToPCM(); err != nil {
		return nil, 0, err
	}

	buf := audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: int(decoder.NumChans),
			SampleRate:  int(decoder.SampleRate),
		},
		Data: make([]int, decoder.PCMLen()),
	}
	if _, err := decoder.PCMBuffer(&buf); err != nil {
		return nil, 0, fmt.Errorf("failed to decode WAV PCM: %w", err)
	}

	sampleRate := buf.Format.SampleRate
	numCh := buf.Format.NumChannels
	samples := monoMixInt(buf.Data, numCh)
	return samples, sampleRate, nil
}

func decodeMP3(file io.ReadSeeker) ([]float64, int, error) {
	decoder, err := go_mp3.NewDecoder(file)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create MP3 decoder: %w", err)
	}

	var rawBytes []byte
	buf := make([]byte, 8192)
	for {
		n, err := decoder.Read(buf)
		if n > 0 {
			rawBytes = append(rawBytes, buf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("MP3 read error: %w", err)
		}
	}

	// go-mp3 always outputs stereo 16-bit LE PCM
	samples := make([]float64, 0, len(rawBytes)/4)
	for i := 0; i+3 < len(rawBytes); i += 4 {
		l := float64(int16(uint16(rawBytes[i]) | uint16(rawBytes[i+1])<<8))
		r := float64(int16(uint16(rawBytes[i+2]) | uint16(rawBytes[i+3])<<8))
		samples = append(samples, (l+r)/2.0)
	}

	return samples, decoder.SampleRate(), nil
}

// monoMixInt averages numChannels interleaved int samples into a single mono float64 channel.
func monoMixInt(data []int, numChannels int) []float64 {
	if numChannels == 1 {
		out := make([]float64, len(data))
		for i, v := range data {
			out[i] = float64(v)
		}
		return out
	}
	frames := len(data) / numChannels
	out := make([]float64, frames)
	for i := 0; i < frames; i++ {
		sum := 0
		for c := 0; c < numChannels; c++ {
			sum += data[i*numChannels+c]
		}
		out[i] = float64(sum) / float64(numChannels)
	}
	return out
}

// downsample reduces the sample rate by simple nearest-neighbour resampling.
// The caller must apply a lowpass filter first to avoid aliasing.
func downsample(pcm []float64, fromRate, toRate int) []float64 {
	if toRate >= fromRate || toRate <= 0 {
		return pcm
	}
	factor := float64(fromRate) / float64(toRate)
	newLen := int(float64(len(pcm)) / factor)
	out := make([]float64, 0, newLen)
	for i := 0; i < newLen; i++ {
		idx := int(math.Round(float64(i) * factor))
		if idx < len(pcm) {
			out = append(out, pcm[idx])
		}
	}
	return out
}