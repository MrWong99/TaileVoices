package audio

import (
	"errors"
	"math"
	"runtime"
	"time"

	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

var ttsModel whisper.Model

// LoadSTTModel must be called once before Transcribe can be called.
// The path should point to a valid multilangual ggml binary model file.
func LoadSTTModel(path string) (err error) {
	ttsModel, err = whisper.New(path)
	return
}

// UnloadSTTModel should be called to unload the previously loaded ggml model.
func UnloadSTTModel() error {
	return ttsModel.Close()
}

// ErrModelNotLoaded will be returned when Transcribe was called but no model is loaded.
var ErrModelNotLoaded = errors.New("call LoadSTTModel() first before Transcribe()")

type STT struct {
	language string
	ctx      whisper.Context
}

// NewSTT creates a new speech-to-text context.
func NewSTT(language string) (*STT, error) {
	stt := &STT{
		language: language,
	}
	err := stt.newContext()
	return stt, err
}

func (s *STT) newContext() error {
	stt, err := ttsModel.NewContext()
	if err != nil {
		return err
	}
	if err = stt.SetLanguage(s.language); err != nil {
		return err
	}
	stt.SetThreads(uint(runtime.NumCPU()))
	stt.SetTranslate(false)
	s.ctx = stt
	return nil
}

// TranscribeWithCallback the given audio data into text segments.
// The segments will be given to the callback once produced, but their start and end timestamps won't be exact.
//
// You can set an offset to influence the start timestamp.
func (stt *STT) TranscribeWithCallback(audio []float32, audioLength time.Duration, segmentCallback whisper.SegmentCallback) error {
	if audioLength < 30*time.Second {
		missingPadding := (30 * whisper.SampleRate) - len(audio)
		audio = append(audio, make([]float32, missingPadding)...)
	}
	return stt.ctx.Process(audio, segmentCallback, nil)
}

// AudioLength of the given data with set sample rate and channel count.
func AudioLength(data []float32, sampleRate, channels int) time.Duration {
	lengthPerChannel := len(data) / channels
	return time.Second * time.Duration(lengthPerChannel/sampleRate)
}

// HasEnoughSilence returns the starting index of the last audio sample that is followed by desiredLength of silence or -1 if not enough silence exists.
func HasEnoughSilence(data []float32, desiredLength time.Duration, sampleRate, channels int, threshold float64) int {
	// Calculate the number of samples that represent the desired length of silence
	desiredSamples := int(desiredLength.Seconds()) * sampleRate * channels

	// Iterate through the data backwards to find the silence
	silentSamples := 0
	for i := len(data) - 1; i >= 0; i-- {
		if math.Abs(float64(data[i])) < threshold {
			silentSamples++
			if silentSamples >= desiredSamples {
				return i + desiredSamples - 1
			}
		} else {
			silentSamples = 0
		}
	}
	return -1
}
