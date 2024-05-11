package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Goscord/goscord/goscord/discord"
	"github.com/Goscord/goscord/goscord/gateway"
	"github.com/Goscord/goscord/goscord/gateway/event"
	"github.com/MrWong99/TaileVoices/discord_bot/pkg/bot"
	"github.com/MrWong99/TaileVoices/discord_bot/pkg/config"
	"github.com/oov/audio/resampler"
	"github.com/sashabaranov/go-openai"
	"gopkg.in/hraban/opus.v2"
)

const (
	discordAudioSampleRate  = 48000
	discordAudioChannels    = 2
	discordAudioFrameSize   = 960
	discordPcmLength        = discordAudioFrameSize * discordAudioChannels
	discordAudioFrameSizeMs = 20
	openaiSampleRate        = 24000
	openaiAudioChannels     = 1
)

var mainCtx context.Context

var aiclient *openai.Client
var discordEncoder *opus.Encoder
var resample *resampler.Resampler

func init() {
	mainCtx = context.Background()
	var err error
	discordEncoder, err = opus.NewEncoder(discordAudioSampleRate, discordAudioChannels, opus.AppVoIP)
	if err != nil {
		slog.ErrorContext(mainCtx, "could not create opus encoder for Discord requests", "error", err)
		os.Exit(1)
	}
	discordEncoder.SetBitrateToAuto()
	discordEncoder.SetMaxBandwidth(opus.Fullband)
	resample = resampler.New(openaiAudioChannels, openaiSampleRate, discordAudioSampleRate, 4)
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.ErrorContext(mainCtx, "could not open configuration", "error", err)
		os.Exit(1)
	}
	mainCtx = cfg.Agent.AddTokenToContext(mainCtx)
	aiclient = openai.NewClient(cfg.OpenAI.Token)
	bot := bot.New(mainCtx, cfg.Agent.Token)
	setupCallbacks(bot.Client)
	if err := bot.Start(); err != nil {
		slog.ErrorContext(mainCtx, "could not login bot", "error", err)
		os.Exit(1)
	}
	defer bot.Close()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGABRT)

	sig := <-sigChan
	slog.InfoContext(mainCtx, "received signal to shutdown", "signal", sig.String())
}

func setupCallbacks(client *gateway.Session) {
	client.On(event.EventReady, func() {
		slog.InfoContext(mainCtx, "connected to Discord", "user", client.Me().Tag())

		_, err := client.Application.RegisterCommand(client.Me().Id, "", &discord.ApplicationCommand{
			Name:        "say",
			Type:        discord.ApplicationCommandChat,
			Description: "Say what you want",
			Options: []*discord.ApplicationCommandOption{
				{
					Type:        discord.ApplicationCommandOptionString,
					Name:        "text",
					Description: "The voice line to say",
					Required:    true,
				},
			},
		})
		if err != nil {
			slog.ErrorContext(mainCtx, "could not register command", "error", err)
		}
	})

	client.On(event.EventInteractionCreate, func(interaction *discord.Interaction) {
		if interaction.Member == nil {
			return
		}

		if interaction.ApplicationCommandData().Name != "say" {
			return
		}
		client.Interaction.CreateResponse(interaction.Id, interaction.Token, "I am on it")
		conn, err := client.JoinVoiceChannel(interaction.GuildId, interaction.ChannelId, false, false)
		if err != nil {
			slog.ErrorContext(mainCtx, "could not connect to voice channel", "error", err)
			return
		}
		defer conn.Disconnect()
		for {
			if conn.Ready() {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		slog.InfoContext(mainCtx, "speech request started")
		speechResp, err := aiclient.CreateSpeech(mainCtx, openai.CreateSpeechRequest{
			Model:          openai.TTSModel1HD,
			Input:          interaction.ApplicationCommandData().Options[0].Value.(string),
			Voice:          openai.VoiceAlloy,
			ResponseFormat: openai.SpeechResponseFormatOpus,
			Speed:          1,
		})
		if err != nil {
			slog.ErrorContext(mainCtx, "could not generate TTS message", "error", err)
			return
		}
		slog.InfoContext(mainCtx, "speech returned")
		opusInput, err := opus.NewStream(speechResp)
		if err != nil {
			//speechResp.Close()
			slog.ErrorContext(mainCtx, "could not stream opus response from OpenAI", "error", err)
			return
		}
		defer opusInput.Close()
		slog.InfoContext(mainCtx, "starting to speak")
		conn.Speaking(true)
		defer conn.Speaking(false)
	outer:
		for {
			// First we read the next streamed response from the http client.
			// There can be multiple chunks as the response is chunk transfer encoded
			pcmBuf := make([]float32, 700000)
			n, err := opusInput.ReadFloat32(pcmBuf)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					slog.ErrorContext(mainCtx, "could not decode OpenAI speech response with opus", "error", err)
				}
				break
			}
			pcmBuf = pcmBuf[:n]
			resampleBuf := make([]float32, len(pcmBuf)*5)
			_, n = resample.ProcessFloat32(0, pcmBuf, resampleBuf)
			resampleBuf = resampleBuf[:n]
			encodedPackages := make([][]byte, 0)
			for i := 0; i < len(resampleBuf); i += discordPcmLength {
				onePackage := make([]float32, discordPcmLength)
				copy(onePackage, resampleBuf[i:min(i+discordPcmLength, len(resampleBuf))])
				buf := make([]byte, 300000)
				n, err = discordEncoder.EncodeFloat32(onePackage, buf)
				if err != nil {
					slog.ErrorContext(mainCtx, "could not encode opus data to send to Discord", "error", err)
					break outer
				}
				encodedPackages = append(encodedPackages, buf[:n])
			}
			for _, pkg := range encodedPackages {
				_, err := conn.Write(pkg)
				if err != nil {
					slog.ErrorContext(mainCtx, "could not send audio data to discord", "error", err)
					break outer
				}
			}
		}
		slog.InfoContext(mainCtx, "speech stopped")
	})
}

func resampleAndStereo(monoSamples []float32) []float32 {
	bufChan1 := make([]float32, len(monoSamples)*5)
	_, n := resample.ProcessFloat32(0, monoSamples, bufChan1)
	return bufChan1[:n]
	/*
		bufChan1 = bufChan1[:n]
		bufChan2 := make([]float32, len(monoSamples)*5)
		_, n = resample.ProcessFloat32(1, monoSamples, bufChan2)
		bufChan2 = bufChan2[:n]
		if len(bufChan1) != len(bufChan2) {
			slog.ErrorContext(mainCtx, "resampling mono to stereo produced wrong results")
			return nil
		}
		stereoSamples := make([]float32, len(bufChan1)*2)
		for i := 0; i < len(stereoSamples); i++ {
			if i%2 == 0 {
				stereoSamples[i] = bufChan1[i/2]
			} else {
				stereoSamples[i] = bufChan2[i/2]
			}
		}
		return stereoSamples
	*/
}
