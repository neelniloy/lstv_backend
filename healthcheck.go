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
		// Update stability and get new score
		res.Entry.Stability = UpdateStability(res.Entry.URL, res.Success)

		if res.Success && res.Entry.Stability >= 0.5 {
			healthyEntries = append(healthyEntries, res.Entry)
		} else {
			if !strings.HasPrefix(res.Entry.URL, "http") {
				invalidURLs++
			} else {
				deadStreams++
			}
		}
	}

	fmt.Printf("Health check completed: %d total, %d healthy/stable, %d dead/unstable, %d invalid URLs removed.\n",
		count, len(healthyEntries), deadStreams, invalidURLs)

	return healthyEntries
}

func worker(wg *sync.WaitGroup, jobs <-chan StreamEntry, results chan<- Result) {
	defer wg.Done()
	client := &http.Client{
		Timeout: 7 * time.Second, // Increased slightly for segment test
	}

	for entry := range jobs {
		// Basic URL validation
		if !strings.HasPrefix(entry.URL, "http://") && !strings.HasPrefix(entry.URL, "https://") {
			results <- Result{Entry: entry, Success: false}
			continue
		}

		// Detect Type
		lowerURL := strings.ToLower(entry.URL)
		if entry.Type == "" {
			if strings.Contains(lowerURL, ".m3u8") {
				entry.Type = "hls"
			} else if strings.Contains(lowerURL, ".mpd") {
				entry.Type = "dash"
			} else if strings.Contains(lowerURL, ".ts") {
				entry.Type = "mpegts"
			}
		}

		start := time.Now()
		req, _ := http.NewRequest("GET", entry.URL, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

		resp, err := client.Do(req)
		// TTFB = Time from request start to header response
		ttfb := time.Since(start).Milliseconds()

		if err != nil {
			results <- Result{Entry: entry, Success: false}
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			results <- Result{Entry: entry, Success: false}
			continue
		}

		// Read body to check size and content
		body, err := io.ReadAll(resp.Body)
		if err != nil || len(body) < 50 {
			results <- Result{Entry: entry, Success: false}
			continue
		}
		bodyStr := string(body)

		// Validation based on type
		success := false
		if entry.Type == "hls" || strings.Contains(bodyStr, "#EXTM3U") {
			entry.Type = "hls"
			// Must contain EXTM3U AND at least one stream/inf tag
			success = strings.Contains(bodyStr, "#EXTM3U") &&
				(strings.Contains(bodyStr, "#EXTINF") || strings.Contains(bodyStr, "#EXT-X-STREAM-INF"))
			
			// Optional Segment Test for HLS
			if success {
				segmentURL := extractFirstSegment(bodyStr, entry.URL)
				if segmentURL != "" {
					segReq, _ := http.NewRequest("GET", segmentURL, nil)
					segReq.Header.Set("User-Agent", req.Header.Get("User-Agent"))
					segResp, segErr := client.Do(segReq)
					if segErr != nil || segResp.StatusCode != http.StatusOK {
						success = false // Segment test failed
					} else {
						// Body check for segment (shouldn't be an error page)
						segResp.Body.Close()
					}
				}
			}
		} else if entry.Type == "dash" || strings.Contains(bodyStr, "<MPD") {
			entry.Type = "dash"
			success = strings.Contains(bodyStr, "<MPD")
		} else if entry.Type == "mpegts" {
			success = true
		} else {
			success = true // Fallback
		}

		if success {
			entry.Latency = ttfb
			results <- Result{Entry: entry, Success: true}
		} else {
			results <- Result{Entry: entry, Success: false}
		}
	}
}

// extractFirstSegment tries to find the first media segment in an M3U8 file.
func extractFirstSegment(body string, baseURL string) string {
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			// Found a URL or relative path
			if strings.HasPrefix(line, "http") {
				return line
			}
			// Relative path logic (simple)
			lastSlash := strings.LastIndex(baseURL, "/")
			if lastSlash != -1 {
				return baseURL[:lastSlash+1] + line
			}
			return line
		}
	}
	return ""
}
