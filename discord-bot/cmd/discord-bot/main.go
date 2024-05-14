package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/MrWong99/TaileVoices/discord_bot/pkg/bot"
	"github.com/MrWong99/TaileVoices/discord_bot/pkg/config"
	"github.com/MrWong99/TaileVoices/discord_bot/pkg/oai"
	"github.com/MrWong99/TaileVoices/discord_bot/pkg/stt"
	"github.com/bwmarrin/discordgo"
)

var mainCtx context.Context

func init() {
	mainCtx = context.Background()
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.ErrorContext(mainCtx, "could not open configuration", "error", err)
		os.Exit(1)
	}
	mainCtx = cfg.Agent.AddTokenToContext(mainCtx)
	oai.Init(cfg.OpenAI.Token)
	stt.SetModelPath(cfg.SpeechToText.ModelPath)
	discordSession, err := discordgo.New("Bot " + cfg.Agent.Token)
	if err != nil {
		slog.ErrorContext(mainCtx, "wrong bot params", "error", err)
		os.Exit(1)
	}
	discordSession.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		slog.InfoContext(mainCtx, "bot has been started", "username", s.State.User.Username+"#"+s.State.User.Discriminator)
	})
	if err := discordSession.Open(); err != nil {
		slog.ErrorContext(mainCtx, "could not start bot", "error", err)
		os.Exit(1)
	}
	defer discordSession.Close()

	if err := bot.SetupCommands(discordSession); err != nil {
		slog.ErrorContext(mainCtx, "could not setup commands", "error", err)
		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGABRT)

	sig := <-sigChan
	slog.InfoContext(mainCtx, "received signal to shutdown", "signal", sig.String())
}
