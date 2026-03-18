package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	maxVideoBytes  = 200 * 1024 * 1024 // 200 MB
	maxFrames      = 600
	maxDurationSec = 600 // 10 minutes
)

var (
	// ErrVideoTooLarge is returned when video exceeds the 200MB limit.
	ErrVideoTooLarge = errors.New("ffmpeg: video exceeds 200MB limit")
	// ErrVideoDurationTooLong is returned when video exceeds 10 minutes.
	ErrVideoDurationTooLong = errors.New("ffmpeg: video exceeds 10-minute limit")
)

// FFmpegClient wraps ffmpeg/ffprobe subprocess calls for video processing.
type FFmpegClient struct{}

// NewFFmpegClient creates an FFmpegClient.
func NewFFmpegClient() *FFmpegClient { return &FFmpegClient{} }

// GetDurationSeconds returns the video duration in seconds using ffprobe.
func (f *FFmpegClient) GetDurationSeconds(ctx context.Context, videoBytes []byte) (float64, error) {
	tmpFile, err := writeTempFile(videoBytes, "brevio-video-*.mp4")
	if err != nil {
		return 0, err
	}
	defer os.Remove(tmpFile)

	cmd := exec.CommandContext(ctx,
		"ffprobe", "-v", "quiet", "-print_format", "json",
		"-show_format", tmpFile,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe: %w", err)
	}

	var result struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return 0, fmt.Errorf("ffprobe: parse: %w", err)
	}
	return strconv.ParseFloat(result.Format.Duration, 64)
}

// ExtractFrames extracts JPEG frames from video at the given fps.
// Returns at most maxFrameCount JPEG byte slices.
func (f *FFmpegClient) ExtractFrames(
	ctx context.Context,
	videoBytes []byte,
	fps float64,
	maxFrameCount int,
) ([][]byte, error) {
	if len(videoBytes) > maxVideoBytes {
		return nil, ErrVideoTooLarge
	}
	if fps <= 0 {
		fps = 1.0
	}
	if maxFrameCount <= 0 || maxFrameCount > maxFrames {
		maxFrameCount = maxFrames
	}

	// Validate duration.
	duration, err := f.GetDurationSeconds(ctx, videoBytes)
	if err == nil && duration > float64(maxDurationSec) {
		return nil, ErrVideoDurationTooLong
	}

	tmpIn, err := writeTempFile(videoBytes, "brevio-video-*.mp4")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpIn)

	tmpDir, err := os.MkdirTemp("", "brevio-frames-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	outputPattern := filepath.Join(tmpDir, "frame-%04d.jpg")
	cmd := exec.CommandContext(ctx,
		"ffmpeg", "-i", tmpIn,
		"-vf", fmt.Sprintf("fps=%g", fps),
		"-vframes", strconv.Itoa(maxFrameCount),
		"-q:v", "2",
		outputPattern,
	)
	cmd.Stderr = &bytes.Buffer{}
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg extract_frames: %w", err)
	}

	files, err := filepath.Glob(filepath.Join(tmpDir, "frame-*.jpg"))
	if err != nil {
		return nil, err
	}
	frames := make([][]byte, 0, len(files))
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		frames = append(frames, data)
	}
	return frames, nil
}

// ExtractAudio extracts the audio track to MP3 bytes.
// Returns (nil, nil) if the video has no audio track.
func (f *FFmpegClient) ExtractAudio(ctx context.Context, videoBytes []byte) ([]byte, error) {
	tmpIn, err := writeTempFile(videoBytes, "brevio-video-*.mp4")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpIn)

	tmpOut, err := os.CreateTemp("", "brevio-audio-*.mp3")
	if err != nil {
		return nil, err
	}
	tmpOut.Close()
	defer os.Remove(tmpOut.Name())

	cmd := exec.CommandContext(ctx,
		"ffmpeg", "-i", tmpIn,
		"-vn",
		"-acodec", "mp3",
		"-y",
		tmpOut.Name(),
	)
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if strings.Contains(errMsg, "no audio") ||
			strings.Contains(errMsg, "Output file is empty") ||
			strings.Contains(errMsg, "does not contain any stream") {
			return nil, nil // no audio track — not an error
		}
		return nil, fmt.Errorf("ffmpeg extract_audio: %w", err)
	}

	data, err := os.ReadFile(tmpOut.Name())
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil // empty audio
	}
	return data, nil
}

// WriteTempFileExported writes bytes to a temp file. Exported for cross-package use.
func WriteTempFileExported(data []byte, pattern string) (string, error) {
	return writeTempFile(data, pattern)
}

// writeTempFile writes bytes to a named temp file and returns the path.
func writeTempFile(data []byte, pattern string) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}
