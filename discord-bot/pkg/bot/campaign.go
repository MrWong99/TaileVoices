package bot

import (
	"bytes"
	"log/slog"
	"os"
	"time"

	"github.com/MrWong99/TaileVoices/discord_bot/pkg/pnp"
	"github.com/MrWong99/TaileVoices/discord_bot/pkg/uservoice"
	"github.com/bwmarrin/discordgo"
)

var campaignCommand = discordgo.ApplicationCommand{
	Name:        "campaign",
	Description: "Join the voice channel and manage the campaign with given name.",
	Options:     optionsByName("campaign", "language"),
}

func campaignHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
	resolvedOptions := resolveAllOptions(data.Options, "campaign", "language")

	campaignData, err := os.ReadFile(resolvedOptions["campaign"].(string) + "-campaign.yml")
	if err != nil {
		slog.Warn("could not find campaign data", "campaign", resolvedOptions["campaign"], "error", err)
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "No configuration found for this campaign",
			},
		})
		if err != nil {
			slog.Warn("could not create interaction response", "error", err)
		}
		return
	}
	campaign, err := pnp.CampaignFromYaml(campaignData)
	if err != nil {
		slog.Warn("could not read campaign data", "campaign", resolvedOptions["campaign"], "error", err)
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "No valid configuration found for this campaign",
			},
		})
		if err != nil {
			slog.Warn("could not create interaction response", "error", err)
		}
		return
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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
							CustomID: "stop_campaign",
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

	voiceConn, err := s.ChannelVoiceJoin(i.GuildID, i.ChannelID, false, false)
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

	go handleCampaignAudioOutput(campaign, voiceConn)

	if _, ok := componentButtons[i.GuildID]; !ok {
		componentButtons[i.GuildID] = make(map[string]chan *discordgo.Interaction)
	}
	componentButtons[i.GuildID]["stop_campaign"] = make(chan *discordgo.Interaction)

	voices := make(map[uint32]*uservoice.Voice)
	names := make(map[uint32]string)

	s.VoiceConnections[voiceConn.GuildID].AddHandler(func(_ *discordgo.VoiceConnection, vs *discordgo.VoiceSpeakingUpdate) {
		_, ok := names[uint32(vs.SSRC)]
		if ok {
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
		names[uint32(vs.SSRC)] = name
		voice, ok := voices[uint32(vs.SSRC)]
		if !ok {
			return
		}
		voice.Username = name
	})

	for {
		var p *discordgo.Packet
		var ok bool
		select {
		case respI := <-componentButtons[i.GuildID]["stop_campaign"]:
			defer func() {
				if err := campaign.Close(); err != nil {
					slog.Warn("unexpected error while stopping campaign", "campaign", campaign.Name, "error", err)
				}
				// Cleanup
				close(componentButtons[i.GuildID]["stop_campaign"])
				delete(componentButtons[i.GuildID], "stop_campaign")
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
							Reader:      bytes.NewReader([]byte(campaign.CurrentSessionTranscript)),
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
			name := names[p.SSRC]
			voice, err = uservoice.NewVoice(name, p.SSRC, resolvedOptions["language"].(string))
			if err != nil {
				slog.Error("could not create voice receiver", "SSRC", p.SSRC, "error", err)
				continue
			}
			voices[p.SSRC] = voice
			defer voice.Close()
			go handleCampaignAudioInput(voice, campaign)
		}
		if err := voice.Process(p.Opus); err != nil {
			slog.Error("could not process audio data", "SSRC", p.SSRC, "error", err)
		}
	}
}

func handleCampaignAudioInput(voice *uservoice.Voice, campaign *pnp.Campaign) {
	for segment := range voice.C() {
		campaign.HandleText(voice.Username, segment.Text)
	}
}

func handleCampaignAudioOutput(campaign *pnp.Campaign, voiceConn *discordgo.VoiceConnection) {
	for response := range campaign.C() {
		o, err := createAudioResponse(response.Text, response.Actor.Voice)
		if err != nil {
			slog.Error("failed to create audio response", "error", err)
			return
		}

		speakAudio(voiceConn, o)
	}
}
