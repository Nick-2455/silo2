package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type videoMeta struct {
	Title       string
	Channel     string
	DurationSec int
	Description string
}

// extractVideoMeta uses yt-dlp to fetch metadata for a single video URL.
// Best-effort: callers should treat errors as non-fatal.
func extractVideoMeta(ctx context.Context, url string) (videoMeta, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return videoMeta{}, errors.New("url is empty")
	}
	bin, err := exec.LookPath("yt-dlp")
	if err != nil {
		return videoMeta{}, errors.New("yt-dlp not found in PATH")
	}

	// --dump-single-json is the most stable shape. --no-playlist ensures we
	// don't accidentally expand a playlist URL.
	args := []string{"--dump-single-json", "--no-warnings", "--no-playlist", url}
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.Output()
	if err != nil {
		return videoMeta{}, fmt.Errorf("yt-dlp: %w", err)
	}

	var payload struct {
		Title       string `json:"title"`
		Uploader    string `json:"uploader"`
		Channel     string `json:"channel"`
		Duration    int    `json:"duration"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return videoMeta{}, fmt.Errorf("parse yt-dlp json: %w", err)
	}
	ch := strings.TrimSpace(payload.Channel)
	if ch == "" {
		ch = strings.TrimSpace(payload.Uploader)
	}
	return videoMeta{
		Title:       strings.TrimSpace(payload.Title),
		Channel:     ch,
		DurationSec: payload.Duration,
		Description: strings.TrimSpace(payload.Description),
	}, nil
}

func extractVideoMetaBestEffort(url string) (videoMeta, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return extractVideoMeta(ctx, url)
}
