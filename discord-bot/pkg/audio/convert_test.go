package audio

import (
	"bytes"
	"os"
	"testing"
)

func TestResample(t *testing.T) {
	for _, sample := range samples {
		f, err := os.Open(sample.localPath)
		if err != nil {
			t.Errorf("could not read sample file: %v", err)
			continue
		}
		defer f.Close()
		outBuf := new(bytes.Buffer)
		err = Resample(&AudioInput{
			Data:       f,
			Channels:   sample.channels,
			SampleRate: sample.sampleRate,
			Format:     sample.format,
		}, &AudioOutput{
			Output:     outBuf,
			Channels:   1,
			SampleRate: 12000,
			Format:     Opus,
		})
		if err != nil {
			t.Errorf("could not resample %s to opus: %v", sample.format, err)
			continue
		}
		err = os.WriteFile(sample.localPath+".resampled", outBuf.Bytes(), 0666)
		if err != nil {
			t.Errorf("saved resampled data for %s: %v", sample.format, err)
			continue
		}
	}
}
