package bot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/MrWong99/TaileVoices/discord_bot/pkg/oai"
	"github.com/MrWong99/TaileVoices/discord_bot/pkg/stt"
	"github.com/bwmarrin/discordgo"
	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"github.com/sashabaranov/go-openai"
	"gopkg.in/hraban/opus.v2"
)

type valueSolver func([]*discordgo.ApplicationCommandInteractionDataOption) map[string]any

type advancedCommandOption struct {
	option   *discordgo.ApplicationCommandOption
	resolver valueSolver
}

var commandOptions = map[string]advancedCommandOption{
	"text": {
		option: &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "text",
			Description: "The text that should be spoken",
			Required:    true,
		},
		resolver: func(options []*discordgo.ApplicationCommandInteractionDataOption) map[string]any {
			for _, option := range options {
				if option.Name == "text" {
					return map[string]any{
						"text": option.StringValue(),
					}
				}
			}
			return make(map[string]any)
		},
	},
	"voice": {
		option: &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "voice",
			Description: "The voice that the bot should use while speaking",
			Required:    true,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{
					Name:  string(openai.VoiceAlloy),
					Value: string(openai.VoiceAlloy),
				},
				{
					Name:  string(openai.VoiceEcho),
					Value: string(openai.VoiceEcho),
				},
				{
					Name:  string(openai.VoiceFable),
					Value: string(openai.VoiceFable),
				},
				{
					Name:  string(openai.VoiceOnyx),
					Value: string(openai.VoiceOnyx),
				},
				{
					Name:  string(openai.VoiceNova),
					Value: string(openai.VoiceNova),
				},
				{
					Name:  string(openai.VoiceShimmer),
					Value: string(openai.VoiceAlloy),
				},
			},
		},
		resolver: func(options []*discordgo.ApplicationCommandInteractionDataOption) map[string]any {
			res := map[string]any{
				"voice": openai.VoiceAlloy,
			}
			for _, option := range options {
				if option.Name == "voice" {
					res["voice"] = openai.SpeechVoice(option.StringValue())
				}
			}
			return res
		},
	},
	"language": {
		option: &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "language",
			Description: "The spoken language. Can be set to 'auto' which is a bit slower though.",
			Required:    true,
		},
		resolver: func(options []*discordgo.ApplicationCommandInteractionDataOption) map[string]any {
			for _, option := range options {
				if option.Name == "language" {
					return map[string]any{
						"language": option.StringValue(),
					}
				}
			}
			return make(map[string]any)
		},
	},
}

func optionsByName(names ...string) []*discordgo.ApplicationCommandOption {
	allOptions := make([]*discordgo.ApplicationCommandOption, len(names))
	for i, name := range names {
		advOpt, ok := commandOptions[name]
		if !ok {
			panic(fmt.Errorf("option with name %q does not exist", name))
		}
		allOptions[i] = advOpt.option
	}
	return allOptions
}

func resolveAllOptions(options []*discordgo.ApplicationCommandInteractionDataOption, names ...string) map[string]any {
	res := make(map[string]any)
	for _, name := range names {
		advOpt, ok := commandOptions[name]
		if !ok {
			slog.Error("tried to resolve an option that doesn't exist", "name", name)
			continue
		}
		for k, v := range advOpt.resolver(options) {
			res[k] = v
		}
	}
	return res
}

var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "say",
		Description: "Just joins the voice channel that this command was posted in a says out your text aloud.",
		Options:     optionsByName("text", "voice"),
	},
	{
		Name:        "transcribe",
		Description: "Join the voice channel and create per-user transcriptions until stopped.",
		Options:     optionsByName("language"),
	},
}

var handlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
	"say": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		data := i.ApplicationCommandData()
		if !isVoiceChannel(s, i.ChannelID) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content: "This command can only be run in the text channel of a guild voice channel!",
				},
			})
			return
		}
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content: "I'll shout your message ASAP!",
			},
		})
		resolvedOptions := resolveAllOptions(data.Options, "text", "voice")

		slog.Info("speech request started")
		speechResp, err := oai.Client.CreateSpeech(context.Background(), openai.CreateSpeechRequest{
			Model:          openai.TTSModel1HD,
			Input:          resolvedOptions["text"].(string),
			Voice:          resolvedOptions["voice"].(openai.SpeechVoice),
			ResponseFormat: openai.SpeechResponseFormatOpus,
			Speed:          1,
		})
		if err != nil {
			slog.Error("could not generate TTS message", "error", err)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content: "There was an error generating the TTS message...",
				},
			})
			return
		}
		slog.Info("speech returned")
		opusInput, err := opus.NewStream(speechResp)
		if err != nil {
			slog.Error("could not stream opus response from OpenAI", "error", err)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content: "There was an error generating the TTS message...",
				},
			})
			return
		}
		defer opusInput.Close()

		voiceConn, err := s.ChannelVoiceJoin(i.GuildID, i.ChannelID, false, true)
		if err != nil {
			slog.Error("could not join voice channel", "error", err)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content: "There was an error joining the voice channel...",
				},
			})
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
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: &discordgo.InteractionResponseData{
						Content: "There was an error joining the voice channel...",
					},
				})
				return
			}
			time.Sleep(50 * time.Millisecond)
		}

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
	},
	"transcribe": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		data := i.ApplicationCommandData()
		if !isVoiceChannel(s, i.ChannelID) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content: "This command can only be run in the text channel of a guild voice channel!",
				},
			})
			return
		}
		resolvedOptions := resolveAllOptions(data.Options, "language")

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content: "Ok let's see what you are talking about.",
			},
		})

		transcribe, err := stt.New(resolvedOptions["language"].(string))
		if err != nil {
			slog.Error("could not create STT model", "error", err)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content: "There was an error setting up the transcriber...",
				},
			})
			return
		}

		voiceConn, err := s.ChannelVoiceJoin(i.GuildID, i.ChannelID, true, false)
		if err != nil {
			slog.Error("could not join voice channel", "error", err)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content: "There was an error joining the voice channel...",
				},
			})
			return
		}
		defer voiceConn.Disconnect()
		startTime := time.Now()
		for {
			if voiceConn.Ready && voiceConn.OpusRecv != nil {
				break
			}
			if time.Since(startTime) > 5*time.Second {
				slog.Error("voice channel won't become ready")
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: &discordgo.InteractionResponseData{
						Content: "There was an error joining the voice channel...",
					},
				})
				return
			}
			time.Sleep(50 * time.Millisecond)
		}

		decodersPerUser := make(map[uint32]*opus.Decoder)

		for {
			p, ok := <-voiceConn.OpusRecv
			if !ok {
				return
			}
			if time.Since(startTime) > 30*time.Second {
				slog.Info("stopping after 30s")
				return
			}
			decoder := decodersPerUser[p.SSRC]
			if decoder == nil {
				decoder, err = newDecoder()
				if err != nil {
					slog.Error("could not create Discord decoder", "error", err)
					return
				}
				decodersPerUser[p.SSRC] = decoder
			}
			pcmBuf := make([]float32, 7000)
			n, err := decoder.DecodeFloat32(p.Opus, pcmBuf)
			if err != nil {
				if err != nil {
					slog.Warn("could not decode some audio data", "error", err)
					continue
				}
			}
			pcmBuf = pcmBuf[:n]

			transcribe.Process(pcmBuf, func(segment whisper.Segment, processingDelay time.Duration) {
				fmt.Printf("[%s -> %s (%s delay)] %s", segment.Start, segment.End, processingDelay, segment.Text)
			})
		}
	},
}

// SetupCommands that the session will respond to.
// Returns any errors that might occur upon registration of commands.
func SetupCommands(session *discordgo.Session) error {
	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := handlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		} else {
			s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Content: "I don't know command " + i.Interaction.Message.Interaction.Name,
			})
		}
	})
	for _, command := range commands {
		if _, err := session.ApplicationCommandCreate(session.State.User.ID, "", command); err != nil {
			return fmt.Errorf("could not register command %q: %w", command.Name, err)
		}
	}
	return nil
}
