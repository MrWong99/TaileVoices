package bot

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// Maps various component button actions to their pressed channels.
// Mapping goes Interaction.GuildID -> Component.CustomID
var componentButtons = make(map[string]map[string]chan *discordgo.Interaction)

var commands = []*discordgo.ApplicationCommand{&sayCommand, &transcribeCommand, &recordRawCommand}

var handlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
	sayCommand.Name:        sayHandler,
	transcribeCommand.Name: transcribeHandler,
	recordRawCommand.Name:  recordRawHandler,
}

// SetupCommands that the session will respond to.
// Returns any errors that might occur upon registration of commands.
func SetupCommands(session *discordgo.Session) error {
	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionMessageComponent {
			_, ok := componentButtons[i.GuildID]
			if !ok {
				return
			}
			if c, ok := componentButtons[i.GuildID][i.MessageComponentData().CustomID]; ok {
				go func() {
					c <- i.Interaction
				}()
			}
			return
		}
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}
		if h, ok := handlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		} else {
			s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Content: "I don't know command " + i.Interaction.Message.Interaction.Name,
			})
		}
	})
	if _, err := session.ApplicationCommandBulkOverwrite(session.State.User.ID, "", commands); err != nil {
		return fmt.Errorf("could not register commands %v: %w", commands, err)
	}
	return nil
}
