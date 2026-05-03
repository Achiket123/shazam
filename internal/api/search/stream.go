package search

import (
	"encoding/binary"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"shazam/internal/db"
	"shazam/internal/fingerprint"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// PCM format coming from the Flutter client:
	//   PCM signed 16-bit little-endian, mono, 16 kHz
	// The fingerprinting pipeline expects 8 kHz, so we downsample by 2.
	clientSampleRate = 16000
	targetSampleRate = fingerprint.SampleRate // 8000

	// How many samples to accumulate before running a search pass.
	// 8000 samples at 8 kHz = 1 second of audio per search window.
	searchWindowSamples = targetSampleRate * 3

	// How many new samples must arrive before the next search pass fires.
	// 4000 samples at 8 kHz = 0.5 seconds, so we search every 500 ms of new audio.
	searchHopSamples = targetSampleRate / 2

	// Minimum number of samples in the ring buffer before we attempt a search.
	// At least 2 seconds of audio must have accumulated to get meaningful fingerprints.
	minSamplesBeforeSearch = targetSampleRate * 5

	writeTimeout = 10 * time.Second
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// StreamSearch upgrades the HTTP connection to a WebSocket then:
//   - reads raw PCM16 LE mono 16 kHz binary frames from the client
//   - downsamples each frame to 8 kHz into a growing ring buffer
//   - every searchHopSamples of new audio, fingerprints the buffered audio
//     and pushes a JSON result array back to the client
//   - on receiving a {"type":"eof"} text frame, runs a final search and closes
func StreamSearch(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Default().Printf("[stream] upgrade error: %v\n", err)
		return
	}
	defer conn.Close()

	log.Println("[stream] client connected")

	// Accumulates all downsampled PCM received so far.
	// Grows continuously — we always search the full buffer so that
	// fingerprints spanning multiple chunks are captured correctly.
	var sampleBuffer []float64

	// samplesAtLastSearch tracks how many samples were in the buffer the
	// last time we ran a search. We only run again when enough new samples
	// have arrived (searchHopSamples).
	samplesAtLastSearch := 0

	// downsampleRemainder holds any leftover PCM bytes from the previous
	// chunk that didn't make a complete int16 sample pair for downsampling.
	var rawRemainder []byte

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Default().Printf("[stream] read error: %v\n", err)
			}
			break
		}

		switch msgType {
		case websocket.BinaryMessage:
			// Prepend any leftover bytes from the previous chunk
			chunk := append(rawRemainder, data...)
			rawRemainder = nil

			newSamples, remainder := pcm16ToFloat64Downsampled(chunk)
			rawRemainder = remainder
			sampleBuffer = append(sampleBuffer, newSamples...)

			newCount := len(sampleBuffer) - samplesAtLastSearch
			if len(sampleBuffer) >= minSamplesBeforeSearch && newCount >= searchHopSamples {
				results := runSearch(sampleBuffer)
				samplesAtLastSearch = len(sampleBuffer)
				if len(results) > 0 {
					sendResults(conn, results)
				}
			}

		case websocket.TextMessage:
			var msg map[string]string
			if err := json.Unmarshal(data, &msg); err != nil {
				log.Default().Printf("[stream] bad text frame: %s\n", string(data))
				continue
			}
			if msg["type"] == "eof" {
				log.Default().Println("[stream] EOF received — running final search")
				if len(sampleBuffer) >= minSamplesBeforeSearch {
					results := runSearch(sampleBuffer)
					sendResults(conn, results)
				}
				conn.WriteMessage(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				)
				return
			}
		}
	}
}

// runSearch fingerprints the buffer and queries the DB.
func runSearch(samples []float64) []MatchedSong {
	fps := fingerprint.Fingerprint(&samples, "query")
	if len(fps) == 0 {
		return nil
	}

	results, err := MatchHashes(fps, db.DB)
	if err != nil {
		log.Default().Printf("[stream] MatchHashes error: %v\n", err)
		return nil
	}

	log.Default().Printf("[stream] search pass: %d fingerprints, %d results\n", len(fps), len(results))
	return results
}

// sendResults serialises results to JSON and writes a text frame to the client.
func sendResults(conn *websocket.Conn, results []MatchedSong) {
	payload, err := json.Marshal(results)
	if err != nil {
		log.Default().Printf("[stream] marshal error: %v\n", err)
		return
	}
	conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		log.Default().Printf("[stream] write error: %v\n", err)
	}
}

// pcm16ToFloat64Downsampled converts raw PCM16 LE mono bytes at 16 kHz to
// float64 samples at 8 kHz by simply taking every other sample (2:1 decimation).
//
// Because the client already records at 16 kHz mono and the target is 8 kHz,
// simple 2:1 decimation is correct as long as the signal contains no energy
// above 4 kHz (the Nyquist of 8 kHz). Speech and music recorded by a phone mic
// are typically bandlimited to 8 kHz by the hardware, so no explicit anti-alias
// filter is needed here. If you later switch to 44.1 kHz input you must add one.
//
// Returns the converted samples and any leftover bytes (< 4 bytes) that should
// be prepended to the next chunk.
func pcm16ToFloat64Downsampled(raw []byte) ([]float64, []byte) {
	// Each 16 kHz sample is 2 bytes (int16 LE).
	// We keep every other sample, so consume 4 bytes (2 samples) per output sample.
	stride := 2 * (clientSampleRate / targetSampleRate) // = 4
	numOut := len(raw) / stride
	out := make([]float64, numOut)

	for i := 0; i < numOut; i++ {
		offset := i * stride
		// Read the first of the two 16 kHz samples — drop the second one.
		s := int16(binary.LittleEndian.Uint16(raw[offset : offset+2]))
		// Normalise to float64 in the same range the fingerprinting pipeline expects
		// (it does not normalise internally; it works with raw int16-scaled values).
		out[i] = float64(s)
	}

	leftover := raw[numOut*stride:]
	remainder := make([]byte, len(leftover))
	copy(remainder, leftover)

	return out, remainder
}

// freqToBinLocal mirrors the unexported freqToBin in the fingerprint package.
// Kept here so the stream package has no unexported dependency.
func freqToBinLocal(hz float64) int {
	bin := int(math.Round(hz * float64(fingerprint.WindowSize) / float64(fingerprint.SampleRate)))
	maxBin := fingerprint.WindowSize / 2
	if bin > maxBin {
		return maxBin
	}
	return bin
}
