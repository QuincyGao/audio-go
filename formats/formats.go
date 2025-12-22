package formats

import (
	"errors"
	"fmt"
)

// // -c:a args
// type AudioCodecFormat string

// const (
// 	PCM_ALAW         AudioCodecFormat = "alaw"             // mu-law / G.711 mu-law
// 	PCM_F16LE        AudioCodecFormat = "pcm_f16le"        // 16.8 floating point little-endian
// 	PCM_F24LE        AudioCodecFormat = "pcm_f24le"        // 24.0 floating point little-endian
// 	PCM_F32BE        AudioCodecFormat = "pcm_f32be"        // 32-bit floating point big-endian
// 	PCM_F32LE        AudioCodecFormat = "pcm_f32le"        // 32-bit floating point little-endian
// 	PCM_F64BE        AudioCodecFormat = "pcm_f64be"        // 64-bit floating point big-endian
// 	PCM_F64LE        AudioCodecFormat = "pcm_f64le"        // 64-bit floating point little-endian
// 	PCM_MULAW        AudioCodecFormat = "pcm_mulaw"        // mu-law / G.711 mu-law
// 	PCM_S16BE        AudioCodecFormat = "pcm_s16be"        // signed 16-bit big-endian
// 	PCM_S16BE_PLANAR AudioCodecFormat = "pcm_s16be_planar" // signed 16-bit big-endian planar
// 	PCM_S16LE        AudioCodecFormat = "pcm_s16le"        // signed 16-bit little-endian
// 	PCM_S16LE_PLANAR AudioCodecFormat = "pcm_s16le_planar" // signed 16-bit little-endian planar
// 	PCM_S24BE        AudioCodecFormat = "pcm_s24be"        // signed 24-bit big-endian
// 	PCM_S24DAUD      AudioCodecFormat = "pcm_s24daud"      // D-Cinema audio signed 24-bit
// 	PCM_S24LE        AudioCodecFormat = "pcm_s24le"        // signed 24-bit little-endian
// 	PCM_S24LE_PLANAR AudioCodecFormat = "pcm_s24le_planar" // signed 24-bit little-endian planar
// 	PCM_S32BE        AudioCodecFormat = "pcm_s32be"        // signed 32-bit big-endian
// 	PCM_S32LE        AudioCodecFormat = "pcm_s32le"        // signed 32-bit little-endian
// 	PCM_S32LE_PLANAR AudioCodecFormat = "pcm_s32le_planar" // signed 32-bit little-endian planar
// 	PCM_S64BE        AudioCodecFormat = "pcm_s64be"        // signed 64-bit big-endian
// 	PCM_S64LE        AudioCodecFormat = "pcm_s64le"        // signed 64-bit little-endian
// 	PCM_S8           AudioCodecFormat = "pcm_s8"           // signed 8-bit
// 	PCM_S8_PLANAR    AudioCodecFormat = "pcm_s8_planar"    // signed 8-bit planar
// 	PCM_U16BE        AudioCodecFormat = "pcm_u16be"        // unsigned 16-bit big-endian
// 	PCM_U16LE        AudioCodecFormat = "pcm_u16le"        // unsigned 16-bit little-endian
// 	PCM_U24BE        AudioCodecFormat = "pcm_u24be"        // unsigned 24-bit big-endian
// 	PCM_U24LE        AudioCodecFormat = "pcm_u24le"        // unsigned 24-bit little-endian
// 	PCM_U32BE        AudioCodecFormat = "pcm_u32be"        // unsigned 32-bit big-endian
// 	PCM_U32LE        AudioCodecFormat = "pcm_u32le"        // unsigned 32-bit little-endian
// 	PCM_U8           AudioCodecFormat = "pcm_u8"           // unsigned 8-bit
// 	C_MP3            AudioCodecFormat = "mp3"
// 	WAVPACK          AudioCodecFormat = "wavpack"
// 	C_AAC            AudioCodecFormat = "aac"
// 	C_OPUS           AudioCodecFormat = "opus"
// 	C_GSM            AudioCodecFormat = "gsm"
// 	C_G729           AudioCodecFormat = "g729"
// )

