package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	audiogo "github.com/QuincyGao/audio-go"
	"github.com/QuincyGao/audio-go/formats"
)

const (
	testInputFile   = "./sample_data/convert.pcm"
	outInputFile    = "./sample_data/out-one-8kHz.pcm"
	stereoFile      = "./sample_data/audio-stereo.mp3"
	leftMonoFile    = "./sample_data/out-left.pcm"
	rightMonoFile   = "./sample_data/out-right.pcm"
	audiofile1      = "./sample_data/audio-8kHz.wav"
	audiofile2      = "./sample_data/audio-24kHz.mp3"
	audioStereoFile = "./sample_data/out-stereo.wav"
	tickerInterval  = 20
	chunkByteLen    = 160
)

var bufferPool = sync.Pool{
	New: func() any {
		b := make([]byte, 4096)
		return &b
	},
}

var (
	testConfig = formats.AudioConfig{
		OpType: formats.FORMATCONVERT,
		InputArgs: []formats.AudioArgs{
			{
				AudioFileFormat: formats.S16LE,
				SampleRate:      8000,
				Channels:        1,
			},
		},
		OutputArgs: []formats.AudioArgs{
			{
				AudioFileFormat: formats.WAV,
				SampleRate:      8000,
				Channels:        1,
			},
		},
	}

	splitConfig = formats.AudioConfig{
		OpType: formats.CHANNELSPLIT,
		InputArgs: []formats.AudioArgs{
			{
				AudioFileFormat: formats.MP3,
				SampleRate:      44100,
				Channels:        2,
			},
		},
		OutputArgs: []formats.AudioArgs{
			{
				AudioFileFormat: formats.S16LE,
				SampleRate:      8000,
				Channels:        1,
			},
			{
				AudioFileFormat: formats.WAV,
				SampleRate:      16000,
				Channels:        1,
			},
		},
	}

	mergeConfig = formats.AudioConfig{
		OpType:    formats.AUDIOMERGE,
		MergeMode: formats.SideBySide,
		InputArgs: []formats.AudioArgs{
			{
				AudioFileFormat: formats.WAV,
				SampleRate:      8000,
				Channels:        1,
			},
			{
				AudioFileFormat: formats.MP3,
				SampleRate:      24000,
				Channels:        1,
			},
		},
		OutputArgs: []formats.AudioArgs{
			{
				AudioFileFormat: formats.WAV,
				SampleRate:      16000,
				Channels:        2,
			},
		},
	}
)

func main() {
	fmt.Println("--- Audio-Go Library Examples ---")
	runStreamConvertExample()
	runFileConvertExample()

	runStreamSplitExample()
	runFileSplitExample()

	runStreamMergeExample()
	runFileMergeExample()

}

// runStreamMergeExample 演示如何实时合并两个音频流
func runStreamMergeExample() {
	log.Println("\n[Example] Real-time Stream Merging...")
	audioByte1, _ := os.ReadFile(audiofile1)
	audioByte2, _ := os.ReadFile(audiofile2)

	// 使用两路独立配置的 mergeConfig
	engine := audiogo.NewAudioEngine(audiogo.Stream, mergeConfig)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		log.Fatalf("Failed to start: %v", err)
	}
	defer engine.Done()

	var wgWriter sync.WaitGroup // 专门用于追踪写入协程
	errChan := make(chan error, 10)

	writeFunc := func(data []byte, isPrimary bool) {
		defer wgWriter.Done() // 写入完成减 1
		ticker := time.NewTicker(time.Duration(tickerInterval) * time.Millisecond)
		defer ticker.Stop()

		remaining := data
		for len(remaining) > 0 {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n := min(chunkByteLen, len(remaining))
				var err error
				if isPrimary {
					err = engine.WritePrimary(remaining[:n])
				} else {
					err = engine.WriteSecondary(remaining[:n])
				}
				if err != nil {
					// 如果是因为 CloseInput 导致的关闭，忽略错误，否则报错
					if !errors.Is(err, os.ErrClosed) && !strings.Contains(err.Error(), "closed") {
						errChan <- fmt.Errorf("write error: %v", err)
					}
					return
				}
				remaining = remaining[n:]
			}
		}
	}

	wgWriter.Add(2)
	go writeFunc(audioByte1, true)
	go writeFunc(audioByte2, false)

	go func() {
		wgWriter.Wait()
		engine.CloseInput()
	}()
	outfile, _ := os.Create(audioStereoFile)
	defer outfile.Close()
	readCount := 0

	var wgReader sync.WaitGroup
	wgReader.Add(1)
	go func() {
		defer wgReader.Done()
		for {
			pBuf := bufferPool.Get().(*[]byte)
			n, err := engine.ReadLeft(*pBuf)
			if n > 0 {
				outfile.Write((*pBuf)[:n])
				readCount += n
			}
			bufferPool.Put(pBuf)
			if err != nil {
				break
			}
		}
	}()

	// 最终等待
	wgReader.Wait()
	close(errChan)

	if err := engine.Wait(); err != nil {
		log.Fatalf("Merge failed: %v", err)
	}

	for err := range errChan {
		log.Fatalf("Runtime Error: %v", err)
	}
	log.Printf("Merge Result: %d bytes saved to %s", readCount, audioStereoFile)
}

