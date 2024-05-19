package audio

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if err := downloadSamples(); err != nil {
		fmt.Printf("setup failed: %v\n", err)
		os.Exit(1)
	}
	if env, ok := os.LookupEnv("STT_MODEL_PATH"); ok {
		if err := LoadSTTModel(env); err != nil {
			fmt.Printf("could not load model from %s: %v\n", env, err)
			os.Exit(1)
		}
	} else {
		fmt.Println("you must set ENV STT_MODEL_PATH to point to a valid whisper.cpp model file")
		os.Exit(1)
	}
	os.Exit(m.Run())
}

type sample struct {
	format     AudioFormat
	localPath  string
	sourceUrl  string
	channels   int
	sampleRate int
}

const (
	mp3Sample = "./sample.mp3"
	wavSample = "./sample.wav"
	sttSample = "./stt_sample.wav"
)

var samples = []sample{
	{
		format:     Mp3,
		localPath:  mp3Sample,
		sourceUrl:  "https://github.com/rafaelreis-hotmart/Audio-Sample-files/raw/master/sample.mp3",
		channels:   2,
		sampleRate: 44100,
	},
	{
		format:     Wav,
		localPath:  wavSample,
		sourceUrl:  "https://github.com/rafaelreis-hotmart/Audio-Sample-files/raw/master/sample.wav",
		channels:   2,
		sampleRate: 16000,
	},
	{
		format:     Wav,
		localPath:  sttSample,
		sourceUrl:  "https://opus-codec.org/static/examples/samples/speech_24kbps_swb.wav",
		channels:   1,
		sampleRate: 48000,
	},
}

func downloadSamples() error {
	for _, sample := range samples {
		_, err := os.Stat(sample.localPath)
		if err == nil {
			fmt.Printf("sample for format %s already present\n", sample.format)
			continue
		}
		resp, err := http.Get(sample.sourceUrl)
		if err != nil {
			return fmt.Errorf("could not download %s sample: %w", sample.format, err)
		}
		content, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("could not download %s sample: %w", sample.format, err)
		}
		if err := os.WriteFile(sample.localPath, content, 0666); err != nil {
			return fmt.Errorf("could not save %s sample: %w", sample.format, err)
		}
		fmt.Printf("%s sample downloaded successfully\n", sample.format)
	}
	return nil
}
