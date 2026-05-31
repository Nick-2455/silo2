package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/nicolasperalta/silo2/internal/config"
	"github.com/nicolasperalta/silo2/internal/engram"
	"github.com/nicolasperalta/silo2/internal/obsidian"
	"github.com/nicolasperalta/silo2/internal/synthesis"
)

// `silo import-playlist <url>` imports a YouTube playlist into individual
// video Seeds so each video can be triaged and tracked independently.
//
// Implementation note: We intentionally shell out to yt-dlp for extraction.
// YouTube pages are highly dynamic; yt-dlp is the stable boundary.

type playlistItem struct {
	VideoID string
	Title   string
}

func runImportPlaylist(args []string) error {
	fs := flag.NewFlagSet("import-playlist", flag.ContinueOnError)
	projectFlag := fs.String("project", "", "Engram project (overrides config.project)")
	limitFlag := fs.Int("limit", 0, "Max videos to import (0 = all)")
	whyFlag := fs.String("why", "", "Capture why (applied to every imported video)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	pos := fs.Args()
	if len(pos) == 0 {
		return errors.New("silo import-playlist: missing playlist URL\n\nUsage: silo import-playlist <url> [--limit N] [--why \"...\"] [--project <name>]")
	}
	playlistURL := strings.TrimSpace(pos[0])
	if playlistURL == "" {
		return errors.New("playlist URL is empty")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.VaultPath) == "" {
		return errors.New("vault_path is empty in config")
	}
	project := resolveProject(*projectFlag, cfg.Project)

	items, err := extractPlaylistItems(playlistURL, *limitFlag)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Println("no videos found")
		return nil
	}

	v := obsidian.NewVault(cfg.VaultPath)
	if err := v.EnsureDir(); err != nil {
		return err
	}
	if err := ensureInboxLayout(v); err != nil {
		return err
	}

	deps := saveDeps{
		Client:  engram.NewClient(cfg),
		Synth:   synthesis.NewFallback(),
		Vault:   v,
		Timeout: 2 * time.Second,
		Stdout:  io.Discard,
		Stderr:  os.Stderr,
	}

	ctx := context.Background()
	var imported int
	for i, it := range items {
		text := strings.TrimSpace(it.Title)
		if text == "" {
			text = "YouTube video " + it.VideoID
		}
		// Keep the observation itself minimal; the canonical link lives in
		// the seed Sources section.
		_, err := saveCore(ctx, deps, saveInput{
			Project:    project,
			Text:       text,
			Why:        strings.TrimSpace(*whyFlag),
			SourceURL:  youtubeWatchURL(it.VideoID),
			SourceType: "video",
		})
		if err != nil {
			return fmt.Errorf("import item %d/%d (%s): %w", i+1, len(items), it.VideoID, err)
		}
		imported++
	}

	fmt.Printf("imported videos: %d\n", imported)
	fmt.Printf("vault: %s\n", cfg.VaultPath)
	fmt.Printf("next: run `silo videos` to regenerate the global watch later list\n")
	return nil
}

func extractPlaylistItems(url string, limit int) ([]playlistItem, error) {
	// yt-dlp is our supported extraction engine.
	bin, err := exec.LookPath("yt-dlp")
	if err != nil {
		return nil, errors.New("yt-dlp not found in PATH; install it (brew install yt-dlp) to use import-playlist")
	}

	// We use --flat-playlist for speed and stability. Output is one line per
	// entry: <id>\t<title>
	args := []string{"--flat-playlist"}
	if limit > 0 {
		args = append(args, "--playlist-end", strconv.Itoa(limit))
	}
	args = append(args, "--print", "%(id)s\t%(title)s", url)

	cmd := exec.Command(bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var out []playlistItem
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		id, title, ok := splitTab(line)
		if !ok {
			continue
		}
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		out = append(out, playlistItem{VideoID: id, Title: strings.TrimSpace(title)})
	}
	if err := sc.Err(); err != nil {
		_ = cmd.Wait()
		return nil, err
	}
	if err := cmd.Wait(); err != nil {
		return nil, err
	}
	return out, nil
}

func splitTab(s string) (a, b string, ok bool) {
	i := strings.IndexByte(s, '\t')
	if i < 0 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

func youtubeWatchURL(videoID string) string {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return ""
	}
	return "https://www.youtube.com/watch?v=" + videoID
}