func runStreamConvertExample() {
	pcmByte, err := os.ReadFile(testInputFile)
	if err != nil {
		log.Fatal("Skipping test: test input file not found")
	}

	engine := audiogo.NewAudioEngine(audiogo.Stream, testConfig)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		log.Fatalf("Failed to start: %v", err)
	}
	defer engine.Done()

	errChan := make(chan error, 2)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer engine.CloseInput()
		ticker := time.NewTicker(time.Duration(tickerInterval) * time.Millisecond)
		defer ticker.Stop()

		remaining := pcmByte
		for len(remaining) > 0 {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n := min(chunkByteLen, len(remaining))
				if err := engine.WritePrimary(remaining[:n]); err != nil {
					errChan <- err
					return
				}
				remaining = remaining[n:]
			}
		}
	}()

	outfile, _ := os.Create(outInputFile)
	defer outfile.Close()
	readCount := 0
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			pBuf := bufferPool.Get().(*[]byte)
			n, err := engine.ReadLeft(*pBuf)
			if n > 0 {
				outfile.Write((*pBuf)[:n])
				readCount += n
			}
			bufferPool.Put(pBuf)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					errChan <- err
				}
				break
			}
		}
	}()

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for err := range errChan {
		log.Fatalf("Runtime error: %v", err)
	}

	if err := engine.Wait(); err != nil {
		log.Fatalf("FFmpeg engine failed: %v", err)
	}
	log.Printf("Convert completed: %d bytes", readCount)
}

// runFileConvertExample 演示如何转换本地磁盘文件
func runFileConvertExample() {
	log.Println("\n[Example] Offline File Conversion...")

	testConfig.InputFiles = []string{testInputFile}
	testConfig.OutputFiles = []string{outInputFile}

	engine := audiogo.NewAudioEngine(audiogo.File, testConfig)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		log.Fatalf("Engine start failed: %v", err)
	}

	if err := engine.Wait(); err != nil {
		log.Fatalf("Conversion failed: %v", err)
	}

	// Verify output exists and has content
	if info, err := os.Stat(outInputFile); err != nil || info.Size() == 0 {
		log.Fatal("Output file invalid or empty")
	}
	log.Printf("File conversion successful: %s", outInputFile)
}

func runStreamSplitExample() {
	log.Println("\n[Example] stream channel split...")
	stereoByte, err := os.ReadFile(stereoFile)
	if err != nil {
		log.Fatal("Skipping test: test input file not found")
	}

	engine := audiogo.NewAudioEngine(audiogo.Stream, splitConfig)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		log.Fatalf("Failed to start: %v", err)
	}
	defer engine.Done()
	errChan := make(chan error, 3)
	var wg sync.WaitGroup
	wg.Add(3) // 1 write, 2 reads

	// 1. Write stereo data
	go func() {
		ticker := time.NewTicker(time.Duration(tickerInterval) * time.Millisecond)
		defer ticker.Stop()
		defer wg.Done()
		defer engine.CloseInput()
		remaining := stereoByte
		for len(remaining) > 0 {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n := min(chunkByteLen, len(remaining))
				if err := engine.WritePrimary(remaining[:n]); err != nil {
					errChan <- err
					return
				}
				remaining = remaining[n:]
			}
		}
	}()
	leftfile, _ := os.Create(leftMonoFile)
	defer leftfile.Close()
	rightfile, _ := os.Create(rightMonoFile)
	defer rightfile.Close()

	readLeft := 0
	go func() {
		defer wg.Done()
		for {
			pBuf := bufferPool.Get().(*[]byte)
			n, err := engine.ReadLeft(*pBuf)
			if n > 0 {
				leftfile.Write((*pBuf)[:n])
				readLeft += n
			}
			bufferPool.Put(pBuf)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					errChan <- err
				}
				break
			}
		}
	}()

	// 3. Read Right channel
	readRight := 0
	go func() {
		defer wg.Done()
		for {
			pBuf := bufferPool.Get().(*[]byte)
			n, err := engine.ReadRight(*pBuf)
			if n > 0 {
				rightfile.Write((*pBuf)[:n])
				readRight += n
			}
			bufferPool.Put(pBuf)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					errChan <- err
				}
				break
			}
		}
	}()
	go func() {
		wg.Wait()
		close(errChan)
	}()
	for err := range errChan {
		if err != nil {
			log.Fatalf("Coroutine error: %v", err)
		}
	}
	if err := engine.Wait(); err != nil {
		log.Fatalf("Engine exit status: %v", err)
	}
	log.Printf("Split completed: Left %d bytes, Right %d bytes", readLeft, readRight)
	if readLeft == 0 || readRight == 0 {
		log.Fatalf("Data loss in split: L=%d, R=%d", readLeft, readRight)
	}
}

func runFileSplitExample() {
	log.Println("\n[Example] file channel split...")
	splitConfig.InputFiles = []string{stereoFile}
	splitConfig.OutputFiles = []string{leftMonoFile, rightMonoFile}

	engine := audiogo.NewAudioEngine(audiogo.File, splitConfig)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		log.Fatalf("Engine start failed: %v", err)
	}

	if err := engine.Wait(); err != nil {
		log.Fatalf("Split failed: %v", err)
	}

	// Validation
	for _, f := range []string{leftMonoFile, rightMonoFile} {
		if info, err := os.Stat(f); err != nil || info.Size() == 0 {
			log.Fatalf("Output file %s missing or empty", f)
		}
	}
	log.Println("File channel split successful")
}

func runFileMergeExample() {
	log.Println("\n[Example] file channel merge...")
	mergeConfig.InputFiles = []string{audiofile1, audiofile2}
	mergeConfig.OutputFiles = []string{audioStereoFile}

	engine := audiogo.NewAudioEngine(audiogo.File, mergeConfig)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		log.Fatalf("Engine start failed: %v", err)
	}

	if err := engine.Wait(); err != nil {
		log.Fatalf("Merge failed: %v", err)
	}

	if info, err := os.Stat(audioStereoFile); err != nil || info.Size() == 0 {
		log.Fatal("Merged file invalid")
	}
	log.Printf("File merge successful: %s", audioStereoFile)
}
