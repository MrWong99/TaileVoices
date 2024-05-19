# TaileVoices Discord Bot integration

This subfolder contains the code that does Discord Bot integration for the agents.

## Prerequisites

You need to **clone the submodules** located in the [third_party](./third_party/) folder!
Also you need to set these environment variables during testing and building:

```env
STT_MODEL_PATH=${workspaceFolder}/discord-bot/third_party/whisper.cpp/bindings/go/models/ggml-large-v3.bin
CGO_LDFLAGS=-L/opt/cuda/targets/x86_64-linux/lib/stubs/ -L/opt/cuda/lib64/ -L/usr/lib/wsl/lib/ -lcuda -lcublas -lcudart -lculibos -lcublasLt
C_INCLUDE_PATH=${workspaceFolder}/discord-bot/third_party/whisper.cpp/
LIBRARY_PATH=${workspaceFolder}/discord-bot/third_party/whisper.cpp/
```

This assumes you have Cuda setup. Also to get the ggml-large-v3.bin file you must:

1. Navigate to [third_party/whisper.cpp/bindings/go](./third_party/whisper.cpp/bindings/go/)
2. Run `make all` once if not done already.
3. Download the model using `./build/go-model-download --out models/ --timeout 2h30m -- ggml-large-v3`
