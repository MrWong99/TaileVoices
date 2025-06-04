package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strings"

	"golang.org/x/exp/constraints"
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
	// NoArgs can force the channel. sample rate and any other arguments not to be set for ffmpeg command.
	NoArgs bool
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
	if !input.NoArgs {
		allArgs = append(allArgs, "-f", strings.ToLower(input.Format.String()))
	}
	if (input.Format == F32le || input.Format == S16le) && !input.NoArgs {
		allArgs = append(allArgs, "-ac", stringify(input.Channels), "-ar", stringify(input.SampleRate))
	}
	allArgs = append(allArgs, "-i", "pipe:")
	allArgs = append(allArgs, outArgs...)
	allArgs = append(allArgs, "pipe:")
	cmd := exec.Command("ffmpeg", allArgs...)
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

// ReadBytes converts given bytes from reader to a slice of the defined number type in little endianess.
func ReadBytes[T constraints.Integer | constraints.Float](reader io.Reader) ([]T, error) {
	var result []T
	for {
		var num T
		err := binary.Read(reader, binary.LittleEndian, &num)
		if err != nil {
			if err == io.EOF {
				break
			}
			return result, err
		}
		result = append(result, num)
	}
	return result, nil
}

// ConvertStereoToMono converts stereo PCM data to mono by averaging the channels.
func ConvertStereoToMono(stereo []float32) []float32 {
	mono := make([]float32, len(stereo)/2)
	for i := 0; i < len(stereo); i += 2 {
		mono[i/2] = (stereo[i] + stereo[i+1]) / 2
	}
	return mono
}

// ResamplePCM resamples the PCM data from srcRate to dstRate using linear interpolation.
func ResamplePCM(data []float32, srcRate, dstRate int) []float32 {
	ratio := float64(srcRate) / float64(dstRate)
	dstLen := int(math.Ceil(float64(len(data)) / ratio))
	resampled := make([]float32, dstLen)

	for i := range resampled {
		srcIndex := float64(i) * ratio
		intPart := int(srcIndex)
		fracPart := srcIndex - float64(intPart)

		if intPart+1 < len(data) {
			resampled[i] = float32(float64(data[intPart])*(1-fracPart) + float64(data[intPart+1])*fracPart)
		} else {
			resampled[i] = data[intPart]
		}
	}

	return resampled
}
