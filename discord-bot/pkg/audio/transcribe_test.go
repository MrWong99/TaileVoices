package audio

import (
	"encoding/gob"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"gopkg.in/hraban/opus.v2"
)

func TestTrascribeWithCallback(t *testing.T) {
	// Load raw sample as recorded from Discord
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

	// Discord Opus decoder
	opusDec, err := opus.NewDecoder(48000, 2)
	if err != nil {
		t.Fatalf("could not create Discord decoder: %v", err)
	}
	// STT for transcriptions with callback
	stt, err := NewSTT("de")
	if err != nil {
		t.Fatalf("could not create STT context: %v", err)
	}

	// Wait for all callbacks and collect their results
	wg := new(sync.WaitGroup)
	samples := make([]whisper.Segment, 0)
	samplesLock := new(sync.Mutex)
	samplesCallback := func(s whisper.Segment) {
		defer wg.Done()
		samplesLock.Lock()
		defer samplesLock.Unlock()
		samples = append(samples, s)
		fmt.Printf("[%s -> %s] %s\n", s.Start, s.End, s.Text)
	}

	allBuf := make([]float32, 0)

	for _, data := range storedAudio {
		pcmBuf := make([]float32, 60000)
		n, err := opusDec.DecodeFloat32(data, pcmBuf)
		if err != nil {
			t.Errorf("could not decode one audio package: %v", err)
			continue
		}
		pcmBuf = ConvertStereoToMono(pcmBuf[:n])

		resampleBuf := ResamplePCM(pcmBuf, 48000, whisper.SampleRate)
		allBuf = append(allBuf, resampleBuf...)

		// We need atleast 5s of audio
		if len(allBuf) < whisper.SampleRate*5 {
			continue
		}
		wg.Add(1)
		if err := stt.TranscribeWithCallback(allBuf, samplesCallback); err != nil {
			t.Errorf("unexpected error during transcribing: %v", err)
		}
		allBuf = make([]float32, 0)
	}
	// Add remaining audio
	if len(allBuf) > whisper.SampleRate {
		wg.Add(1)
		if err := stt.TranscribeWithCallback(allBuf, samplesCallback); err != nil {
			t.Errorf("unexpected error during transcribing: %v", err)
		}
	}
	wg.Wait()

	if len(samples) == 0 {
		t.Error("samples were empty?!")
	} else {
		t.Log(samples)
	}
}

func TestAudioLength(t *testing.T) {
	length := AudioLength(make([]float32, 16000*3), 16000, 1)
	if length != 3*time.Second {
		t.Errorf("expected audio length to be calculated to 3s but got %s", length)
	}
}

func TestHasEnoughSilence(t *testing.T) {
	// Example data
	sampleRate := 48000
	channels := 2
	threshold := 0.01                // Threshold for silence, adjust as necessary
	desiredLength := 2 * time.Second // Desired length of silence

	// Generate example PCM data (10 seconds of stereo audio)
	data := make([]float32, sampleRate*channels*10)
	// Fill with some non-silent data and add 3 seconds of silence at the end
	for i := 0; i < len(data)-sampleRate*channels*3; i++ {
		data[i] = 0.02
	}

	index := HasEnoughSilence(data, desiredLength, sampleRate, channels, threshold)
	if index != -1 {
		t.Logf("found silence starting at index %d", index)
	} else {
		t.Error("there should be enough silence, but none was found")
	}
}
