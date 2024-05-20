package oai

import (
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
