package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

// Result represents the outcome of a playlist download.
type downloadResult struct {
	Index   int
	Content string
	Error   error
}

// FetchPlaylists reads sources.txt and downloads each playlist content.
func FetchPlaylists(sourcesPath string) ([]string, error) {
	file, err := os.Open(sourcesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sources file: %w", err)
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		url := strings.TrimSpace(scanner.Text())
		if url != "" && !strings.HasPrefix(url, "#") {
			urls = append(urls, url)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading sources file: %w", err)
	}

	total := len(urls)
	playlists := make([]string, total)
	results := make(chan downloadResult, total)
	var wg sync.WaitGroup

	for i, url := range urls {
		wg.Add(1)
		go func(index int, u string) {
			defer wg.Done()
			fmt.Printf("Fetching playlist [%d/%d]: %s\n", index+1, total, u)
			content, err := downloadURL(u)
			results <- downloadResult{Index: index, Content: content, Error: err}
		}(i, url)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	successCount := 0
	for res := range results {
		if res.Error != nil {
			fmt.Printf("Warning: failed to download playlist at index %d: %v\n", res.Index, res.Error)
			continue
		}
		playlists[res.Index] = res.Content
		successCount++
	}

	// Filter out empty strings (failed downloads)
	var filtered []string
	for _, p := range playlists {
		if p != "" {
			filtered = append(filtered, p)
		}
	}

	fmt.Printf("Successfully fetched %d/%d playlists.\n", successCount, total)
	return filtered, nil
}

func downloadURL(url string) (string, error) {
	resp, err := http.Get(url)
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
