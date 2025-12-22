<div align="center" style="text-decoration: none;">
  <img width="100%" src="assets/audio-go.png" alt="audio-go">
  <p style="font-size: 1.3em; font-weight: 600; margin-bottom: 20px;">
    <a href="README_zh.md"> 简体中文 </a>
    |
    <a href="README.md"> English </a>
  </p>
</div>

# audio-go

`audio-go` is a lightweight audio processing framework built on Go and FFmpeg. It is designed to provide developers with a simple and intuitive way to handle **real-time streaming audio** (e.g., WebSocket voice streams, real-time transcoding) and **offline audio files**.

With a highly abstracted API, you can easily implement audio format conversion, channel splitting, and multi-stream real-time synthesis without getting bogged down in complex FFmpeg command-line arguments.

## ✨ Key Features

* **Dual-Mode Architecture**:
  * **Stream Mode**: Powered by `io.Pipe` communication, specifically designed for low-latency, high-concurrency real-time scenarios.
  * **File Mode**: Leverages FFmpeg to efficiently process disk files with support for batch conversion.
* **Multifunctional Toolbox**:
  * **Format Convert**: Seamless conversion between various audio formats.
  * **Channel Split**: Split stereo audio into independent Left/Right mono channels.
  * **Audio Merge**: Synthesize multiple mono streams, supporting "Side-by-Side" (Stereo mapping) or "Mix" modes.
* **Intelligent Resampling**: Built-in `aresample` filter to automatically align sample rates, channel counts, and encoding formats during processing.
* **Robust Error Handling**: Automatically captures FFmpeg `stderr` output and wraps it into standard Go errors, making it easy to debug issues caused by corrupted audio or parameter mismatches.

## 🚀 Quick Start

### Prerequisites

1. **Go**: 1.20+
2. **FFmpeg**: Ensure FFmpeg dynamic libraries or binaries are pre-installed and `ffmpeg` is in your system `PATH`. [FFmpeg Official Download](https://ffmpeg.org/download.html).

### Installation

**Bash**

```
go get github.com/QuincyGao/audio-go
```

## 🛠 Usage Examples

### 1. Real-time Streaming: Multi-Stream Synthesis (Stream Merge)

This example simulates merging two users' mono PCM raw streams into one real-time output.

**Go**

```
package main

import (
    "context"
    "github.com/QuincyGao/audio-go"
    "github.com/QuincyGao/audio-go/formats"
)

func main() {
    // Configuration: 2 Mono inputs -> 1 Stereo output
    cfg := formats.AudioConfig{
        OpType:    formats.AUDIOMERGE,
        MergeMode: formats.SideBySide, // User A to Left, User B to Right
        InputArgs: []formats.AudioArgs{
            {AudioFileFormat: formats.S16LE, SampleRate: 16000, Channels: 1},
        },
        OutputArgs: []formats.AudioArgs{
            {AudioFileFormat: formats.WAV, SampleRate: 16000, Channels: 2},
        },
    }

    engine := audiogo.NewAudioEngine(audiogo.Stream, cfg)
    engine.Start(context.Background())

    // Simulated concurrent writes: WritePrimary (pipe:0), WriteSecondary (pipe:3)
    go engine.WritePrimary(pcmDataA)
    go engine.WriteSecondary(pcmDataB)

    // Main loop: Read the synthesized WAV stream from the engine
    buf := make([]byte, 4096)
    for {
        n, err := engine.ReadLeft(buf) 
        if err != nil { break }
        // Process or forward synthesized data (buf[:n])
    }
}
```

### 2. Offline Processing: Format Conversion (File Convert)

Convert a high-resolution WAV file into a low-sample-rate mono MP3.

**Go**

```
cfg := formats.AudioConfig{
    OpType: formats.FORMATCONVERT,
    InputFiles:  []string{"high_res.wav"},
    OutputFiles: []string{"low_res.mp3"},
    OutputArgs:  []formats.AudioArgs{
        {AudioFileFormat: formats.MP3, SampleRate: 16000, Channels: 1},
    },
}

engine := audiogo.NewAudioEngine(audiogo.File, cfg)
if err := engine.Start(context.Background()); err == nil {
    // Wait will block until the FFmpeg process completes
    engine.Wait() 
}
```

---

## 📐 Logical Architecture

`audio-go` manages communication between Go and FFmpeg processes by handling Standard Input (`stdin`), Standard Output (`stdout`), and Extended File Descriptors (`ExtraFiles`).


| **Mode**    | **Input Channels (Go -> FFmpeg)** | **Output Channels (FFmpeg -> Go)** | **Use Case**                                       |
| ----------- | --------------------------------- | ---------------------------------- | -------------------------------------------------- |
| **Convert** | `pipe:0`(Primary)                 | `pipe:1`(Left)                     | Real-time transcoding, resampling                  |
| **Split**   | `pipe:0`(Primary)                 | `pipe:1`,`pipe:3`                  | Channel separation (e.g., extracting Left channel) |
| **Merge**   | `pipe:0`,`pipe:3`                 | `pipe:1`(Left)                     | Voice intercom merging, BGM overlay                |

## ⚙️ Core Configuration (AudioArgs)

The configuration supports slices, allowing unique parameters to be specified for each input/output stream.

* **AudioFileFormat**: Supports `WAV`, `MP3`, `AAC`, `S16LE` (Raw PCM), and more.
* **SampleRate**: Supports any sample rate (Automatic resampling built-in).
* **Channels**: Supports conversion between Mono (1) and Stereo (2).

---

## 🤝 Contribution & Feedback

If you encounter any issues or have suggestions for new features, please feel free to submit an Issue or Pull Request.

1. Fork the project.
2. Create your feature branch (`git checkout -b feature/AmazingFeature`).
3. Commit your changes (`git commit -m 'feat: add new audio filter'`).
4. Open a Pull Request.

---

## 📄 License

This project is licensed under the [MIT License](https://www.google.com/search?q=LICENSE).
