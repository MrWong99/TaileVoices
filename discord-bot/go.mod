module github.com/MrWong99/TaileVoices/discord_bot

go 1.22.3

replace github.com/ggerganov/whisper.cpp/bindings/go => ./third_party/whisper.cpp/bindings/go

require (
	github.com/bwmarrin/discordgo v0.28.1
	github.com/ggerganov/whisper.cpp/bindings/go v0.0.0-20240525074622-a7dc2aab1682
	github.com/oov/audio v0.0.0-20171004131523-88a2be6dbe38
	github.com/sashabaranov/go-openai v1.24.1
	golang.org/x/exp v0.0.0-20240525044651-4c93da0ed11d
	gopkg.in/hraban/opus.v2 v2.0.0-20230925203106-0188a62cb302
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/gorilla/websocket v1.5.1 // indirect
	golang.org/x/crypto v0.23.0 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
)
