package stream

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/QuincyGao/audio-go/formats"
	"github.com/QuincyGao/audio-go/utils"
)

type StreamHandle struct {
	config  formats.AudioConfig
	cmd     *exec.Cmd
	stdins  []io.WriteCloser
	stdouts []io.ReadCloser
	ctx     context.Context
	cancel  context.CancelFunc
	stderr  *utils.TailBuffer
}

func NewStreamHandle(cfg formats.AudioConfig) *StreamHandle {
	return &StreamHandle{
		config: cfg,
	}
}

func (s *StreamHandle) Init(ctx context.Context) (err error) {
	s.config.SetDefaults()
	if err := s.config.Validate(); err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found")
	}
	s.stderr = &utils.TailBuffer{Limit: 2048}
	var args []string
	// 通用低延迟参数
	fastArgs := []string{"-analyzeduration", "0", "-probesize", "32", "-fflags", "+nobuffer", "-flags", "+low_delay"}
	args = append(args, fastArgs...)

	switch s.config.OpType {
	case formats.FORMATCONVERT:
		args = s.buildConvertArgs(args)
	case formats.CHANNELSPLIT:
		args = s.buildSplitArgs(args)
	case formats.AUDIOMERGE:
		args = s.buildMergeArgs(args)
	default:
		return fmt.Errorf("unsupported opType: %s", s.config.OpType)
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	// fmt.Printf("args: %+v\n", args)
	s.cmd = exec.CommandContext(s.ctx, path, args...)
	s.cmd.Stderr = s.stderr
	if err := s.setupPipes(); err != nil {
		return err
	}
	return nil
}

// non-block
func (s *StreamHandle) Run() error {
	if err := s.cmd.Start(); err != nil {
		s.closeAllPipes()
		return err
	}
	for _, f := range s.cmd.ExtraFiles {
		if f != nil {
			f.Close()
		}
	}
	return nil
}

func (s *StreamHandle) Wait() error {
	if s.cmd == nil {
		return nil
	}

	err := s.cmd.Wait()
	if err != nil {
		if s.ctx.Err() != nil {
			return s.ctx.Err()
		}
		errMsg := s.stderr.String()
		if errMsg != "" {
			return fmt.Errorf("ffmpeg exit error: %w, stderr: %s", err, errMsg)
		}
		return fmt.Errorf("ffmpeg exit error: %w", err)
	}
	return nil
}

func (s *StreamHandle) buildConvertArgs(args []string) []string {
	in := s.config.GetInputArg(0)
	out := s.config.GetOutputArg(0)
	if formats.IsRawPCM(in.AudioFileFormat) {
		args = append(args, "-ar", fmt.Sprintf("%d", in.SampleRate), "-ac", fmt.Sprintf("%d", in.Channels))
	}
	args = append(args, "-f", string(in.AudioFileFormat), "-i", "pipe:0")

	// 使用 aresample 确保输出质量
	args = append(args, "-af", fmt.Sprintf("aresample=%d", out.SampleRate))
	args = append(args,
		"-ar", fmt.Sprintf("%d", out.SampleRate),
		"-ac", fmt.Sprintf("%d", out.Channels),
		"-f", string(out.AudioFileFormat),
		"pipe:1",
	)
	return args
}

func (s *StreamHandle) buildSplitArgs(args []string) []string {
	in := s.config.GetInputArg(0)
	if formats.IsRawPCM(in.AudioFileFormat) {
		args = append(args, "-ar", fmt.Sprintf("%d", in.SampleRate), "-ac", fmt.Sprintf("%d", in.Channels))
	}
	args = append(args, "-f", string(in.AudioFileFormat), "-i", "pipe:0")

	outL := s.config.GetOutputArg(0)
	outR := s.config.GetOutputArg(1)

	// 分别重采样左右声道
	filterStr := fmt.Sprintf(
		"[0:a]channelsplit=channel_layout=stereo[l][r]; [l]aresample=%d[left]; [r]aresample=%d[right]",
		outL.SampleRate, outR.SampleRate,
	)

	args = append(args, "-filter_complex", filterStr)
	args = append(args, "-map", "[left]", "-ar", fmt.Sprintf("%d", outL.SampleRate), "-f", string(outL.AudioFileFormat), "pipe:1")
	args = append(args, "-map", "[right]", "-ar", fmt.Sprintf("%d", outR.SampleRate), "-f", string(outR.AudioFileFormat), "pipe:3")
	return args
}

