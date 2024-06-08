package bot

import (
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/sashabaranov/go-openai"
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
	"campaign": {
		option: &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "campaign",
			Description: "The name of your pen & paper campaign. This determines the DB.",
			Required:    true,
		},
		resolver: func(options []*discordgo.ApplicationCommandInteractionDataOption) map[string]any {
			for _, option := range options {
				if option.Name == "campaign" {
					return map[string]any{
						"campaign": option.StringValue(),
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
