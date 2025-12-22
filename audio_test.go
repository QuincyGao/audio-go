package audiogo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuincyGao/audio-go/formats"
)

const (
	testInputFile   = "./example/sample_data/convert.pcm"
	outInputFile    = "./example/sample_data/out-one-8kHz.pcm"
	stereoFile      = "./example/sample_data/audio-stereo.mp3"
	leftMonoFile    = "./example/sample_data/out-left.pcm"
	rightMonoFile   = "./example/sample_data/out-right.pcm"
	audiofile1      = "./example/sample_data/audio-8kHz.wav"
	audiofile2      = "./example/sample_data/audio-24kHz.mp3"
	audioStereoFile = "./example/sample_data/out-stereo.wav"
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

// TestStreamFormatConvert
func TestStreamFormatConvert(t *testing.T) {
	pcmByte, err := os.ReadFile(testInputFile)
	if err != nil {
		t.Skip("Skipping test: test input file not found")
	}

	engine := NewAudioEngine(Stream, testConfig)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Failed to start: %v", err)
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
		t.Errorf("Runtime error: %v", err)
	}

	if err := engine.Wait(); err != nil {
		t.Errorf("FFmpeg engine failed: %v", err)
	}
	t.Logf("Convert completed: %d bytes", readCount)
}

// TestFileFormatConvert tests offline file conversion
func TestFileFormatConvert(t *testing.T) {
	testConfig.InputFiles = []string{testInputFile} // Using constant from your stream test
	testConfig.OutputFiles = []string{outInputFile}

	engine := NewAudioEngine(File, testConfig)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Engine start failed: %v", err)
	}

	if err := engine.Wait(); err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// Verify output exists and has content
	if info, err := os.Stat(outInputFile); err != nil || info.Size() == 0 {
		t.Errorf("Output file invalid or empty")
	}
	t.Logf("File conversion successful: %s", outInputFile)
}

// TestStreamChannelSplit tests channel splitting (Stereo -> Left/Right Mono)
func TestStreamChannelSplit(t *testing.T) {
	stereoByte, err := os.ReadFile(stereoFile)
	if err != nil {
		t.Skip("Skipping test: test input file not found")
	}

	engine := NewAudioEngine(Stream, splitConfig)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Failed to start: %v", err)
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
			t.Errorf("Coroutine error: %v", err)
		}
	}
	if err := engine.Wait(); err != nil {
		t.Logf("Engine exit status: %v", err)
	}
	t.Logf("Split completed: Left %d bytes, Right %d bytes", readLeft, readRight)
	if readLeft == 0 || readRight == 0 {
		t.Errorf("Data loss in split: L=%d, R=%d", readLeft, readRight)
	}
}

// TestFileChannelSplit tests splitting an MP3 file into two Mono PCM files
func TestFileChannelSplit(t *testing.T) {
	splitConfig.InputFiles = []string{stereoFile}
	splitConfig.OutputFiles = []string{leftMonoFile, rightMonoFile}

	engine := NewAudioEngine(File, splitConfig)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Engine start failed: %v", err)
	}

	if err := engine.Wait(); err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	// Validation
	for _, f := range []string{leftMonoFile, rightMonoFile} {
		if info, err := os.Stat(f); err != nil || info.Size() == 0 {
			t.Errorf("Output file %s missing or empty", f)
		}
	}
	t.Log("File channel split successful")
}

func TestStreamChannelMerge(t *testing.T) {
	audioByte1, _ := os.ReadFile(audiofile1)
	audioByte2, _ := os.ReadFile(audiofile2)

	engine := NewAudioEngine(Stream, mergeConfig)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Failed to start: %v", err)
	}
	defer engine.Done()

	var wgWriter sync.WaitGroup
	errChan := make(chan error, 10)

	writeFunc := func(data []byte, isPrimary bool) {
		defer wgWriter.Done()
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

	wgReader.Wait()
	close(errChan)

	if err := engine.Wait(); err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	for err := range errChan {
		t.Errorf("Runtime Error: %v", err)
	}
	t.Logf("Merge Result: %d bytes saved to %s", readCount, audioStereoFile)
}

// TestFileAudioMerge tests merging two mono files into one stereo WAV
func TestFileAudioMerge(t *testing.T) {
	mergeConfig.InputFiles = []string{audiofile1, audiofile2}
	mergeConfig.OutputFiles = []string{audioStereoFile}

	engine := NewAudioEngine(File, mergeConfig)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Engine start failed: %v", err)
	}

	if err := engine.Wait(); err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if info, err := os.Stat(audioStereoFile); err != nil || info.Size() == 0 {
		t.Errorf("Merged file invalid")
	}
	t.Logf("File merge successful: %s", audioStereoFile)
}
