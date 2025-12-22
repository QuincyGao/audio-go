package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	audiogo "github.com/QuincyGao/audio-go"
	"github.com/QuincyGao/audio-go/formats"
)

var (
	inpcmFile      = "test2.pcm"
	outPcmFile     = "out.pcm"
	tickerInterval = 20
	chunkByteLen   = 160
)

func main() {

	// 配置：将 48k 立体声 WAV 转换为 16k 单声道 MP3
	cfg := formats.AudioConfig{
		OpType: formats.FORMATCONVERT,
		InputArgs: []formats.AudioArgs{
			{AudioFileFormat: formats.WAV}, // 输入格式由扩展名或此参数决定
		},
		OutputArgs: []formats.AudioArgs{
			{
				AudioFileFormat: formats.MP3,
				SampleRate:      16000,
				Channels:        1,
			},
		},
		InputFiles:  []string{"high_quality.wav"},
		OutputFiles: []string{"low_bitrate_mono.mp3"},
	}

	// 创建文件引擎
	engine := audiogo.NewAudioEngine(audiogo.File, cfg)

	fmt.Println("开始文件转换...")

	// Start 会直接调用 FFmpeg 处理磁盘文件
	if err := engine.Start(context.Background()); err != nil {
		log.Fatalf("任务启动失败: %v", err)
	}

	// 等待 FFmpeg 进程结束
	if err := engine.Wait(); err != nil {
		log.Fatalf("转换过程中出错: %v", err)
	}

	fmt.Println("转换成功！输出文件：low_bitrate_mono.mp3")

	// 1. 配置：将两路单声道 PCM 合并为一路立体声 WAV
	cfg := formats.AudioConfig{
		OpType:    formats.AUDIOMERGE,
		MergeMode: formats.SideBySide, // 模式：左耳 A 流，右耳 B 流
		InputArgs: []formats.AudioArgs{
			{AudioFileFormat: formats.S16LE, SampleRate: 16000, Channels: 1},
		},
		OutputArgs: []formats.AudioArgs{
			{AudioFileFormat: formats.WAV, SampleRate: 16000, Channels: 2},
		},
	}

	// 2. 创建并启动引擎
	engine := audiogo.NewAudioEngine(audiogo.Stream, cfg)
	ctx := context.Background()
	if err := engine.Start(ctx); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
	defer engine.Done()

	// 3. 模拟输入：从本地文件读取并实时写入引擎
	// 实际场景中，这里可能是来自 WebSocket 或网络包
	f1, _ := os.Open("user_a.pcm")
	f2, _ := os.Open("user_b.pcm")
	defer f1.Close()
	defer f2.Close()

	// 协程：写入第一路流
	go func() {
		io.Copy(io.WriterFunc(func(p []byte) (n int, err error) {
			return len(p), engine.WritePrimary(p)
		}), f1)
	}()

	// 协程：写入第二路流
	go func() {
		io.Copy(io.WriterFunc(func(p []byte) (n int, err error) {
			return len(p), engine.WriteSecondary(p)
		}), f2)
	}()

	// 4. 读取输出：保存合并后的结果
	outFile, _ := os.Create("merged_output.wav")
	defer outFile.Close()

	fmt.Println("正在处理流音频...")
	buf := make([]byte, 4096)
	for {
		n, err := engine.ReadLeft(buf) // ReadLeft 获取合并后的主输出
		if n > 0 {
			outFile.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
	}

	engine.Wait()
	fmt.Println("处理完成，结果已保存至 merged_output.wav")
}
