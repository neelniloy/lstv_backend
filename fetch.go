package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// FetchPlaylists reads sources.txt and downloads each playlist content.
func FetchPlaylists(sourcesPath string) ([]string, error) {
	file, err := os.Open(sourcesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sources file: %w", err)
	}
	defer file.Close()

	var playlists []string
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

	for _, url := range urls {
		fmt.Printf("Fetching playlist: %s\n", url)
		content, err := downloadURL(url)
		if err != nil {
			fmt.Printf("Warning: failed to download %s: %v\n", url, err)
			continue
		}
		playlists = append(playlists, content)
	}

	fmt.Printf("Successfully fetched %d playlists.\n", len(playlists))
	return playlists, nil
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
