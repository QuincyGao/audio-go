package formats

import (
	"errors"
	"fmt"
)

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
	AAC   AudioFileFormat = "aac"
	GSM   AudioFileFormat = "gsm"
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
	// Mix
	Mix MergeMode = iota
	// SideBySide: stereo
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
	return fmt != WAV && fmt != MP3 && fmt != G722 && fmt != G729 && fmt != OPUS && fmt != AAC
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
	if err := c.validateOpType(); err != nil {
		return err
	}

	if err := c.validateInputArgs(); err != nil {
		return err
	}

	if err := c.validateOutputArgs(); err != nil {
		return err
	}

	return c.validateOpSpecificRules()
}

// validateOpType validates the operation type
func (c *AudioConfig) validateOpType() error {
	validOps := map[string]bool{
		FORMATCONVERT: true,
		CHANNELSPLIT:  true,
		AUDIOMERGE:    true,
	}

	if !validOps[c.OpType] {
		return fmt.Errorf("invalid OpType: %s", c.OpType)
	}
	return nil
}

// validateInputArgs validates all input arguments
func (c *AudioConfig) validateInputArgs() error {
	for i := range c.InputArgs {
		arg := c.GetInputArg(i)
		isInputRaw := IsRawPCM(arg.AudioFileFormat)
		label := fmt.Sprintf("InputArgs[%d]", i)
		if err := arg.check(label, isInputRaw); err != nil {
			return err
		}
	}
	return nil
}

// validateOutputArgs validates all output arguments
func (c *AudioConfig) validateOutputArgs() error {
	for i := range c.OutputArgs {
		arg := c.GetOutputArg(i)
		label := fmt.Sprintf("OutputArgs[%d]", i)
		if err := arg.check(label, true); err != nil {
			return err
		}
	}
	return nil
}

// validateOpSpecificRules validates operation-specific rules
func (c *AudioConfig) validateOpSpecificRules() error {
	switch c.OpType {
	case CHANNELSPLIT:
		return c.validateChannelSplit()
	case AUDIOMERGE:
		return c.validateAudioMerge()
	}
	return nil
}

// validateChannelSplit validates CHANNELSPLIT specific rules
func (c *AudioConfig) validateChannelSplit() error {
	inArg := c.GetInputArg(0)
	if inArg.Channels != 2 {
		return errors.New("CHANNELSPLIT requires input channels to be 2 (Stereo)")
	}
	if len(c.OutputArgs) > 1 && len(c.OutputArgs) < 2 {
		return errors.New("CHANNELSPLIT needs at least 2 OutputArgs for Left and Right channels")
	}
	return nil
}

// validateAudioMerge validates AUDIOMERGE specific rules
func (c *AudioConfig) validateAudioMerge() error {
	if c.MergeMode == SideBySide {
		outArg := c.GetOutputArg(0)
		if outArg.Channels != 2 {
			return errors.New("SideBySide MergeMode requires OutputArgs.Channels to be 2")
		}
	}

	for i := range 2 {
		if c.GetInputArg(i).Channels > 1 && c.MergeMode == SideBySide {
			return fmt.Errorf("input %d must be Mono (Channels=1) for SideBySide Merge", i)
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
