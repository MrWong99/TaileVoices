package stt

import (
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

var modelPath string

// SetModelPath for the whisper.cpp model binary to use.
func SetModelPath(path string) {
	slog.Info("whisper model path set", "path", path)
	modelPath = path
}

// SpeechToText wraps whisper.cpp for transcribing.
type SpeechToText struct {
	// Language that should be transcribed. Can be "auto" for some models.
	Language        string
	model           whisper.Model
	context         whisper.Context
	processingQueue chan processingRequest
}

type processingRequest struct {
	data      []float32
	startTime time.Time
	callback  SegmentCallback
}

// New returns a SpeechToText setup for given model and language.
func New(lang string) (*SpeechToText, error) {
	model, err := whisper.New(modelPath)
	if err != nil {
		return nil, err
	}
	if lang != "auto" && !slices.Contains(model.Languages(), lang) {
		return nil, fmt.Errorf("model doesn't support language %q but only %v", lang, model.Languages())
	}
	ctx, err := model.NewContext()
	if err != nil {
		model.Close()
		return nil, err
	}
	ctx.SetTranslate(false)
	if err := ctx.SetLanguage(lang); err != nil {
		model.Close()
		return nil, err
	}
	queue := make(chan processingRequest, 10)
	go handleQueue(ctx, queue)
	return &SpeechToText{
		Language:        lang,
		model:           model,
		context:         ctx,
		processingQueue: queue,
	}, nil
}

// SegmentCallback can be passed to SpeechToText.Process so any newly generated segments can
// be processed immediately once they are generated.
// The processingDelay is the delay it took between when the initial Process() method was called
// and the moment the callback is called.
type SegmentCallback func(segment whisper.Segment, processingDelay time.Duration)

// Process mono audio data and return any errors.
// If defined, newly generated segments are passed to the
// callback function during processing.
//
// Internally each call to Process will be put into a queue that is limited to 10 requests
// after which each call to Process will become blocking until a new slot in the queue is
// available.
func (s *SpeechToText) Process(pcmData []float32, callback SegmentCallback) {
	s.processingQueue <- processingRequest{
		data:      pcmData,
		startTime: time.Now(),
		callback:  callback,
	}
}

// handleQueue is setup as goroutine in New
func handleQueue(context whisper.Context, queue <-chan processingRequest) {
	for {
		request, ok := <-queue
		if !ok {
			return
		}
		var cb whisper.SegmentCallback
		if request.callback != nil {
			cb = func(r processingRequest) whisper.SegmentCallback {
				return func(s whisper.Segment) {
					delay := time.Since(r.startTime)
					r.callback(s, delay)
				}
			}(request)
		}
		if err := context.Process(request.data, cb, nil); err != nil {
			slog.Error("could not process audio data", "error", err)
		}
	}
}

// Close the underlying whisper.cpp model.
func (s *SpeechToText) Close() error {
	close(s.processingQueue)
	return s.model.Close()
}
