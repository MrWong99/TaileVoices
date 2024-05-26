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

func TestReadBytes(t *testing.T) {
	expectedInts := []int16{0, 1, 32767, -1}
	first := bytes.NewReader([]byte{0x00, 0x00, 0x01, 0x00, 0xff, 0x7f, 0xff, 0xff})
	res, err := ReadBytes[int16](first)
	if err != nil {
		t.Fatalf("got unexpected error while reading bytes: %v", err)
	}
	if len(res) != len(expectedInts) {
		t.Errorf("didn't get expected amount of data. Expected %d but got %d", len(expectedInts), len(res))
	} else {
		for i, num := range expectedInts {
			if res[i] != num {
				t.Errorf("result at index %d should be %d but it is %d", i, num, res[i])
			}
		}
	}
}
