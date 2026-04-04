package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var playlistClient = &http.Client{Timeout: 30 * time.Second}

// Result represents the outcome of a playlist download.
type downloadResult struct {
	Index   int
	Content string
	Error   error
}

// FetchPlaylists reads sourcesPath and downloads or reads each playlist content.
func FetchPlaylists(sourcesPath string) ([]string, error) {
	file, err := os.Open(sourcesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sources file: %w", err)
	}
	defer file.Close()

	var sources []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			sources = append(sources, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading sources file: %w", err)
	}

	total := len(sources)
	playlists := make([]string, total)
	results := make(chan downloadResult, total)
	var wg sync.WaitGroup

	for i, source := range sources {
		wg.Add(1)
		go func(index int, s string) {
			defer wg.Done()
			var content string
			var err error

			if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
				fmt.Printf("Fetching playlist [%d/%d]: %s\n", index+1, total, s)
				content, err = downloadURL(s)
			} else {
				fmt.Printf("Reading local playlist [%d/%d]: %s\n", index+1, total, s)
				content, err = readLocalFile(s)
			}

			results <- downloadResult{Index: index, Content: content, Error: err}
		}(i, source)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	successCount := 0
	for res := range results {
		if res.Error != nil {
			fmt.Printf("Warning: failed to load playlist at index %d: %v\n", res.Index, res.Error)
			continue
		}
		playlists[res.Index] = res.Content
		successCount++
	}

	// Filter out empty strings (failed loads)
	var filtered []string
	for _, p := range playlists {
		if p != "" {
			filtered = append(filtered, p)
		}
	}

	fmt.Printf("Successfully loaded %d/%d playlists.\n", successCount, total)
	return filtered, nil
}

func downloadURL(url string) (string, error) {
	resp, err := playlistClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func readLocalFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read local file %s: %w", path, err)
	}
	return string(content), nil
}
