package pnp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"sync"
	"text/template"

	"github.com/MrWong99/TaileVoices/discord_bot/pkg/oai"
	"github.com/MrWong99/TaileVoices/discord_bot/pkg/vecdb"
	"github.com/sashabaranov/go-openai"
	"gopkg.in/yaml.v3"

	_ "embed"
)

//go:embed summarization_prompt.txt
var summarizationPrompt string

//go:embed stt_init_prompt.tpl
var sttPromptText string

var sttPromptTemplate *template.Template

func init() {
	var err error
	// Check if the system prompt template can be resolved.
	sttPromptTemplate, err = template.New("sttInit").Parse(sttPromptText)
	if err != nil {
		panic(fmt.Errorf("could not parse STT init prompt template: %w", err))
	}
	sampleCampaign := Campaign{
		Name: "Test Me",
		Players: map[string]string{
			"test": "Me",
		},
		Actors: []*Actor{
			{
				Name: "Foo",
			},
			{
				Name: "Bar",
			},
		},
	}
	err = sttPromptTemplate.Execute(io.Discard, sampleCampaign)
	if err != nil {
		panic(fmt.Errorf("STT init prompt template can not be executed: %w", err))
	}
}

// ActorResponse that will be passed to the campaigns output chan.
type ActorResponse struct {
	// Actor that spoke.
	Actor Actor
	// Text that the actor wants to say.
	Text string
}

// Campaign wraps various actors together under one hood and manages who speaks and who doesn't.
type Campaign struct {
	// Name of this campaign. Will be used for the vector DB.
	Name string `yaml:"name"`
	// Players mapping between Discord user ID and actual ingame name.
	Players map[string]string `yaml:"players"`
	// Actors involved in the current session.
	Actors []*Actor `yaml:"actors"`
	// CurrentSessionTranscript of the running session. Will be stored in the vector DB once Close() is called.
	CurrentSessionTranscript string `yaml:"transcript"`
	transcriptMu             *sync.Mutex
	dbClient                 *vecdb.Client
	actorResponses           chan ActorResponse
}

type tmpCampaign struct {
	Name                     string            `yaml:"name"`
	Players                  map[string]string `yaml:"players"`
	Actors                   []*Actor          `yaml:"actors"`
	CurrentSessionTranscript string            `yaml:"transcript"`
}

// UnmarshalYAML implements the unmarshalling including the required initialization.
func (c *Campaign) UnmarshalYAML(value *yaml.Node) error {
	var tmpCampaign tmpCampaign

	if err := value.Decode(&tmpCampaign); err != nil {
		return err
	}

	c.Name = tmpCampaign.Name
	c.Players = tmpCampaign.Players
	c.Actors = tmpCampaign.Actors
	c.CurrentSessionTranscript = tmpCampaign.CurrentSessionTranscript
	c.dbClient = vecdb.DefaultClient()
	c.transcriptMu = &sync.Mutex{}
	c.actorResponses = make(chan ActorResponse)
	return nil
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

// CampaignFromYaml read first YAML object from data as campaign and initialize all of the actors so the campaign is ready to receive messages.
// The YAML data should be provided like this:
//
//	name: <name of the campaign>
//	transcript: |-
//	  <transcript of the current session. can be omitted>
//	players:
//	  player1discordn4me: Some Character
//	  pl4yer2: Foo Name
//	  lastpla1er: GameMaster
//	actors:
//	  - name: <name of the first actor>
//	    aliases: # can be omitted
//	      - <first alias>
//	      - <second alias>
//	      - ...
//		voice: <name of the OpenAI voice to use>
//	    script: |-
//	      <script that the first actor should follow
//	      with multiple lines indented by 2 spaces after script:>
//	  - name: <name of the second actor>
//	    ...
func CampaignFromYaml(data []byte) (*Campaign, error) {
	var c Campaign
	return &c, yaml.Unmarshal(data, &c)
}

// C is the campaigns response channel. If any actor produced a response after HandleText was called it will be returned in C.
// C is unbuffered and MUST be received. Also C will be closed once the campaign is closed.
//
// The actor responses returned in C will automatically be fed into HandleText() so the caller of C SHOULD NOT call HandleText for the responses.
func (c *Campaign) C() <-chan ActorResponse {
	return c.actorResponses
}

// PlayerName of the player with given Discord user ID as registered in the campaign.
func (c *Campaign) PlayerName(userID string) (name string, ok bool) {
	name, ok = c.Players[userID]
	return
}

// STTPrompt that should be fed to the STT context for better name recognition.
func (c *Campaign) STTPrompt() (string, error) {
	promptBuf := bytes.NewBuffer(make([]byte, 0))
	err := sttPromptTemplate.Execute(promptBuf, *c)
	return promptBuf.String(), err
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
		promptContext := PromptContext{
			CurrentTranscript: c.CurrentTranscript(),
		}
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
			Actor: *nextActor,
			Text:  result,
		}
		c.HandleText(nextActor.Name, result)
	}()
}

// Summary of the current campaign transcript. Excludes all non pen & paper related content.
// Uses a GenAI to do the summary.
//
// Can be called after Close() has been called.
func (c *Campaign) Summary() (string, error) {
	resp, err := oai.Client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: openai.GPT4o,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: summarizationPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: c.CurrentTranscript(),
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create chat completion: %w", err)
	}
	return resp.Choices[0].Message.Content, nil
}

// CurrentTranscript of this session.
func (c *Campaign) CurrentTranscript() string {
	c.transcriptMu.Lock()
	defer c.transcriptMu.Unlock()
	return c.CurrentSessionTranscript
}

// Close this campaigns session by closing C() and storing its transcript in the vector database.
func (c *Campaign) Close() error {
	close(c.actorResponses)
	c.transcriptMu.Lock()
	fmt.Printf("storing transcript for campaign %q\n%s\n", c.Name, c.CurrentSessionTranscript)
	defer c.transcriptMu.Unlock()
	return c.dbClient.StoreText(c.Name, c.CurrentSessionTranscript)
}
