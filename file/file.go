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
	args := []string{"-y"}
	args = append(args, formats.BuildInputArgs(f.config.GetInputArg(0), f.config.InputFiles[0])...)
	if custom := f.config.GetFilterString(); custom != "" {
		args = append(args, "-af", custom)
	}
	args = append(args, formats.BuildOutputArgs(f.config.GetOutputArg(0), f.config.OutputFiles[0])...)
	return args, nil
}

func (f *FileHandle) buildSplitArgs() ([]string, error) {
	args := []string{"-y"}
	args = append(args, formats.BuildInputArgs(f.config.GetInputArg(0), f.config.InputFiles[0])...)
	fStr, tags := formats.BuildFilterComplex(&f.config)
	args = append(args, "-filter_complex", fStr)

	args = append(args, "-map", tags[0])
	args = append(args, formats.BuildOutputArgs(f.config.GetOutputArg(0), f.config.OutputFiles[0])...)
	args = append(args, "-map", tags[1])
	args = append(args, formats.BuildOutputArgs(f.config.GetOutputArg(1), f.config.OutputFiles[1])...)
	return args, nil
}

func (f *FileHandle) buildMergeArgs() ([]string, error) {
	args := []string{"-y"}
	for i, path := range f.config.InputFiles {
		args = append(args, formats.BuildInputArgs(f.config.GetInputArg(i), path)...)
	}
	fStr, tags := formats.BuildFilterComplex(&f.config)
	args = append(args, "-filter_complex", fStr, "-map", tags[0])
	args = append(args, formats.BuildOutputArgs(f.config.GetOutputArg(0), f.config.OutputFiles[0])...)
	return args, nil
}

func (f *FileHandle) WriteTo(index int, data []byte) error {
	return fmt.Errorf("WriteTo is not supported in File mode")
}

func (f *FileHandle) ReadFrom(index int, p []byte) (int, error) {
	return 0, fmt.Errorf("ReadFrom is not supported in File mode")
}

func (f *FileHandle) CloseInput() {}
