// Code generated by "stringer -type AudioFormat"; DO NOT EDIT.

package audio

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[Opus-0]
	_ = x[F32le-1]
	_ = x[S16le-2]
	_ = x[Wav-3]
	_ = x[Mp3-4]
}

const _AudioFormat_name = "OpusF32leS16leWavMp3"

var _AudioFormat_index = [...]uint8{0, 4, 9, 14, 17, 20}

func (i AudioFormat) String() string {
	if i < 0 || i >= AudioFormat(len(_AudioFormat_index)-1) {
		return "AudioFormat(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _AudioFormat_name[_AudioFormat_index[i]:_AudioFormat_index[i+1]]
}