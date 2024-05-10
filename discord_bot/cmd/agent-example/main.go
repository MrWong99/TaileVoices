package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/MrWong99/TaileVoices/discord_bot/pkg/bot"
	"github.com/MrWong99/TaileVoices/discord_bot/pkg/config"
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
	mainCtx = cfg.AddTokenToContext(mainCtx)

	bot := bot.New(mainCtx, cfg.Token)
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
