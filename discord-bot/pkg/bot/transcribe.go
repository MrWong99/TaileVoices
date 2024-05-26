package bot

import (
	"bytes"
	"fmt"
	"log/slog"
	"time"

	"github.com/MrWong99/TaileVoices/discord_bot/pkg/uservoice"
	"github.com/bwmarrin/discordgo"
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

	voices := make(map[uint32]*uservoice.Voice)

	var entireTranscript string
	s.VoiceConnections[voiceConn.GuildID].AddHandler(func(_ *discordgo.VoiceConnection, vs *discordgo.VoiceSpeakingUpdate) {
		voice, ok := voices[uint32(vs.SSRC)]
		if !ok || voice.Username != "" {
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
		voice.Username = name
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
				Type: discordgo.InteractionResponseUpdateMessage,
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
		}
		voice, ok := voices[p.SSRC]
		if !ok {
			voice, err = uservoice.NewVoice("", p.SSRC, resolvedOptions["language"].(string))
			if err != nil {
				slog.Error("could not create voice receiver", "SSRC", p.SSRC, "error", err)
				continue
			}
			voices[p.SSRC] = voice
			defer voice.Close()
			go handleAudio(voice, &entireTranscript)
		}
		if err := voice.Process(p.Opus); err != nil {
			slog.Error("could not process audio data", "SSRC", p.SSRC, "error", err)
		}
	}
}

func handleAudio(voice *uservoice.Voice, transcript *string) {
	for segment := range voice.C() {
		s := fmt.Sprintf("[%s -> %s] %s: %s\n", segment.Start, segment.End, voice.Username, segment.Text)
		fmt.Println(s)
		*transcript += s
	}
}
