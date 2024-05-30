package bot

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/MrWong99/TaileVoices/discord_bot/pkg/oai"
	"github.com/bwmarrin/discordgo"
	"github.com/sashabaranov/go-openai"
	"gopkg.in/hraban/opus.v2"
)

var sayCommand = discordgo.ApplicationCommand{
	Name:        "say",
	Description: "Just joins the voice channel that this command was posted in a says out your text aloud.",
	Options:     optionsByName("text", "voice"),
}

func sayHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	if !isVoiceChannel(s, i.ChannelID) {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "This command can only be run in the text channel of a guild voice channel!",
			},
		})
		if err != nil {
			slog.Warn("could not create interaction response", "error", err)
		}
		return
	}
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "I'll shout your message ASAP!",
		},
	})
	if err != nil {
		slog.Warn("could not create interaction response", "error", err)
	}
	resolvedOptions := resolveAllOptions(data.Options, "text", "voice")

	opusInput, err := createAudioResponse(resolvedOptions["text"].(string), resolvedOptions["voice"].(openai.SpeechVoice))
	if err != nil {
		slog.Error("could not stream opus response from OpenAI", "error", err)
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "There was an error generating the TTS message...",
			},
		})
		if err != nil {
			slog.Warn("could not create interaction response", "error", err)
		}
		return
	}
	defer opusInput.Close()

	voiceConn, err := s.ChannelVoiceJoin(i.GuildID, i.ChannelID, false, true)
	if err != nil {
		slog.Error("could not join voice channel", "error", err)
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "There was an error joining the voice channel...",
			},
		})
		if err != nil {
			slog.Warn("could not create interaction response", "error", err)
		}
		return
	}
	defer voiceConn.Disconnect()
	startTime := time.Now()
	for {
		if voiceConn.Ready && voiceConn.OpusSend != nil {
			break
		}
		if time.Since(startTime) > 5*time.Second {
			slog.Error("voice channel won't become ready")
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "There was an error joining the voice channel...",
				},
			})
			if err != nil {
				slog.Warn("could not create interaction response", "error", err)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	speakAudio(voiceConn, opusInput)
}

func createAudioResponse(text string, voice openai.SpeechVoice) (*opus.Stream, error) {
	slog.Info("speech request started")
	speechResp, err := oai.Client.CreateSpeech(context.Background(), openai.CreateSpeechRequest{
		Model:          openai.TTSModel1HD,
		Input:          text,
		Voice:          voice,
		ResponseFormat: openai.SpeechResponseFormatOpus,
		Speed:          1,
	})
	if err != nil {
		slog.Error("could not generate TTS message", "error", err)
		return nil, err
	}
	slog.Info("speech returned")
	return opus.NewStream(speechResp)
}

func speakAudio(voiceConn *discordgo.VoiceConnection, opusInput *opus.Stream) {
	voiceConn.Speaking(true)
	defer voiceConn.Speaking(false)
outer:
	for {
		// First we read the next streamed response from the http client.
		// There can be multiple chunks as the response is chunk transfer encoded
		pcmBuf := make([]float32, 700000)
		n, err := opusInput.ReadFloat32(pcmBuf)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				slog.Error("could not decode OpenAI speech response with opus", "error", err)
			}
			break
		}
		pcmBuf = pcmBuf[:n]
		resampleBuf := make([]float32, len(pcmBuf)*5)
		_, n = oai.ToDiscordResampler.ProcessFloat32(0, pcmBuf, resampleBuf)
		resampleBuf = resampleBuf[:n]
		encodedPackages := make([][]byte, 0)
		for i := 0; i < len(resampleBuf); i += discordPcmLength {
			onePackage := make([]float32, discordPcmLength)
			copy(onePackage, resampleBuf[i:min(i+discordPcmLength, len(resampleBuf))])
			buf := make([]byte, 300000)
			n, err = discordEncoder.EncodeFloat32(onePackage, buf)
			if err != nil {
				slog.Error("could not encode opus data to send to Discord", "error", err)
				break outer
			}
			encodedPackages = append(encodedPackages, buf[:n])
		}
		for _, pkg := range encodedPackages {
			voiceConn.OpusSend <- pkg
		}
	}
}
