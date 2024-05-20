package audio

import (
	"errors"
	"io"
	"log/slog"
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

func NewSTT(language string) (*STT, error) {
	stt, err := ttsModel.NewContext()
	if err != nil {
		return nil, err
	}
	if err = stt.SetLanguage(language); err != nil {
		return nil, err
	}
	stt.SetThreads(uint(runtime.NumCPU()))
	stt.SetTranslate(false)
	return &STT{
		language: language,
		ctx:      stt,
	}, nil
}

// Transcribe the given audio data into text segments.
// The language can be set to "auto" or a 2 character country code (e.g. "us").
func (stt *STT) Transcribe(audio []float32) ([]whisper.Segment, error) {
	if err := stt.ctx.Process(audio, nil, nil); err != nil {
		return nil, err
	}
	segments := make([]whisper.Segment, 0)
	for {
		next, err := stt.ctx.NextSegment()
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

// Transcribe the given audio data into text segments and use prompt as reference.
// The language can be set to "auto" or a 2 character country code (e.g. "us").
func (stt *STT) TranscribeWithPromptAndOffset(audio []float32, prompt string, offset time.Duration) ([]whisper.Segment, error) {
	stt.ctx.SetInitialPrompt(prompt)
	stt.ctx.SetOffset(offset)
	if err := stt.ctx.Process(audio, nil, nil); err != nil {
		return nil, err
	}
	segments := make([]whisper.Segment, 0)
	for {
		next, err := stt.ctx.NextSegment()
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
