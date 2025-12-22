package audiogo

import (
	"context"
	"fmt"

	"github.com/QuincyGao/audio-go/file"
	"github.com/QuincyGao/audio-go/formats"
	"github.com/QuincyGao/audio-go/stream"
)

type AudioEngine struct {
	processor Processor
	running   bool
}

type AudioEngineType int

const (
	Stream AudioEngineType = iota
	File
)

func NewAudioEngine(engineType AudioEngineType,
	config formats.AudioConfig) *AudioEngine {
	engine := &AudioEngine{}
	switch engineType {
	case Stream:
		engine.processor = stream.NewStreamHandle(config)
	case File:
		engine.processor = file.NewFileHandle(config)
	}
	return engine
}

func (ae *AudioEngine) Start(ctx context.Context) error {
	if err := ae.processor.Init(ctx); err != nil {
		return err
	}
	if err := ae.processor.Run(); err != nil {
		return err
	}
	ae.running = true
	return nil
}

func (ae *AudioEngine) Wait() error {
	if !ae.running {
		return fmt.Errorf("engine not running")
	}
	return ae.processor.Wait()
}

func (ae *AudioEngine) WritePrimary(data []byte) error {
	return ae.processor.WriteTo(0, data)
}

// WriteSecondary 写入辅助输入（Index 1，用于 Merge 场景）
func (ae *AudioEngine) WriteSecondary(data []byte) error {
	return ae.processor.WriteTo(1, data)
}

// ReadLeft 读取左声道/第一路输出（Index 0）
func (ae *AudioEngine) ReadLeft(p []byte) (int, error) {
	return ae.processor.ReadFrom(0, p)
}

// ReadRight 读取右声道/第二路输出（Index 1，用于 Split 场景）
func (ae *AudioEngine) ReadRight(p []byte) (int, error) {
	return ae.processor.ReadFrom(1, p)
}
func (ae *AudioEngine) CloseInput() {
	if !ae.running {
		return
	}
	ae.processor.CloseInput()
}

func (ae *AudioEngine) Done() {
	if !ae.running {
		return
	}
	ae.processor.Done()
	ae.running = false
}
