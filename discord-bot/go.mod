module github.com/MrWong99/TaileVoices/discord_bot

go 1.23.0

replace github.com/ggerganov/whisper.cpp/bindings/go => ./third_party/whisper.cpp/bindings/go

require (
	github.com/bwmarrin/discordgo v0.28.1
	github.com/ggerganov/whisper.cpp/bindings/go v0.0.0-20240525074622-a7dc2aab1682
	github.com/oov/audio v0.0.0-20171004131523-88a2be6dbe38
	github.com/sashabaranov/go-openai v1.24.1
	github.com/weaviate/weaviate v1.25.1
	github.com/weaviate/weaviate-go-client/v4 v4.14.0
	golang.org/x/exp v0.0.0-20240525044651-4c93da0ed11d
	gopkg.in/hraban/opus.v2 v2.0.0-20230925203106-0188a62cb302
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/PuerkitoBio/purell v1.1.1 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/go-openapi/analysis v0.21.2 // indirect
	github.com/go-openapi/errors v0.22.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.19.6 // indirect
	github.com/go-openapi/loads v0.21.1 // indirect
	github.com/go-openapi/spec v0.20.4 // indirect
	github.com/go-openapi/strfmt v0.23.0 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-openapi/validate v0.21.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.1 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	go.mongodb.org/mongo-driver v1.14.0 // indirect
	golang.org/x/crypto v0.35.0 // indirect
	golang.org/x/net v0.26.0 // indirect
	golang.org/x/oauth2 v0.20.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240318140521-94a12d6c2237 // indirect
	google.golang.org/grpc v1.64.1 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
)
