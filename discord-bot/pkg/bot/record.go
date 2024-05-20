package bot

import (
	"bytes"
	"encoding/gob"
	"log/slog"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
)

var recordCommand = discordgo.ApplicationCommand{
	Name:        "record-raw",
	Description: "Records raw data as send by discord and stores it as float32 pcm",
}

func recordRawHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
			Content: "Ok I'm listening.",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Emoji: &discordgo.ComponentEmoji{
								Name: "❎",
							},
							Style:    discordgo.DangerButton,
							Label:    "STOP",
							CustomID: "stop_record",
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
	componentButtons[i.GuildID]["stop_record"] = make(chan *discordgo.Interaction)

	audioPackages := make([][]byte, 0)

	for {
		var p *discordgo.Packet
		var ok bool
		select {
		case respI := <-componentButtons[i.GuildID]["stop_record"]:
			defer func() {
				// Cleanup
				close(componentButtons[i.GuildID]["stop_record"])
				delete(componentButtons[i.GuildID], "stop_record")
				slog.Info("stopped by user")
			}()
			encodedData, err := encodePackages(audioPackages)
			if err != nil {
				slog.Error("encoding gob data failed", "error", err)
				s.InteractionRespond(respI, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Failed to store recording",
					},
				})
				return
			}
			if err := os.WriteFile("recording.raw", encodedData, 0644); err != nil {
				slog.Error("failed to store recording locally", "error", err)
			}
			s.InteractionRespond(respI, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Recording finished",
					Files: []*discordgo.File{
						{
							Name:        "recording.raw",
							ContentType: "application/octet-stream",
							Reader:      bytes.NewReader(encodedData),
						},
					},
				},
			})
			return
		case p, ok = <-voiceConn.OpusRecv:
			if !ok {
				return
			}
			audioPackages = append(audioPackages, p.Opus)
		}
	}
}

func encodePackages(data [][]byte) ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	if err := enc.Encode(data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
