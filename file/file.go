package file

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/QuincyGao/audio-go/formats"
	"github.com/QuincyGao/audio-go/utils"
)

type FileHandle struct {
	config formats.AudioConfig
	ctx    context.Context
	cancel context.CancelFunc
	cmd    *exec.Cmd
	stderr *utils.TailBuffer
}

func NewFileHandle(cfg formats.AudioConfig) *FileHandle {
	return &FileHandle{
		config: cfg,
	}
}

func (f *FileHandle) Init(ctx context.Context) error {
	f.config.SetDefaults()
	if err := f.config.Validate(); err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found")
	}
	if err := f.validateInputFiles(); err != nil {
		return fmt.Errorf("input file validation failed: %v", err)
	}

	if err := f.validateOutputFiles(); err != nil {
		return fmt.Errorf("output file validation failed: %v", err)
	}

	var args []string
	switch f.config.OpType {
	case formats.FORMATCONVERT:
		args, err = f.buildConvertArgs()
	case formats.CHANNELSPLIT:
		args, err = f.buildSplitArgs()
	case formats.AUDIOMERGE:
		args, err = f.buildMergeArgs()
	default:
		return fmt.Errorf("unsupported file opType: %s", f.config.OpType)
	}
	if err != nil {
		return err
	}
	f.stderr = &utils.TailBuffer{Limit: 2048}

	f.ctx, f.cancel = context.WithCancel(ctx)
	f.cmd = exec.CommandContext(f.ctx, path, args...)
	f.cmd.Stderr = f.stderr

	return nil
}

func (f *FileHandle) Run() error {

	return f.cmd.Start()
}

func (f *FileHandle) Wait() error {
	err := f.cmd.Wait()
	if err != nil {
		if f.ctx.Err() != nil {
			return f.ctx.Err()
		}
		errMsg := f.stderr.String()
		if errMsg != "" {
			return fmt.Errorf("ffmpeg exit error: %w, stderr: %s", err, errMsg)
		}
		return fmt.Errorf("ffmpeg exit error: %w", err)
	}
	return nil
}

func (f *FileHandle) Done() {
	if f.cancel != nil {
		f.cancel()
	}
}

func (f *FileHandle) validateInputFiles() error {
	for i, inputFile := range f.config.InputFiles {
		if inputFile == "" {
			return fmt.Errorf("input file at index %d is empty", i)
		}

		if err := f.checkFileReadable(inputFile); err != nil {
			return fmt.Errorf("input file invalid: %s, error: %v", inputFile, err)
		}
	}
	return nil
}

func (f *FileHandle) validateOutputFiles() error {
	checkedDirs := make(map[string]bool)

	for i, outputFile := range f.config.OutputFiles {
		if outputFile == "" {
			return fmt.Errorf("output file at index %d is empty", i)
		}
		outputDir := filepath.Dir(outputFile)

		if !checkedDirs[outputDir] {
			if err := f.checkDirectoryWritable(outputDir); err != nil {
				return fmt.Errorf("output directory is not writable: %s, error: %v", outputDir, err)
			}
			checkedDirs[outputDir] = true
		}

		if f.checkFileExists(outputFile) {
			if err := f.checkFileWritable(outputFile); err != nil {
				return fmt.Errorf("output file already exists and is not writable: %s, error: %v", outputFile, err)
			}
		}
	}
	return nil
}

func (f *FileHandle) checkFileReadable(filePath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist")
		}
		return fmt.Errorf("cannot stat file: %v", err)
	}

	if info.IsDir() {
		return fmt.Errorf("is a directory, not a file")
	}

	if info.Size() == 0 {
		return fmt.Errorf("file is empty")
	}
	// linux
	if sysInfo, ok := info.Sys().(*syscall.Stat_t); ok {
		if uint32(sysInfo.Mode)&0400 == 0 && uint32(sysInfo.Mode)&0044 == 0 {
			return fmt.Errorf("no read permission")
		}
	} else {
		file, err := os.OpenFile(filePath, os.O_RDONLY, 0)
		if err != nil {
			return fmt.Errorf("cannot open file for reading: %v", err)
		}
		file.Close()
	}

	return nil
}

func (f *FileHandle) checkFileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

func (f *FileHandle) checkDirectoryWritable(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("cannot create directory: %v", err)
		}
		return nil
	}

	info, err := os.Stat(dirPath)
	if err != nil {
		return fmt.Errorf("cannot stat directory: %v", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("is not a directory")
	}

	if sysInfo, ok := info.Sys().(*syscall.Stat_t); ok {
		if uint32(sysInfo.Mode)&0200 == 0 && uint32(sysInfo.Mode)&0002 == 0 {
			return fmt.Errorf("no write permission")
		}
	}

	tempFile, err := os.CreateTemp(dirPath, ".write_test_*")
	if err != nil {
		return fmt.Errorf("cannot create file in directory: %v", err)
	}
	tempFile.Close()
	os.Remove(tempFile.Name())

	return nil
}

