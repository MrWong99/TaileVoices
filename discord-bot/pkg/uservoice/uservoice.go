package uservoice

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log/slog"
	"sync"

	"github.com/MrWong99/TaileVoices/discord_bot/pkg/audio"
	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"gopkg.in/hraban/opus.v2"
)

var zeroFloat32 []byte

func init() {
	err := binary.Write(bytes.NewBuffer(zeroFloat32), binary.LittleEndian, float32(0))
	if err != nil {
		panic(fmt.Errorf("could not store zero value as little endian float32: %w", err))
	}
}

type VoiceData struct {
	Username         string
	Language         string
	SSRC             int
	TextSegments     []whisper.Segment
	sampleRate       int
	channels         int
	bufferedAudio    []byte
	audioBufferMutex *sync.Mutex
	newVoice         chan []byte
	previousSegment  *whisper.Segment
	minimumLength    int
	maximumLength    int
}

// New returns a new VoiceData object for the given user, to be used as audio buffer and trascriber.
// There will only be errors returned if the opus decoder could not be initialized.
//
// The language will be used for transcribing the voice data and can be set to "auto".
func New(username, language string, ssrc, sampleRate, channels int) (*VoiceData, error) {
	decoder, err := opus.NewDecoder(sampleRate, channels)
	if err != nil {
		return nil, err
	}
	vd := &VoiceData{
		bufferedAudio:    make([]byte, 0),
		audioBufferMutex: new(sync.Mutex),
		newVoice:         make(chan []byte, 20),
		TextSegments:     make([]whisper.Segment, 0),
		Username:         username,
		Language:         language,
		SSRC:             ssrc,
		sampleRate:       sampleRate,
		channels:         channels,
		minimumLength:    whisper.SampleRate * 500 * 4,   // 500 frames of float32 data
		maximumLength:    whisper.SampleRate * 10000 * 4, // 10000 frames of float32 data
	}
	go vd.start(decoder)
	return vd, nil
}

func (vd *VoiceData) start(decoder *opus.Decoder) {
	counter := 0
	for {
		data, ok := <-vd.newVoice
		if !ok {
			return
		}
		counter++
		if err := vd.resampleAndStore(data, decoder); err != nil {
			slog.Warn("could not resample voice data", "username", vd.Username, "error", err)
			continue
		}
		if counter < 50 {
			continue
		}
		counter = 0
		splitIndex := vd.goodAudioSplit()
		if splitIndex == -1 {
			continue
		}
		vd.transcribeUntil(splitIndex)
	}
}

func (vd *VoiceData) resampleAndStore(data []byte, decoder *opus.Decoder) error {
	pcmBuf := make([]float32, 3000)
	n, err := decoder.DecodeFloat32(data, pcmBuf)
	if err != nil {
		return nil
	}
	pcmBuf = pcmBuf[:n]
	inBuf := new(bytes.Buffer)
	for _, pcm := range pcmBuf {
		binary.Write(inBuf, binary.LittleEndian, pcm)
	}
	outBuf := new(bytes.Buffer)
	defer func() {
		vd.audioBufferMutex.Lock()
		vd.bufferedAudio = append(vd.bufferedAudio, outBuf.Bytes()...)
		vd.audioBufferMutex.Unlock()
	}()
	return audio.Resample(&audio.AudioInput{
		Data:       inBuf,
		Channels:   vd.channels,
		SampleRate: vd.sampleRate,
		Format:     audio.F32le,
	}, &audio.AudioOutput{
		Output:     outBuf,
		Channels:   1,
		SampleRate: whisper.SampleRate,
		Format:     audio.F32le,
	})
}

func (vd *VoiceData) goodAudioSplit() int {
	audioLength := len(vd.bufferedAudio)
	if audioLength < vd.minimumLength {
		return -1
	}
	if audioLength >= vd.maximumLength {
		return audioLength
	}
	framesOfSilence := vd.sampleRate
	i := 0
	for ; i < audioLength && framesOfSilence > 0; i += 4 {
		oneFloatData := vd.bufferedAudio[i : i+5]
		if bytes.Equal(oneFloatData, zeroFloat32) {
			framesOfSilence--
		} else {
			// Reset
			framesOfSilence = vd.sampleRate
		}
	}
	if framesOfSilence == 0 {
		return i
	}
	return -1
}

func (vd *VoiceData) transcribeUntil(index int) {
	vd.audioBufferMutex.Lock()
	toTranscribe := vd.bufferedAudio[:index]
	vd.bufferedAudio = vd.bufferedAudio[index:]
	vd.audioBufferMutex.Unlock()
	go func() {
		floatAudio, err := audio.ReadBytes[float32](bytes.NewReader(toTranscribe))
		if err != nil {
			slog.Error("could not read audio bytes into float32", "error", err)
			return
		}
		stt, err := audio.NewSTT(vd.Language)
		if err != nil {
			slog.Error("could not create new transcription context", "error", err)
			return
		}
		var segments []whisper.Segment
		if vd.previousSegment == nil {
			segments, err = stt.Transcribe(floatAudio)
		} else {
			segments, err = stt.TranscribeWithPromptAndOffset(floatAudio, vd.previousSegment.Text, vd.previousSegment.End)
		}
		if err != nil {
			slog.Error("could not transcribe audio sample", "error", err)
			return
		}
		lastSegment := segments[len(segments)-1]
		if lastSegment.Text != "" {
			vd.previousSegment = &lastSegment
		}
		vd.TextSegments = append(vd.TextSegments, segments...)
	}()
}

// Process given audio data.
// The audio data must be in Opus format with the sample rate and channel count set with New().
//
// This method is safe to be called concurrently but it might block until the internal processing queue has been cleared.
func (vd *VoiceData) Process(data []byte) {
	vd.newVoice <- data
}

// Close and stop the voice receiver. You can't call process afterwards.
func (vd *VoiceData) Close() {
	close(vd.newVoice)
}
