package oai

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/oov/audio/resampler"
	"github.com/sashabaranov/go-openai"
)

const (
	discordAudioSampleRate = 48000
	openaiSampleRate       = 24000
	openaiAudioChannels    = 1
)

var ToDiscordResampler *resampler.Resampler

func init() {
	ToDiscordResampler = resampler.New(openaiAudioChannels, openaiSampleRate, discordAudioSampleRate, 4)
}

var Client *openai.Client

// Init the OpenAI client using given token.
func Init(token string) {
	Client = openai.NewClient(token)
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
