package formats

import (
	"fmt"
	"strings"
)

// BuildInputArgs: -ar, -ac, -f, -i
func BuildInputArgs(arg AudioArgs, source string) []string {
	var args []string
	if IsRawPCM(arg.AudioFileFormat) {
		args = append(args, "-ar", fmt.Sprintf("%d", arg.SampleRate), "-ac", fmt.Sprintf("%d", arg.Channels))
	}
	// pipe
	if strings.HasPrefix(source, "pipe:") {
		args = append(args, "-thread_queue_size", "1024")
	}
	args = append(args, "-f", string(arg.AudioFileFormat), "-i", source)
	return args
}

// BuildOutputArgs: -ar, -ac, -f, target
func BuildOutputArgs(arg AudioArgs, target string) []string {
	return []string{
		"-ar", fmt.Sprintf("%d", arg.SampleRate),
		"-ac", fmt.Sprintf("%d", arg.Channels),
		"-f", string(arg.AudioFileFormat),
		target,
	}
}

// BuildFilterComplex handle Split 和 Merge filter
func BuildFilterComplex(cfg *AudioConfig) (filterStr string, mapTags []string) {
	custom := cfg.GetFilterString()
	targetOut := cfg.GetOutputArg(0)

	switch cfg.OpType {
	case CHANNELSPLIT:
		// [0:a] -> [l][r]; -> [left][right]
		leftF := "anull"
		if custom != "" {
			leftF = custom
		}
		rightF := "anull"
		if custom != "" {
			rightF = custom
		}
		filterStr = fmt.Sprintf("[0:a]channelsplit=channel_layout=stereo[l][r]; [l]%s[left]; [r]%s[right]", leftF, rightF)
		mapTags = []string{"[left]", "[right]"}

	case AUDIOMERGE:
		var mergePart string
		if cfg.MergeMode == SideBySide {
			mergePart = "[0:a][1:a]join=inputs=2:channel_layout=stereo"
		} else {
			mergePart = "[0:a][1:a]amix=inputs=2:duration=longest"
			if targetOut.Channels == 2 {
				mergePart += ",pan=stereo|c0=c0|c1=c0"
			}
		}
		// custom filter
		if custom != "" {
			filterStr = fmt.Sprintf("%s[tmp]; [tmp]%s[finalout]", mergePart, custom)
			mapTags = []string{"[finalout]"}
		} else {
			filterStr = mergePart + "[out]"
			mapTags = []string{"[out]"}
		}
	}
	return
}
