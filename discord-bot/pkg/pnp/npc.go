package pnp

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"slices"
	"strings"
	"text/template"

	"github.com/MrWong99/TaileVoices/discord_bot/pkg/oai"
	"github.com/sashabaranov/go-openai"
	"gopkg.in/yaml.v3"
)

//go:embed npc_system_prompt.tpl
var npcSystemPromptText string

var npcSystemPromptTemplate *template.Template

//go:embed npc_user_prompt.tpl
var npcUserPromptText string

var npcUserPromptTemplate *template.Template

func init() {
	var err error
	// Check if the system prompt template can be resolved.
	npcSystemPromptTemplate, err = template.New("npcSystem").Parse(npcSystemPromptText)
	if err != nil {
		panic(fmt.Errorf("could not parse NPC system prompt template: %w", err))
	}
	sampleActor := Actor{
		Name:    "Test Me",
		Aliases: []string{"foo", "bar"},
		Script: `A script
		with some lines`,
	}
	err = npcSystemPromptTemplate.Execute(io.Discard, sampleActor)
	if err != nil {
		panic(fmt.Errorf("NPC system prompt template can not be executed: %w", err))
	}

	// Check if the user prompt template can be resolved
	npcUserPromptTemplate, err = template.New("npcUser").Parse(npcUserPromptText)
	if err != nil {
		panic(fmt.Errorf("could not parse NPC user prompt template: %w", err))
	}
	c := PromptContext{
		OldTranscripts: []string{"Test: Hello, World!\nGameMaster: Be quiet...", "Foo: Bar!"},
		CurrentTranscript: `User 1: Hello
		User 2: World!`,
	}
	err = npcUserPromptTemplate.Execute(io.Discard, c)
	if err != nil {
		panic(fmt.Errorf("NPC user prompt template can not be executed: %w", err))
	}
}

// Actor that plays autonomously in a voice conversation. You must use the NewActor init function or else most methods won't work properly.
type Actor struct {
	// Name of the actor. This will be used as his identity. Can be mutliple names separated by spaces.
	Name string `yaml:"name"`
	// Aliases that this actor should also react to.
	Aliases []string `yaml:"aliases"`
	// Script that the actor should follow. Should include info about his behaviour, the pen and paper world setting and all other characters he knows.
	Script string `yaml:"script"`
	// Voice that this actor should use when speaking.
	Voice           openai.SpeechVoice `yaml:"voice"`
	namesAndAliases []string
	systemPrompt    string
}

type tmpActor struct {
	Name    string             `yaml:"name"`
	Aliases []string           `yaml:"aliases"`
	Script  string             `yaml:"script"`
	Voice   openai.SpeechVoice `yaml:"voice"`
}

// UnmarshalYAML implements the unmarshalling including the required initialization.
func (a *Actor) UnmarshalYAML(value *yaml.Node) error {
	var tmp tmpActor

	if err := value.Decode(&tmp); err != nil {
		return err
	}

	a.Name = tmp.Name
	a.Aliases = tmp.Aliases
	a.Script = tmp.Script
	a.Voice = tmp.Voice
	a.init()
	return nil
}

// NewActor to integrate into a campaign.
func NewActor(name, script string, voice openai.SpeechVoice, aliases ...string) *Actor {
	a := Actor{
		Name:    name,
		Script:  script,
		Aliases: aliases,
		Voice:   voice,
	}
	a.init()
	return &a
}

func (a *Actor) init() {
	nameSplits := strings.Split(a.Name, " ")
	a.namesAndAliases = make([]string, len(nameSplits)+len(a.Aliases))
	i := 0
	for _, namePart := range nameSplits {
		cleanPart := removeNonWordRunes(namePart)
		a.namesAndAliases[i] = strings.ToLower(cleanPart)
		i++
	}
	for _, alias := range a.Aliases {
		cleanPart := removeNonWordRunes(alias)
		a.namesAndAliases[i] = strings.ToLower(cleanPart)
		i++
	}
	systemPromptBuf := bytes.NewBuffer(make([]byte, 0))
	if err := npcSystemPromptTemplate.Execute(systemPromptBuf, a); err != nil {
		// should not be possible as sanity check was done in init func
		panic(err)
	}
	a.systemPrompt = systemPromptBuf.String()
}

type PromptContext struct {
	OldTranscripts    []string
	CurrentTranscript string
}

// Act with the given prompt context. Try to keep the sum of text from all old transcripts plus the current transcript below a certain amount.
func (a *Actor) Act(ctx PromptContext) (string, error) {
	userPromptBuf := bytes.NewBuffer(make([]byte, 0))
	err := npcUserPromptTemplate.Execute(userPromptBuf, ctx)
	if err != nil {
		return "", fmt.Errorf("could not resolve user prompt template: %w", err)
	}
	resp, err := oai.Client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: openai.GPT4o,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: a.systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPromptBuf.String(),
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create chat completion: %w", err)
	}
	return resp.Choices[0].Message.Content, nil
}

// IsAdressed will return true if the actors name or any of his aliases is included in the given line of text.
func (a *Actor) IsAdressed(textLine string) bool {
	lowerLine := strings.ToLower(textLine)
	words := strings.Split(lowerLine, " ")
	for i, word := range words {
		words[i] = removeNonWordRunes(word)
	}
	return slices.ContainsFunc(a.namesAndAliases, func(name string) bool {
		return slices.Contains(words, name)
	})
}

var toRemove = []rune{'"', '!', '?', '\'', '.', ','}

func removeNonWordRunes(s string) string {
	res := ""
	for _, letter := range s {
		if !slices.Contains(toRemove, letter) {
			res += string(letter)
		}
	}
	return res
}
