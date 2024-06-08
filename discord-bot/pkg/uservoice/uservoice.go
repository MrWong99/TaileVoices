package uservoice

import (
	"errors"
	"log/slog"
	"math"
	"time"

	"github.com/MrWong99/TaileVoices/discord_bot/pkg/audio"
	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"gopkg.in/hraban/opus.v2"
)

var ErrVoiceClosed = errors.New("voice processing has been closed")

const (
	minimumAudioLength     = 2 * time.Second        // Minimum length of audio to be processed
	maximumAudioLength     = 29 * time.Second       // Maximum length of audio to be processed
	silenceLengthCutoff    = 500 * time.Millisecond // Length of silence to trigger processing
	silenceThreshold       = 0.01                   // Threshold to consider audio as silence
	discordAudioSampleRate = 48000                  // Discord audio sample rate
	discordFrameSize       = 960                    // Frame size for Discord audio (480 samples * 2 channels)
)

// TextSegment represents a segment of transcribed text with start and end timestamps.
type TextSegment struct {
	Text  string        // Transcribed text
	Start time.Duration // Start time of the text segment
	End   time.Duration // End time of the text segment
}

// Voice represents a voice processing instance for a single user.
type Voice struct {
	Username    string           // Username of the user
	SSRC        uint32           // SSRC identifier
	decoder     *opus.Decoder    // Opus decoder
	stt         *audio.STT       // Speech-to-text processor
	voiceStart  time.Time        // Start time of the voice processing
	results     chan TextSegment // Channel to send text segments
	inputBuffer chan []byte      // Buffer to receive audio data
	closed      bool             // Flag to indicate if processing is closed
	lastErr     error            // Last error encountered during processing
}

// NewVoice creates a new Voice instance for a given user.
func NewVoice(username string, ssrc uint32, language string) (*Voice, error) {
	dec, err := opus.NewDecoder(discordAudioSampleRate, 2)
	if err != nil {
		return nil, err
	}
	stt, err := audio.NewSTT(language)
	if err != nil {
		return nil, err
	}
	v := &Voice{
		Username:    username,
		SSRC:        ssrc,
		decoder:     dec,
		stt:         stt,
		results:     make(chan TextSegment, 10),
		inputBuffer: make(chan []byte, 10),
		voiceStart:  time.Now(),
	}
	go v.processingLoop()
	return v, nil
}

// processingLoop processes incoming audio data and handles transcription.
func (v *Voice) processingLoop() {
	audioBuffer := make([]float32, 0, whisper.SampleRate*int(math.Ceil(maximumAudioLength.Seconds())))
	var audioLength time.Duration
	silenceTicker := time.NewTicker(silenceLengthCutoff)
	defer silenceTicker.Stop()

	processSample := false

	for {
		select {
		case data, ok := <-v.inputBuffer:
			if !ok {
				return
			}
			silenceTicker.Reset(silenceLengthCutoff)

			frameAudio := make([]float32, discordFrameSize*2)
			n, err := v.decoder.DecodeFloat32(data, frameAudio)
			if err != nil {
				v.lastErr = err
				slog.Error("there was an error during decoding", "error", err)
				continue
			}
			frameAudio = frameAudio[:n]

			monoPcm := audio.ConvertStereoToMono(frameAudio)
			audioBuffer = append(audioBuffer, audio.ResamplePCM(monoPcm, discordAudioSampleRate, whisper.SampleRate)...)

			audioLength = audio.AudioLength(audioBuffer, whisper.SampleRate, 1)
		case <-silenceTicker.C:
			// We only want to append silence if there is any audio present
			if audioLength == 0 {
				continue
			}
			processSample = true
		}

		if audioLength < minimumAudioLength && !processSample {
			continue
		}
		if processSample || audioLength > maximumAudioLength {
			processSample = false
			bufferCopy := make([]float32, len(audioBuffer))
			copy(bufferCopy, audioBuffer)
			v.processBuffer(bufferCopy, audioLength)
			audioBuffer = make([]float32, 0, whisper.SampleRate*int(math.Ceil(maximumAudioLength.Seconds())))
			audioLength = 0
		}
	}
}

// processBuffer processes the audio buffer and sends transcribed segments to the results channel.
func (v *Voice) processBuffer(audioBuffer []float32, audioLength time.Duration) {
	start := time.Since(v.voiceStart)
	end := start + audio.AudioLength(audioBuffer, whisper.SampleRate, 1)
	v.stt.TranscribeWithCallback(audioBuffer, audioLength, func(s whisper.Segment) {
		if v.closed {
			return
		}
		v.results <- TextSegment{
			Text:  s.Text,
			Start: start,
			End:   end,
		}
	})
}

// Process processes given Discord audio data. Returns ErrVoiceClosed if Close() has been called already.
func (v *Voice) Process(data []byte) error {
	if v.closed {
		return ErrVoiceClosed
	}
	v.inputBuffer <- data
	return nil
}

// C returns the channel that provides text results as soon as they are available. The channel will be closed immediately if Close() is called.
func (v *Voice) C() <-chan TextSegment {
	return v.results
}

// Close closes the voice recording and its result channel.
func (v *Voice) Close() {
	v.closed = true
	close(v.results)
	close(v.inputBuffer)
}

// Err returns the last error the voice processing encountered, if any.
func (v *Voice) Err() error {
	return v.lastErr
}
