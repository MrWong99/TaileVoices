package audio

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// AudioFormat as defined by ffmpeg. There are more,
// but these are just the ones we might remotely be interested in.
//
// The AudioFormat.String() in lowercase should always be the parameter that can be supplied
// as ffmpeg audio format parameter directly.
//
//go:generate stringer -type AudioFormat
type AudioFormat int

const (
	// Opus maps to Opus encoded audio.
	Opus AudioFormat = iota
	// F32le is a 32bit floating point slice with little endianess (like []float32).
	F32le
	// S16le is a 316bit signed integer slice with little endianess (like []uint16).
	S16le
	// Wav file format.
	Wav
	// Mp3 file format.
	Mp3
)

type AudioInput struct {
	Data io.Reader
	// Channels is the count of audio channels (1 = mono; 2 = stereo)
	Channels int
	// SampleRate in Hz
	SampleRate int
	// Format of the input data.
	Format AudioFormat
	// OptionalArgs to add to already set format, channels and sample rate.
	OptionalArgs map[string]any
}

// AllArgs that will be set for ffmpeg audio input.
func (a *AudioInput) AllArgs() map[string]any {
	res := make(map[string]any)
	for k, v := range a.OptionalArgs {
		res[k] = v
	}
	res["f"] = strings.ToLower(a.Format.String())
	res["ac"] = a.Channels
	res["ar"] = a.SampleRate
	return res
}

type AudioOutput struct {
	Output io.Writer
	// Channels is the count of audio channels (1 = mono; 2 = stereo)
	Channels int
	// SampleRate in Hz
	SampleRate int
	// Fomat the output should have.
	Format AudioFormat
	// OptionalArgs to add to already set format, channels and sample rate.
	OptionalArgs map[string]any
}

// AllArgs that will be set for ffmpeg audio output.
func (a *AudioOutput) AllArgs() map[string]any {
	res := make(map[string]any)
	for k, v := range a.OptionalArgs {
		res[k] = v
	}
	res["f"] = strings.ToLower(a.Format.String())
	res["ac"] = a.Channels
	res["ar"] = a.SampleRate
	return res
}

// Resample given input audio into the desired output audio.
func Resample(input *AudioInput, output *AudioOutput) error {
	iArgMap := input.AllArgs()
	inArgs := make([]string, len(iArgMap)*2)
	i := 0
	for k, v := range iArgMap {
		inArgs[i] = "-" + k
		i++
		inArgs[i] = stringify(v)
		i++
	}
	oArgMap := output.AllArgs()
	outArgs := make([]string, len(oArgMap)*2)
	i = 0
	for k, v := range oArgMap {
		outArgs[i] = "-" + k
		i++
		outArgs[i] = stringify(v)
		i++
	}
	allArgs := make([]string, 0)
	allArgs = append(allArgs, "-i", "pipe:")
	allArgs = append(allArgs, outArgs...)
	allArgs = append(allArgs, "pipe:")
	cmd := exec.Command("ffmpeg", allArgs...)
	fmt.Printf("running command %s", cmd)
	errBuf := new(bytes.Buffer)
	cmd.Stdin = input.Data
	cmd.Stdout = output.Output
	cmd.Stderr = errBuf
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("could not resample: %v \toutput: %s", err, errBuf.Bytes())
	}
	return nil
}

func stringify(s any) string {
	switch t := s.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", t)
	case float32, float64:
		return fmt.Sprintf("%f", t)
	case bool:
		return fmt.Sprintf("%t", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// PcmToWAV converts raw pcm data into WAV.
func PcmToWAV(pcmData []int16, sampleRate, channels int) ([]byte, error) {
	// Command to run ffmpeg
	cmd := exec.Command("ffmpeg",
		"-f", "s16le",
		"-ar", fmt.Sprintf("%d", sampleRate),
		"-ac", fmt.Sprintf("%d", channels),
		"-i", "pipe:0",
		"-f", "wav",
		"pipe:1")

	// Create buffers to hold stdin and stdout
	inBuf := bytes.NewBuffer(pcmDataToBytes(pcmData))
	outBuf := &bytes.Buffer{}

	// Set the command's stdin and stdout
	cmd.Stdin = inBuf
	cmd.Stdout = outBuf

	// Run the command
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	return outBuf.Bytes(), nil
}

// Helper function to convert PCM []int16 to []byte
func pcmDataToBytes(data []int16) []byte {
	buf := new(bytes.Buffer)
	for _, v := range data {
		buf.Write([]byte{byte(v), byte(v >> 8)})
	}
	return buf.Bytes()
}