// -f args
type AudioFileFormat string

const (
	ALAW  AudioFileFormat = "alaw"
	F32BE AudioFileFormat = "f32be"
	F32LE AudioFileFormat = "f32le"
	F64BE AudioFileFormat = "f64be"
	F64LE AudioFileFormat = "f64le"
	MULAW AudioFileFormat = "mulaw" // mu-law
	S16BE AudioFileFormat = "s16be"
	S16LE AudioFileFormat = "s16le"
	S24BE AudioFileFormat = "s24be"
	S24LE AudioFileFormat = "s24le"
	S32BE AudioFileFormat = "s32be"
	S32LE AudioFileFormat = "s32le"
	S8    AudioFileFormat = "s8" // signed 8-bit
	U16BE AudioFileFormat = "u16be"
	U16LE AudioFileFormat = "u16le"
	U24BE AudioFileFormat = "u24be"
	U24LE AudioFileFormat = "u24le"
	U32BE AudioFileFormat = "u32be"
	U32LE AudioFileFormat = "u32le"
	U8    AudioFileFormat = "u8" // unsigned 8-bit
	WAV   AudioFileFormat = "wav"
	MP3   AudioFileFormat = "mp3"
	G722  AudioFileFormat = "g722"
	G729  AudioFileFormat = "bit"
	OPUS  AudioFileFormat = "opus"
)

const (
	// FORMATCONVERT
	FORMATCONVERT string = "FormatConvert"
	// CHANNELSPLIT
	CHANNELSPLIT string = "ChannelSplit"
	// AUDIOMERGE
	AUDIOMERGE string = "AudioMerge"
)

type MergeMode int

const (
	// Mix 模式：将两路声音混合在一起（听起来是重叠的）
	Mix MergeMode = iota
	// SideBySide 模式：左声道放第一路，右声道放第二路（立体声）
	SideBySide
)

type AudioArgs struct {
	AudioFileFormat
	SampleRate int
	Channels   int
}

type AudioConfig struct {
	InputArgs   []AudioArgs
	OutputArgs  []AudioArgs
	OpType      string
	MergeMode   MergeMode
	InputFiles  []string
	OutputFiles []string
}

func IsRawPCM(fmt AudioFileFormat) bool {
	return fmt != WAV && fmt != MP3 && fmt != G722 && fmt != G729 && fmt != OPUS
}

// If only one AudioArgs is provided in the slice, it is used for all indices.
func (c *AudioConfig) GetInputArg(index int) AudioArgs {
	if len(c.InputArgs) == 0 {
		return AudioArgs{}
	}
	if len(c.InputArgs) == 1 {
		return c.InputArgs[0]
	}
	if index < len(c.InputArgs) {
		return c.InputArgs[index]
	}
	// Fallback to the last available config if index is out of range
	return c.InputArgs[len(c.InputArgs)-1]
}

// GetOutputArg returns the AudioArgs at the specified index.
// If only one AudioArgs is provided in the slice, it is used for all indices.
func (c *AudioConfig) GetOutputArg(index int) AudioArgs {
	if len(c.OutputArgs) == 0 {
		return AudioArgs{}
	}
	if len(c.OutputArgs) == 1 {
		return c.OutputArgs[0]
	}
	if index < len(c.OutputArgs) {
		return c.OutputArgs[index]
	}
	return c.OutputArgs[len(c.OutputArgs)-1]
}

