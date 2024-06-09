package pnp

import (
	"bytes"
	"fmt"
	"testing"
)

func TestActorSystemPromptTemplate(t *testing.T) {
	tests := make(map[*Actor]string)

	fillTest := func(tests map[*Actor]string, name, script string, aliases ...string) {
		tests[NewActor(name, script, "", aliases...)] = systemTemplateByHand(name, script, aliases...)
	}
	fillTest(tests, "Swan 'Roglu' Chez", "The king\nis here!", "Naaz", "Peter")
	fillTest(tests, "Mia", "")
	fillTest(tests, "cat", "sings all night", "meow")

	for actor, prompt := range tests {
		t.Run("actor "+actor.Name, func(t *testing.T) {
			if actor.systemPrompt != prompt {
				t.Errorf("expected actor %s system prompt to be:\n%s\n\nbut got:\n%s", actor.Name, prompt, actor.systemPrompt)
			}
		})
	}
}

func systemTemplateByHand(name, script string, aliases ...string) string {
	aliasText := ""
	if len(aliases) > 0 {
		aliasText = "You are also known by these aliases:\n"
	}
	for _, alias := range aliases {
		aliasText += "- " + alias + "\n"
	}
	return `You are ` + name + ` a NPC in a pen and paper campaign.
` + aliasText + `

You will perceive the world thorugh two sources: first a possibly empty list of older transcripts and second a transcript of the current pen and paper session.
The transcripts are not perfect so try to deduce some context or fix the spelling or language if needed.

The transcripts will be provided by the user in the following format delimited by """:
"""
- OLD TRANSCRIPTS -
1:
Name: text line
Other Name: text line
...

2:
...


- CURRENT TRANSCRIPT -
Name: text line
Other name: text line
...
"""

Your answers should be responses in natural language that fit into the end of the current transcript.
Keep your answers short unless the following script tells you otherwise.

This is your script that you must follow at all times unless any of the transcripts suggest a different approach:
` + script
}

func TestActorBeingAddressed(t *testing.T) {
	a := NewActor("Swan 'Roglu' Chez", "", "", "Naaz", "Peter")
	isAddressed := []string{
		"RoGlu how are you?", "What's naaz?", "Yooooo peter", "I could eat a Swan", "cheating chez", "santa chez my friend!",
	}
	notAddressed := []string{
		"an Roglight is good", "glueing my face", "Naz what?", "",
	}
	for _, line := range isAddressed {
		t.Run("positive for "+line, func(t *testing.T) {
			if !a.IsAdressed(line) {
				t.Errorf("actor %s with aliases %v should be addressed in line: %s", a.Name, a.Aliases, line)
			}
		})
	}
	for _, line := range notAddressed {
		t.Run("negative for "+line, func(t *testing.T) {
			if a.IsAdressed(line) {
				t.Errorf("actor %s with aliases %v should NOT be addressed in line: %s", a.Name, a.Aliases, line)
			}
		})
	}
}

func TestNpcUserTemplate(t *testing.T) {
	tests := make(map[*PromptContext]string)

	fillTest := func(tests map[*PromptContext]string, currentTranscript string, oldTranscripts ...string) {
		tests[&PromptContext{OldTranscripts: oldTranscripts, CurrentTranscript: currentTranscript}] = userTemplateByHand(currentTranscript, oldTranscripts...)
	}
	fillTest(tests, "Swan 'Roglu' Chez", "The king\nis here!", "Naaz", "Peter")
	fillTest(tests, "Mia", "")
	fillTest(tests, "Chez")
	fillTest(tests, "cat", "sings all night", "meow")

	for promptContext, resultPrompt := range tests {
		t.Run("user prompt "+promptContext.CurrentTranscript, func(t *testing.T) {
			promptBuf := bytes.NewBuffer(make([]byte, 0))
			if err := npcUserPromptTemplate.Execute(promptBuf, *promptContext); err != nil {
				t.Fatalf("got unexpected error while resolving user prompt template: %v", err)
			}
			actualPrompt := promptBuf.String()
			if actualPrompt != resultPrompt {
				t.Errorf("expected user prompt to be:\n%s\n\nbut got:\n%s", resultPrompt, actualPrompt)
			}
		})
	}
}

func userTemplateByHand(currentTranscript string, oldTranscripts ...string) string {
	oldTranscriptText := ""
	for i, t := range oldTranscripts {
		oldTranscriptText += fmt.Sprintf("%d:\n%s\n", i, t)
	}
	return `"""
- OLD TRANSCRIPTS -
` + oldTranscriptText + `

- CURRENT TRANSCRIPT -
` + currentTranscript + `
"""`
}
