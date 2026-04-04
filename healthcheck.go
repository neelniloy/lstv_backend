package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"

// logoCache avoids re-validating the same logo URL across multiple stream entries.
var logoCache sync.Map // url string -> bool (true = valid)

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
		// For skipped (recently-checked) entries, Stability is already set in the worker.
		// For checked entries, update stability from the result.
		if res.Entry.Stability == 0 {
			res.Entry.Stability = UpdateStability(res.Entry.URL, res.Success)
		}

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
	client := &http.Client{Timeout: 7 * time.Second}

	for entry := range jobs {
		// Basic URL validation
		if !strings.HasPrefix(entry.URL, "http://") && !strings.HasPrefix(entry.URL, "https://") {
			results <- Result{Entry: entry, Success: false}
			continue
		}

		// Detect type from URL if not set
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

		// Skip recently checked stable streams (only effective in daemon mode
		// where stability.json persists between runs)
		if ShouldSkipCheck(entry.URL) {
			entry.Stability = GetStability(entry.URL)
			results <- Result{Entry: entry, Success: true}
			continue
		}

		// Decide request method:
		// HLS/DASH manifests need body validation → GET
		// Everything else → HEAD (faster, no body download)
		needsBody := entry.Type == "hls" || entry.Type == "dash" ||
			strings.Contains(lowerURL, ".m3u8") || strings.Contains(lowerURL, ".mpd")

		start := time.Now()

		if !needsBody {
			success, ttfb := doHEAD(client, entry.URL)
			if success {
				entry.Latency = ttfb
				validateLogo(client, &entry)
				results <- Result{Entry: entry, Success: true}
			} else {
				results <- Result{Entry: entry, Success: false}
			}
			continue
		}

		// GET path — HLS / DASH
		req, err := http.NewRequest("GET", entry.URL, nil)
		if err != nil {
			results <- Result{Entry: entry, Success: false}
			continue
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err := client.Do(req)
		ttfb := time.Since(start).Milliseconds()

		if err != nil {
			results <- Result{Entry: entry, Success: false}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			results <- Result{Entry: entry, Success: false}
			continue
		}

		// Read body (capped at 64KB)
		body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		resp.Body.Close()
		if err != nil || len(body) < 50 {
			results <- Result{Entry: entry, Success: false}
			continue
		}
		bodyStr := string(body)

		success := false
		if entry.Type == "hls" || strings.Contains(bodyStr, "#EXTM3U") {
			entry.Type = "hls"
			success = strings.Contains(bodyStr, "#EXTM3U") &&
				(strings.Contains(bodyStr, "#EXTINF") || strings.Contains(bodyStr, "#EXT-X-STREAM-INF"))

			// Segment test for HLS
			if success {
				segmentURL := extractFirstSegment(bodyStr, entry.URL)
				if segmentURL != "" {
					segReq, segErr := http.NewRequest("GET", segmentURL, nil)
					if segErr == nil {
						segReq.Header.Set("User-Agent", userAgent)
						segResp, segErr := client.Do(segReq)
						if segErr != nil || segResp.StatusCode != http.StatusOK {
							success = false
						}
						if segResp != nil {
							segResp.Body.Close()
						}
					}
				}
			}
		} else if entry.Type == "dash" || strings.Contains(bodyStr, "<MPD") {
			entry.Type = "dash"
			success = strings.Contains(bodyStr, "<MPD")
		} else {
			success = true // fallback
		}

		if success {
			entry.Latency = ttfb
			validateLogo(client, &entry)
			results <- Result{Entry: entry, Success: true}
		} else {
			results <- Result{Entry: entry, Success: false}
		}
	}
}

// doHEAD performs a HEAD request and returns (success, ttfbMs).
// Falls back to a minimal GET if the server returns 405 Method Not Allowed.
func doHEAD(client *http.Client, url string) (bool, int64) {
	start := time.Now()

	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false, 0
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	ttfb := time.Since(start).Milliseconds()
	if err != nil {
		return false, 0
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, ttfb
	}

	// 405: server doesn't support HEAD — fall back to a GET with no body read
	if resp.StatusCode == http.StatusMethodNotAllowed {
		start = time.Now()
		getReq, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return false, 0
		}
		getReq.Header.Set("User-Agent", userAgent)
		getResp, err := client.Do(getReq)
		ttfb = time.Since(start).Milliseconds()
		if err != nil {
			return false, 0
		}
		// Drain a tiny bit so the connection can be reused, then close
		io.Copy(io.Discard, io.LimitReader(getResp.Body, 512))
		getResp.Body.Close()
		return getResp.StatusCode == http.StatusOK, ttfb
	}

	return false, 0
}

// validateLogo does a HEAD check on the entry's logo URL and clears it if broken.
// Results are cached so the same logo URL is only checked once per pipeline run.
func validateLogo(client *http.Client, entry *StreamEntry) {
	if entry.Logo == "" {
		return
	}
	if valid, ok := logoCache.Load(entry.Logo); ok {
		if !valid.(bool) {
			entry.Logo = ""
		}
		return
	}

	req, err := http.NewRequest("HEAD", entry.Logo, nil)
	valid := false
	if err == nil {
		resp, err := client.Do(req)
		if err == nil {
			valid = resp.StatusCode == http.StatusOK
			resp.Body.Close()
		}
	}
	logoCache.Store(entry.Logo, valid)
	if !valid {
		entry.Logo = ""
	}
}

// extractFirstSegment tries to find the first media segment in an M3U8 file.
func extractFirstSegment(body string, baseURL string) string {
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			if strings.HasPrefix(line, "http") {
				return line
			}
			lastSlash := strings.LastIndex(baseURL, "/")
			if lastSlash != -1 {
				return baseURL[:lastSlash+1] + line
			}
			return line
		}
	}
	return ""
}