// SetDefaults fills in missing configuration values with sensible defaults
func (c *AudioConfig) SetDefaults() {
	if c.OpType == "" {
		c.OpType = FORMATCONVERT
	}
	// Default at least one arg if slices are empty
	if len(c.InputArgs) == 0 {
		c.InputArgs = append(c.InputArgs, AudioArgs{})
	}
	if len(c.OutputArgs) == 0 {
		c.OutputArgs = append(c.OutputArgs, AudioArgs{})
	}

	for i := range c.InputArgs {
		if c.InputArgs[i].SampleRate <= 0 {
			c.InputArgs[i].SampleRate = 8000
		}
		if c.InputArgs[i].Channels <= 0 {
			c.InputArgs[i].Channels = 1
		}
	}
	for i := range c.OutputArgs {
		if c.OutputArgs[i].SampleRate <= 0 {
			c.OutputArgs[i].SampleRate = 8000
		}
		if c.OutputArgs[i].Channels <= 0 {
			c.OutputArgs[i].Channels = 1
		}
	}
}

// Validate checks the configuration for logical errors and missing required fields
func (c *AudioConfig) Validate() error {
	// 1. 基础操作类型检查
	if c.OpType != FORMATCONVERT && c.OpType != CHANNELSPLIT && c.OpType != AUDIOMERGE {
		return fmt.Errorf("invalid OpType: %s", c.OpType)
	}

	// 2. 校验所有输入参数 (InputArgs Slice)
	for i := range c.InputArgs {
		arg := c.GetInputArg(i)
		// Raw PCM 格式必须有采样率和声道，否则 FFmpeg 无法解析 pipe 流
		isInputRaw := IsRawPCM(arg.AudioFileFormat)
		label := fmt.Sprintf("InputArgs[%d]", i)
		if err := arg.check(label, isInputRaw); err != nil {
			return err
		}
	}

	// 3. 校验所有输出参数 (OutputArgs Slice)
	for i := range c.OutputArgs {
		arg := c.GetOutputArg(i)
		// 输出建议始终明确参数以确保结果可预测
		label := fmt.Sprintf("OutputArgs[%d]", i)
		if err := arg.check(label, true); err != nil {
			return err
		}
	}

	// 4. 特定操作类型的深度逻辑校验
	switch c.OpType {
	case CHANNELSPLIT:
		// 逻辑校验：声道拆分通常针对 2 声道输入
		inArg := c.GetInputArg(0)
		if inArg.Channels != 2 {
			return errors.New("CHANNELSPLIT requires input channels to be 2 (Stereo)")
		}
		// 即使 OutputArgs 只配了一个，逻辑上也应该支持产生两个输出流
		// 如果用户显式配了多个，则数量不能少于 2
		if len(c.OutputArgs) > 1 && len(c.OutputArgs) < 2 {
			return errors.New("CHANNELSPLIT needs at least 2 OutputArgs for Left and Right channels")
		}

	case AUDIOMERGE:
		// 逻辑校验：合成通常至少需要两个输入源
		// 注意：如果 InputArgs 长度为 1，StreamHandle 也会开启两个 Pipe，所以这里允许 len 为 1
		// 但如果用户配了多个，确保它是有效的
		if c.MergeMode == SideBySide {
			outArg := c.GetOutputArg(0)
			if outArg.Channels != 2 {
				return errors.New("SideBySide MergeMode requires OutputArgs.Channels to be 2")
			}
		}

		// 校验输入声道：通常合并的操作针对单声道输入进行 join/amix
		for i := range 2 { // 检查前两路关键输入
			if c.GetInputArg(i).Channels > 1 && c.MergeMode == SideBySide {
				return fmt.Errorf("input %d must be Mono (Channels=1) for SideBySide Merge", i)
			}
		}
	}

	return nil
}

// check stays as a helper to verify AudioArgs fields
func (a *AudioArgs) check(label string, required bool) error {
	if a.AudioFileFormat == "" {
		return fmt.Errorf("%s: AudioFileFormat is missing", label)
	}

	if required {
		if a.SampleRate <= 0 {
			return fmt.Errorf("%s: SampleRate is required for raw formats or output", label)
		}
		if a.Channels <= 0 {
			return fmt.Errorf("%s: Channels is required for raw formats or output", label)
		}
	}
	return nil
}
