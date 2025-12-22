<div align="center" style="text-decoration: none;">
  <img width="100%" src="assets/audio-go.png" alt="audio-go">
  <p style="font-size: 1.3em; font-weight: 600; margin-bottom: 20px;">
    <a href="README_zh.md"> 简体中文 </a>
    |
    <a href="README.md"> English </a>
  </p>
</div>

</div>

# audio-go

`audio-go` 是一个基于 Go 语言和 FFmpeg 构建的轻量级音频处理框架。它旨在为开发者提供一种简单、直观的方式来处理**实时流式音频**（如 WebSocket 语音流、实时转码）和**离线音频文件**。

通过高度抽象的 API，你可以轻松实现音频格式转换、声道拆分以及多路流实时合成，而无需深陷 FFmpeg 复杂的命令行参数中。

---

## ✨ 核心特性

* **双模式架构**：
  * **Stream 模式**：基于 `io.Pipe` 管道通信，专为低延迟、高并发的实时场景设计。
  * **File 模式**：调用 FFmpeg 高效处理磁盘文件，支持批量转换。
* **多功能工具箱**：
  * **Format Convert**：任意音频格式间的相互转换。
  * **Channel Split**：将立体声（Stereo）拆分为独立的左/右单声道。
  * **Audio Merge**：将多路单声道流合成，支持“并列左右耳”（SideBySide）或“混音”（Mix）模式。
* **智能重采样**：内置 `aresample` 滤镜，支持在处理过程中自动对齐采样率、声道数和编码格式。
* **健壮的错误处理**：自动捕获 FFmpeg 的 `stderr` 输出，并将其包装为 Go 标准错误，方便排查由于音频损坏或参数错误引起的问题。

---

## 🚀 快速开始

### 前置条件

1. **Go**: 1.20+
2. **FFmpeg**: 系统需预装 FFmpeg 动态库或二进制文件，并确保 `ffmpeg` 在环境变量 `PATH` 中，[ffmpeg 官方下载地址](https://ffmpeg.org/download.html)。

### 安装

```bash
go get https://github.com/QuincyGao/audio-go
```

## 🛠 功能示例


请参考example/main.go,  说明：

1.  



## 📐 逻辑架构

`audio-go` 通过管理标准输入 (`stdin`)、标准输出 (`stdout`) 以及扩展文件描述符 (`ExtraFiles`) 来实现 Go 与 FFmpeg 之间的数据交换。


| **功能模式** | **输入通道 (Go -> FFmpeg)** | **输出通道 (FFmpeg -> Go)** | **应用场景**                 |
| ------------ | --------------------------- | --------------------------- | ---------------------------- |
| **Convert**  | `pipe:0`(Primary)           | `pipe:1`(Left)              | 实时转码、采样率转换         |
| **Split**    | `pipe:0`(Primary)           | `pipe:1`,`pipe:3`           | 声道分离（如提取左声道）     |
| **Merge**    | `pipe:0`,`pipe:3`           | `pipe:1`(Left)              | 实时语音对讲合流、背景音叠加 |

## ⚙️ 核心配置 (AudioArgs)

配置项支持切片形式，可以为每一路输入/输出流单独指定参数。

* **AudioFileFormat**: 支持 `WAV`, `MP3`, `AAC`, `S16LE` (Raw PCM) 等。
* **SampleRate**: 支持任意采样率（内置自动重采样）。
* **Channels**: 支持单声道 (1) 与立体声 (2) 之间的转换。

## 🤝 贡献与反馈

如果你在使用过程中发现任何问题，或者有功能改进建议，欢迎提交 Issue 或 Pull Request。

## 📄 开源协议

本项目基于 [MIT](https://www.google.com/search?q=LICENSE) 协议开源。
