package bot

import (
	"context"
	"errors"

	"github.com/Goscord/goscord/goscord"
	"github.com/Goscord/goscord/goscord/gateway"
)

// Bot to start that uses discord.
type Bot struct {
	ctx      context.Context
	cancelFn context.CancelFunc
	closed   bool

	// Client gives direct access to the Goscord client.
	Client *gateway.Session
}

// New returns a new Bot to use.
func New(ctx context.Context, token string) *Bot {
	c, cancel := context.WithCancel(ctx)
	return &Bot{
		ctx:      c,
		cancelFn: cancel,
		Client: goscord.New(&gateway.Options{
			Token:   token,
			Intents: gateway.IntentsAll,
		}),
	}
}

// Start the bot. This is not blocking.
func (b *Bot) Start() error {
	if b.closed {
		return errors.New("bot has been closed and can't be started again")
	}
	go func() {
		<-b.ctx.Done()
		b.closed = true
		b.Client.Close()
	}()
	return b.Client.Login()
}

// Stop the bot and close its session. You can't call Start() again afterwards.
func (b *Bot) Close() {
	// I know b.closed = true will be called in the closing goroutine aswell,
	// but as it is async this might fail to prevent Start() from being called again if the timing is just bad.
	b.closed = true
	b.cancelFn()
}
