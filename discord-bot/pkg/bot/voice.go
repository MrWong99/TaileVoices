package bot

import (
	"log/slog"
	"os"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/hraban/opus.v2"
)

const (
	discordAudioSampleRate       = 48000
	discordAudioChannels         = 2
	discordAudioFrameSize        = 960
	discordPcmLength             = discordAudioFrameSize * discordAudioChannels
	discordAudioFrameSizeMs      = 20
	sttSampleDataDurationSeconds = 3
	sampleDataSize               = (1000 / discordAudioFrameSizeMs) * sttSampleDataDurationSeconds * discordAudioFrameSize
	minimumSampleDataSize        = (1000 / discordAudioFrameSizeMs) * discordAudioFrameSize * discordAudioChannels
)

var discordEncoder *opus.Encoder

func init() {
	var err error
	discordEncoder, err = opus.NewEncoder(discordAudioSampleRate, discordAudioChannels, opus.AppVoIP)
	if err != nil {
		slog.Error("could not create opus encoder for Discord requests", "error", err)
		os.Exit(1)
	}
	discordEncoder.SetBitrateToAuto()
	discordEncoder.SetMaxBandwidth(opus.Fullband)
}

func isVoiceChannel(s *discordgo.Session, channelID string) bool {
	channel, err := s.Channel(channelID)
	if err != nil {
		return false
	}
	return channel.Type == discordgo.ChannelTypeGuildVoice
}
