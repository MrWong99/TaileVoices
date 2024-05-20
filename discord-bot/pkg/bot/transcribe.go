package bot

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log/slog"
	"time"

	"github.com/MrWong99/TaileVoices/discord_bot/pkg/audio"
	"github.com/bwmarrin/discordgo"
	whisper "github.com/ggerganov/whisper.cpp/bindings/go"
)

var transcribeCommand = discordgo.ApplicationCommand{
	Name:        "transcribe",
	Description: "Join the voice channel and create per-user transcriptions until stopped.",
	Options:     optionsByName("language"),
}

func transcribeHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
	resolvedOptions := resolveAllOptions(data.Options, "language")

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Ok let's see what you are talking about.",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Emoji: &discordgo.ComponentEmoji{
								Name: "❎",
							},
							Style:    discordgo.DangerButton,
							Label:    "STOP",
							CustomID: "stop_transcript",
						},
					},
				},
			},
		},
	})
	if err != nil {
		slog.Error("could not create interaction response", "error", err)
		return
	}

	voiceConn, err := s.ChannelVoiceJoin(i.GuildID, i.ChannelID, true, false)
	if err != nil {
		msg := "There was an error joining the voice channel..."
		slog.Error("could not join voice channel", "error", err)
		_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
		})
		if err != nil {
			slog.Warn("could not create interaction response", "error", err)
		}
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
			msg := "There was an error joining the voice channel..."
			_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			})
			if err != nil {
				slog.Warn("could not create interaction response", "error", err)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	if _, ok := componentButtons[i.GuildID]; !ok {
		componentButtons[i.GuildID] = make(map[string]chan *discordgo.Interaction)
	}
	componentButtons[i.GuildID]["stop_transcript"] = make(chan *discordgo.Interaction)

	buffersPerUser := make(map[uint32]*bytes.Buffer)
	usernamesPerSSRC := make(map[uint32]string)
	previousUserText := make(map[uint32]string)
	defer func() {
		clear(buffersPerUser)
		clear(usernamesPerSSRC)
		clear(previousUserText)
	}()
	var followUpId string
	var entireTranscript string
	s.VoiceConnections[voiceConn.GuildID].AddHandler(func(_ *discordgo.VoiceConnection, vs *discordgo.VoiceSpeakingUpdate) {
		if _, ok := usernamesPerSSRC[uint32(vs.SSRC)]; ok {
			return
		}
		var name string
		user, err := s.User(vs.UserID)
		if err != nil {
			slog.Warn("could not get username", "userID", vs.UserID, "error", err)
			name = vs.UserID
		} else {
			name = user.Username
		}
		usernamesPerSSRC[uint32(vs.SSRC)] = name
	})

	for {
		var p *discordgo.Packet
		var ok bool
		select {
		case respI := <-componentButtons[i.GuildID]["stop_transcript"]:
			defer func() {
				// Cleanup
				close(componentButtons[i.GuildID]["stop_transcript"])
				delete(componentButtons[i.GuildID], "stop_transcript")
				slog.Info("stopped by user")
			}()
			s.InteractionRespond(respI, &discordgo.InteractionResponse{
				Type: discordgo.InteractionApplicationCommandAutocompleteResult,
				Data: &discordgo.InteractionResponseData{
					Content: "Transcript finished",
					Files: []*discordgo.File{
						{
							Name:        "transcript.txt",
							ContentType: "text/plain",
							Reader:      bytes.NewReader([]byte(entireTranscript)),
						},
					},
				},
			})
			return
		case p, ok = <-voiceConn.OpusRecv:
			if !ok {
				return
			}
			pcmBuf := make([]float32, 70000)
			n, err := discordDecoder.DecodeFloat32(p.Opus, pcmBuf)
			if err != nil {
				slog.Warn("could not decode opus voice data", "error", err)
				continue
			}
			pcmBuf = pcmBuf[:n]
			buf := new(bytes.Buffer)
			for i := 0; i < len(pcmBuf); i++ {
				binary.Write(buf, binary.LittleEndian, pcmBuf[i])
			}
			resBuf, ok := buffersPerUser[p.SSRC]
			if !ok {
				resBuf = new(bytes.Buffer)
				buffersPerUser[p.SSRC] = resBuf
			}
			err = audio.Resample(&audio.AudioInput{
				Data:       buf,
				Channels:   discordAudioChannels,
				SampleRate: discordAudioSampleRate,
				Format:     audio.F32le,
			}, &audio.AudioOutput{
				Output:     resBuf,
				Channels:   1,
				SampleRate: whisper.SampleRate,
				Format:     audio.F32le,
			})
			if err != nil {
				slog.Warn("could not resample package", "error", err)
				continue
			}
		}
		userBuf, ok := buffersPerUser[p.SSRC]
		if ok && len(userBuf.Bytes()) >= sampleDataSize {
			var lang string
			l, ok := resolvedOptions["language"]
			language := l.(string)
			if ok && language != "auto" {
				lang = language
			}
			transcriber, err := audio.NewSTT(lang)
			if err != nil {
				slog.Error("could not initialize transcriber context: %v", err)
				return
			}
			audio, err := audio.ReadBytes[float32](userBuf)
			if err != nil {
				slog.Error("could not read audio buffer: %v", err)
				return
			}
			delete(buffersPerUser, p.SSRC)
			segments, err := transcriber.Transcribe(audio)
			if err != nil {
				slog.Warn("could not trascribe audio sample", "error", err)
				continue
			}
			delete(buffersPerUser, p.SSRC)
			oldText := previousUserText[p.SSRC]
			for _, segment := range segments {
				oldText += ". " + segment.Text
				entireTranscript += fmt.Sprintf("%s: %s\n", usernamesPerSSRC[p.SSRC], segment.Text)
			}
			previousUserText[p.SSRC] = oldText
			if entireTranscript == "" {
				continue
			}
			go func() {
				if followUpId == "" {
					msg, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
						Content: entireTranscript,
					})
					if err != nil {
						slog.Error("could not create interaction response with stop button", "error", err)
						return
					}
					followUpId = msg.ID
					return
				}

				// Follow up exists, so edit it
				s.FollowupMessageEdit(i.Interaction, followUpId, &discordgo.WebhookEdit{
					Content: &entireTranscript,
				})
			}()
		}
	}
}
