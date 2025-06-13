package audio

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	go_mp3 "github.com/hajimehoshi/go-mp3"
)

// ConvertToWAV converts any supported input audio file to WAV format with 44100Hz, 16-bit, mono.
func ConvertToWAV(input string, output string) error {
	file, _ := os.Open("assets/input-1.mp3")
	defer file.Close()
	decoder, _ := go_mp3.NewDecoder(file)
	fmt.Println(decoder.Length())
	pcm := make([]int, 0)
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
			sample := int(int(tmp[i]) | int(tmp[i+1])<<8)
			pcm = append(pcm, sample)
		}
	}

	outFile, err := os.Create("output.wav")
	if err != nil {
		log.Fatalf("Error creating output file: %v", err)
	}
	defer outFile.Close()
	encoder := wav.NewEncoder(outFile, 44100, 16, 1, 1)
	defer encoder.Close()
	data := audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  44100,
		},
		Data:           pcm,
		SourceBitDepth: 16,
	}

	if err := encoder.Write(&data); err != nil {

		log.Fatalf("Error writing to output file: %v", err)
	}
	return nil
}
