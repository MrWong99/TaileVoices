package pnp

import (
	"fmt"
	"log/slog"
	"math/rand"
	"sync"

	"github.com/MrWong99/TaileVoices/discord_bot/pkg/vecdb"
)

// ActorResponse that will be passed to the campaigns output chan.
type ActorResponse struct {
	// Name of the actor that spoke.
	Name string
	// Text that the actor wants to say.
	Text string
}

// Campaign wraps various actors together under one hood and manages who speaks and who doesn't.
type Campaign struct {
	// Name of this campaign. Will be used for the vector DB.
	Name string
	// Actors involved in the current session.
	Actors []*Actor
	// CurrentSessionTranscript of the running session. Will be stored in the vector DB once Close() is called.
	CurrentSessionTranscript string
	transcriptMu             *sync.Mutex
	dbClient                 *vecdb.Client
	actorResponses           chan ActorResponse
}

// NewCampaign or just new session of an existing campaign. Call Close() to store the transcript.
func NewCampaign(name string, actors []*Actor, dbClient *vecdb.Client) *Campaign {
	return &Campaign{
		Name:                     name,
		Actors:                   actors,
		CurrentSessionTranscript: "",
		transcriptMu:             &sync.Mutex{},
		dbClient:                 dbClient,
		actorResponses:           make(chan ActorResponse),
	}
}

// C is the campaigns response channel. If any actor produced a response after HandleText was called it will be returned in C.
// C is unbuffered and MUST be received.
//
// The actor responses returned in C will automatically be fed into HandleText() so the caller of C SHOULD NOT call HandleText for the responses.
func (c *Campaign) C() chan<- ActorResponse {
	return c.actorResponses
}

// HandleText spoken by a person or NPC actor.
func (c *Campaign) HandleText(name string, segment string) {
	line := fmt.Sprintf("%s: %s", name, segment)
	c.transcriptMu.Lock()
	if c.CurrentSessionTranscript == "" {
		c.CurrentSessionTranscript = line
	} else {
		c.CurrentSessionTranscript += "\n" + line
	}
	c.transcriptMu.Unlock()

	involvedActors := make([]*Actor, 0)
	for _, actor := range c.Actors {
		// Actors should not respond to themselfes aswell as they must be included in the text.
		if actor.Name == name || !actor.IsAdressed(segment) {
			continue
		}
		involvedActors = append(involvedActors, actor)
	}
	var nextActor *Actor
	switch len(involvedActors) {
	case 0:
		return // No one involved, just update transcript
	case 1:
		nextActor = involvedActors[0]
	default:
		// Multiple actors involved. Roll one randomly to be the next
		nextActor = involvedActors[rand.Intn(len(involvedActors))]
	}

	go func() {
		c.transcriptMu.Lock()
		promptContext := PromptContext{
			CurrentTranscript: c.CurrentSessionTranscript,
		}
		c.transcriptMu.Unlock() // don't defer as it will be a deadlock once c.HandleText() is called
		oldTranscripts, err := c.dbClient.SearchTranscripts(c.Name, segment)
		if err != nil {
			slog.Warn("could not search old transcripts for reference", "error", err, "collection", c.Name, "concept", segment)
		}
		promptContext.OldTranscripts = oldTranscripts

		result, err := nextActor.Act(promptContext)
		if err != nil {
			slog.Error("actor had an error while responding", "name", nextActor.Name, "error", err)
			result = "Sorry I wanted to say something but my brain just broke... Don't count on me right now!"
		}
		c.actorResponses <- ActorResponse{
			Name: nextActor.Name,
			Text: result,
		}
		c.HandleText(nextActor.Name, result)
	}()
}

// Close this campaigns session by closing C() and storing its transcript in the vector database.
func (c *Campaign) Close() error {
	close(c.actorResponses)
	return c.dbClient.StoreText(c.Name, c.CurrentSessionTranscript)
}
