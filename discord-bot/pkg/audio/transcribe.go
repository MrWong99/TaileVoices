package audio

import (
	"errors"
	"io"
	"log/slog"

	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

var ttsModel whisper.Model

func LoadSTTModel(path string) (err error) {
	ttsModel, err = whisper.New(path)
	return
}

func UnloadSTTModel() error {
	return ttsModel.Close()
}

var ErrModelNotLoaded = errors.New("call LoadSTTModel() first before Transcribe()")

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
