module github.com/MrWong99/TaileVoices/discord_bot

go 1.22.3

replace github.com/ggerganov/whisper.cpp/bindings/go => ./third_party/whisper.cpp/bindings/go

require (
	github.com/bwmarrin/discordgo v0.28.1
	github.com/ggerganov/whisper.cpp/bindings/go v0.0.0-20240513123346-4ef8d9f44eb4
	github.com/oov/audio v0.0.0-20171004131523-88a2be6dbe38
	github.com/sashabaranov/go-openai v1.23.1
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842
	gopkg.in/hraban/opus.v2 v2.0.0-20230925203106-0188a62cb302
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/gorilla/websocket v1.5.0 // indirect
	golang.org/x/crypto v0.17.0 // indirect
	golang.org/x/sys v0.15.0 // indirect
)
