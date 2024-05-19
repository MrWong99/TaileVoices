package audio

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

func TestTranscribe(t *testing.T) {
	var stt *sample
	for _, sample := range samples {
		if sample.localPath == sttSample {
			stt = &sample
			break
		}
	}
	if stt == nil {
		t.Fatal("no stt sample defined")
	}
	f, err := os.Open(stt.localPath)
	if err != nil {
		t.Fatalf("could not open stt sample: %v", err)
	}
	defer f.Close()
	buf := new(bytes.Buffer)
	err = Resample(&AudioInput{
		Data:       f,
		Channels:   stt.channels,
		SampleRate: stt.sampleRate,
		Format:     stt.format,
	}, &AudioOutput{
		Output:     buf,
		Channels:   stt.channels,
		SampleRate: stt.sampleRate,
		Format:     F32le,
	})
	if err != nil {
		t.Fatalf("could not convert stt sample to float32: %v", err)
	}
	audioData := make([]float32, len(buf.Bytes())/4)
	if err = binary.Read(buf, binary.LittleEndian, audioData); err != nil {
		t.Fatalf("read stt sample as float32: %v", err)
	}
	samples, err := Transcribe(audioData, "en")
	if err != nil {
		t.Fatalf("could not transcribe stt sample: %v", err)
	}
	for _, sample := range samples {
		t.Log(sample.Text)
	}
}
