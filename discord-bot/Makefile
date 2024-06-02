LDFLAGS := -L/opt/cuda/targets/x86_64-linux/lib/stubs/ -L/opt/cuda/lib64/ -L/usr/lib/wsl/lib/ -lcuda -lcublas -lcudart -lculibos -lcublasLt
INCLUDES := third_party/whisper.cpp

setup.whisper:
	cd third_party/whisper.cpp/bindings/go && make whisper && ./build/go-model-download -out models/ -- ggml-large-v3.bin

go.build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 CGO_LDFLAGS="$(LDFLAGS)" C_INCLUDE_PATH="$(shell pwd)/$(INCLUDES)/" LIBRARY_PATH="$(shell pwd)/$(INCLUDES)/" go build -o bot-cuda cmd/discord-bot/main.go
