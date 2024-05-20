package audio

import (
	"errors"
	"io"
	"log/slog"
	"runtime"

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

// Transcribe the given audio data into text segments.
// The language can be set to "auto" or a 2 character country code (e.g. "us").
func Transcribe(audio []float32, lang string) ([]whisper.Segment, error) {
	if ttsModel == nil {
		return nil, ErrModelNotLoaded
	}
	tts, err := ttsModel.NewContext()
	if err != nil {
		return nil, err
	}
	if err = tts.SetLanguage(lang); err != nil {
		return nil, err
	}
	tts.SetThreads(uint(runtime.NumCPU()))
	tts.SetTranslate(false)
	if err = tts.Process(audio, nil, nil); err != nil {
		return nil, err
	}
	segments := make([]whisper.Segment, 0)
	for {
		next, err := tts.NextSegment()
		if err == nil {
			segments = append(segments, next)
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		slog.Warn("could not transcribe speech segment", "error", err)
		break
	}
	return segments, nil
}