func (s *StreamHandle) buildMergeArgs(args []string) []string {
	numInputs := 2 // 当前标准支持 2 路
	targetOut := s.config.GetOutputArg(0)

	for i := range numInputs {
		in := s.config.GetInputArg(i)
		pipeIdx := 0
		if i > 0 {
			pipeIdx = i + 2
		}

		if formats.IsRawPCM(in.AudioFileFormat) {
			args = append(args, "-ar", fmt.Sprintf("%d", in.SampleRate), "-ac", fmt.Sprintf("%d", in.Channels))
		}
		args = append(args, "-thread_queue_size", "1024", "-f", string(in.AudioFileFormat), "-i", fmt.Sprintf("pipe:%d", pipeIdx))
	}

	var filterComplex string
	for i := range numInputs {
		filterComplex += fmt.Sprintf("[%d:a]aresample=%d[a%d]; ", i, targetOut.SampleRate, i)
	}

	if s.config.MergeMode == formats.SideBySide {
		filterComplex += "[a0][a1]join=inputs=2:channel_layout=stereo[out]"
	} else {
		filterComplex += "[a0][a1]amix=inputs=2:duration=longest[mixed]"
		if targetOut.Channels == 2 {
			filterComplex += "; [mixed]pan=stereo|c0=c0|c1=c0[out]"
		} else {
			filterComplex += "; [mixed]anull[out]"
		}
	}

	args = append(args, "-filter_complex", filterComplex, "-map", "[out]")
	args = append(args,
		"-ar", fmt.Sprintf("%d", targetOut.SampleRate),
		"-ac", fmt.Sprintf("%d", targetOut.Channels),
		"-f", string(targetOut.AudioFileFormat), "pipe:1")
	return args
}

func (s *StreamHandle) setupPipes() error {
	// 标准输入输出 (Index 0)
	in0, _ := s.cmd.StdinPipe()
	out0, _ := s.cmd.StdoutPipe()
	s.stdins = append(s.stdins, in0)
	s.stdouts = append(s.stdouts, out0)

	// 如果是拆分，需要额外创建一个 ReadPipe
	if s.config.OpType == formats.CHANNELSPLIT {
		pr, pw, _ := os.Pipe()
		s.cmd.ExtraFiles = append(s.cmd.ExtraFiles, pw) // PW 传给 FFmpeg (fd:3)
		s.stdouts = append(s.stdouts, pr)
	}

	// 如果是合成，需要额外创建一个 WritePipe
	if s.config.OpType == formats.AUDIOMERGE {
		pr, pw, _ := os.Pipe()
		s.cmd.ExtraFiles = append(s.cmd.ExtraFiles, pr) // PR 传给 FFmpeg (fd:3)
		s.stdins = append(s.stdins, pw)
	}

	return nil
}

func (s *StreamHandle) WriteTo(index int, data []byte) error {
	if index < len(s.stdins) && s.stdins[index] != nil {
		_, err := s.stdins[index].Write(data)
		return err
	}
	return fmt.Errorf("stdin index %d out of range", index)
}

func (s *StreamHandle) ReadFrom(index int, p []byte) (int, error) {
	if index < len(s.stdouts) && s.stdouts[index] != nil {
		return s.stdouts[index].Read(p)
	}
	return 0, fmt.Errorf("stdout index %d out of range", index)
}

func (s *StreamHandle) CloseInput() {
	for _, in := range s.stdins {
		if in != nil {
			in.Close()
		}
	}
}

func (s *StreamHandle) Done() {
	s.cancel()
	s.closeAllPipes()
}

func (s *StreamHandle) closeAllPipes() {
	for _, in := range s.stdins {
		if in != nil {
			in.Close()
		}
	}
	for _, out := range s.stdouts {
		if out != nil {
			out.Close()
		}
	}
}