func (f *FileHandle) checkFileWritable(filePath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("cannot stat file: %v", err)
	}
	if info.IsDir() {
		return fmt.Errorf("is a directory, not a file")
	}

	if sysInfo, ok := info.Sys().(*syscall.Stat_t); ok {
		if uint32(sysInfo.Mode)&0200 == 0 && uint32(sysInfo.Mode)&0002 == 0 {
			return fmt.Errorf("no write permission")
		}
		return nil
	}

	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return fmt.Errorf("cannot open file for writing: %v", err)
	}
	file.Close()

	return nil
}

func (f *FileHandle) buildConvertArgs() ([]string, error) {
	if len(f.config.InputFiles) == 0 {
		return nil, fmt.Errorf("no input file specified")
	}
	if len(f.config.OutputFiles) == 0 {
		return nil, fmt.Errorf("no output file specified")
	}

	in := f.config.GetInputArg(0)
	out := f.config.GetOutputArg(0)
	args := []string{"-y"}

	if formats.IsRawPCM(in.AudioFileFormat) {
		args = append(args, "-ar", fmt.Sprintf("%d", in.SampleRate), "-ac", fmt.Sprintf("%d", in.Channels))
	}
	args = append(args, "-f", string(in.AudioFileFormat), "-i", f.config.InputFiles[0])

	args = append(args, "-af", fmt.Sprintf("aresample=%d", out.SampleRate))
	args = append(args,
		"-ar", fmt.Sprintf("%d", out.SampleRate),
		"-ac", fmt.Sprintf("%d", out.Channels),
		"-f", string(out.AudioFileFormat),
		f.config.OutputFiles[0],
	)
	return args, nil
}

// 2. 声道拆分：input1 (立体声) -> output1 (左), output2 (右)
func (f *FileHandle) buildSplitArgs() ([]string, error) {
	if len(f.config.InputFiles) == 0 {
		return nil, fmt.Errorf("channel split requires 1 input file")
	}
	if len(f.config.OutputFiles) < 2 {
		return nil, fmt.Errorf("channel split requires exactly 2 output files (Left/Right)")
	}

	in := f.config.GetInputArg(0)
	outL := f.config.GetOutputArg(0)
	outR := f.config.GetOutputArg(1)

	args := []string{"-y"}
	if formats.IsRawPCM(in.AudioFileFormat) {
		args = append(args, "-ar", fmt.Sprintf("%d", in.SampleRate), "-ac", fmt.Sprintf("%d", in.Channels))
	}
	args = append(args, "-f", string(in.AudioFileFormat), "-i", f.config.InputFiles[0])

	filterStr := fmt.Sprintf(
		"[0:a]channelsplit=channel_layout=stereo[l][r]; [l]aresample=%d[left]; [r]aresample=%d[right]",
		outL.SampleRate, outR.SampleRate,
	)

	args = append(args, "-filter_complex", filterStr)
	args = append(args,
		"-map", "[left]",
		"-ar", fmt.Sprintf("%d", outL.SampleRate),
		"-f", string(outL.AudioFileFormat),
		f.config.OutputFiles[0])
	args = append(args,
		"-map", "[right]",
		"-ar", fmt.Sprintf("%d", outR.SampleRate),
		"-f", string(outR.AudioFileFormat),
		f.config.OutputFiles[1])
	return args, nil
}

// 3. 音频合成：input1, input2... -> output1
func (f *FileHandle) buildMergeArgs() ([]string, error) {
	if len(f.config.InputFiles) < 2 {
		return nil, fmt.Errorf("audio merge requires at least 2 input files")
	}
	if len(f.config.OutputFiles) == 0 {
		return nil, fmt.Errorf("no output file specified for audio merge")
	}

	targetOut := f.config.GetOutputArg(0)
	args := []string{"-y"}

	for i, filePath := range f.config.InputFiles {
		in := f.config.GetInputArg(i)
		if formats.IsRawPCM(in.AudioFileFormat) {
			args = append(args, "-ar", fmt.Sprintf("%d", in.SampleRate), "-ac", fmt.Sprintf("%d", in.Channels))
		}
		args = append(args, "-f", string(in.AudioFileFormat), "-i", filePath)
	}

	// 构建滤镜：先重采样再混合
	filterComplex := ""
	for i := range f.config.InputFiles {
		filterComplex += fmt.Sprintf("[%d:a]aresample=%d[a%d]; ", i, targetOut.SampleRate, i)
	}

	if f.config.MergeMode == formats.SideBySide {
		filterComplex += "[a0][a1]join=inputs=2:channel_layout=stereo[out]"
	} else {
		filterComplex += fmt.Sprintf("[a0][a1]amix=inputs=%d:duration=longest[mixed]", len(f.config.InputFiles))
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
		"-f", string(targetOut.AudioFileFormat), f.config.OutputFiles[0])
	return args, nil
}

func (f *FileHandle) WriteTo(index int, data []byte) error {
	return fmt.Errorf("WriteTo is not supported in File mode")
}

func (f *FileHandle) ReadFrom(index int, p []byte) (int, error) {
	return 0, fmt.Errorf("ReadFrom is not supported in File mode")
}

func (f *FileHandle) CloseInput() {}
