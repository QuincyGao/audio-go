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
	fmt.Printf("args: %+v\n", args)
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
	args = append(args, formats.BuildInputArgs(s.config.GetInputArg(0), "pipe:0")...)
	if custom := s.config.GetFilterString(); custom != "" {
		args = append(args, "-af", custom)
	}
	args = append(args, formats.BuildOutputArgs(s.config.GetOutputArg(0), "pipe:1")...)
	return args
}

func (s *StreamHandle) buildSplitArgs(args []string) []string {
	args = append(args, formats.BuildInputArgs(s.config.GetInputArg(0), "pipe:0")...)
	fStr, tags := formats.BuildFilterComplex(&s.config)
	args = append(args, "-filter_complex", fStr)
	// 映射输出
	args = append(args, "-map", tags[0])
	args = append(args, formats.BuildOutputArgs(s.config.GetOutputArg(0), "pipe:1")...)
	args = append(args, "-map", tags[1])
	args = append(args, formats.BuildOutputArgs(s.config.GetOutputArg(1), "pipe:3")...)
	return args
}

func (s *StreamHandle) buildMergeArgs(args []string) []string {
	for i := 0; i < 2; i++ {
		src := "pipe:0"
		if i > 0 {
			src = fmt.Sprintf("pipe:%d", i+2)
		}
		args = append(args, formats.BuildInputArgs(s.config.GetInputArg(i), src)...)
	}
	fStr, tags := formats.BuildFilterComplex(&s.config)
	args = append(args, "-filter_complex", fStr, "-map", tags[0])
	args = append(args, formats.BuildOutputArgs(s.config.GetOutputArg(0), "pipe:1")...)
	return args
}

func (s *StreamHandle) setupPipes() error {
	in0, _ := s.cmd.StdinPipe()
	out0, _ := s.cmd.StdoutPipe()
	s.stdins = append(s.stdins, in0)
	s.stdouts = append(s.stdouts, out0)

	if s.config.OpType == formats.CHANNELSPLIT {
		pr, pw, _ := os.Pipe()
		s.cmd.ExtraFiles = append(s.cmd.ExtraFiles, pw) // PW send FFmpeg (fd:3)
		s.stdouts = append(s.stdouts, pr)
	}

	if s.config.OpType == formats.AUDIOMERGE {
		pr, pw, _ := os.Pipe()
		s.cmd.ExtraFiles = append(s.cmd.ExtraFiles, pr) // PR send FFmpeg (fd:3)
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
