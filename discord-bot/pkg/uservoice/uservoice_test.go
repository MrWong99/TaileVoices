package uservoice

import (
	"encoding/gob"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/MrWong99/TaileVoices/discord_bot/pkg/audio"
)

func TestMain(m *testing.M) {
	if env, ok := os.LookupEnv("STT_MODEL_PATH"); ok {
		if err := audio.LoadSTTModel(env); err != nil {
			fmt.Printf("could not load model from %s: %v\n", env, err)
			os.Exit(1)
		}
	} else {
		fmt.Println("you must set ENV STT_MODEL_PATH to point to a valid whisper.cpp model file")
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestVoiceTranscription(t *testing.T) {
	f, err := os.Open("recording.raw")
	if err != nil {
		t.Fatalf("could not open sample recording.raw: %v", err)
	}
	defer f.Close()
	dec := gob.NewDecoder(f)
	var storedAudio [][]byte
	if err := dec.Decode(&storedAudio); err != nil {
		t.Fatalf("could not decode stored audio data: %v", err)
	}
	vd, err := New("MrWong99", "auto", 1, 48000, 2)
	if err != nil {
		t.Fatalf("could not setup voice data: %v", err)
	}
	for _, sample := range storedAudio {
		vd.Process(sample)
	}
	for {
		if len(vd.TextSegments) == 0 {
			time.Sleep(2 * time.Second)
		} else {
			break
		}
	}
	t.Log(vd.TextSegments)
}
