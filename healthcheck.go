package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Result represents the outcome of a health check.
type Result struct {
	Entry   StreamEntry
	Success bool
}

// RunHealthChecks performs concurrent health checks on a list of stream entries.
func RunHealthChecks(entries []StreamEntry, workerCount int) []StreamEntry {
	var wg sync.WaitGroup
	jobs := make(chan StreamEntry, len(entries))
	results := make(chan Result, len(entries))

	// Start workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go worker(&wg, jobs, results)
	}

	// Push jobs
	for _, entry := range entries {
		jobs <- entry
	}
	close(jobs)

	// Wait for workers in a separate goroutine
	go func() {
		wg.Wait()
		close(results)
	}()

	var healthyEntries []StreamEntry
	count := 0
	invalidURLs := 0
	deadStreams := 0

	for res := range results {
		count++
		if res.Success {
			healthyEntries = append(healthyEntries, res.Entry)
		} else {
			if !strings.HasPrefix(res.Entry.URL, "http") {
				invalidURLs++
			} else {
				deadStreams++
			}
		}
	}

	fmt.Printf("Health check completed: %d total, %d healthy, %d dead, %d invalid URLs removed.\n",
		count, len(healthyEntries), deadStreams, invalidURLs)

	return healthyEntries
}

func worker(wg *sync.WaitGroup, jobs <-chan StreamEntry, results chan<- Result) {
	defer wg.Done()
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for entry := range jobs {
		// Basic URL validation
		if !strings.HasPrefix(entry.URL, "http://") && !strings.HasPrefix(entry.URL, "https://") {
			results <- Result{Entry: entry, Success: false}
			continue
		}

		start := time.Now()
		req, _ := http.NewRequest("GET", entry.URL, nil)
		// Set a standard User-Agent to avoid blocks
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

		resp, err := client.Do(req)
		latency := time.Since(start).Milliseconds()

		if err != nil {
			results <- Result{Entry: entry, Success: false}
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			results <- Result{Entry: entry, Success: false}
			continue
		}

		// Read first 1024 bytes to check for HLS tags
		buf := make([]byte, 1024)
		n, _ := io.ReadAtLeast(resp.Body, buf, 1) // Read at least 1 byte
		bodyPrefix := string(buf[:n])

		// Relaxed validation: EXTM3U is required.
		// Then either EXTINF (media playlist) or EXT-X-STREAM-INF (master playlist)
		isHLS := strings.Contains(bodyPrefix, "#EXTM3U") &&
			(strings.Contains(bodyPrefix, "#EXTINF") || strings.Contains(bodyPrefix, "#EXT-X-STREAM-INF"))

		if isHLS {
			entry.Latency = latency
			results <- Result{Entry: entry, Success: true}
		} else {
			results <- Result{Entry: entry, Success: false}
		}
	}
}
