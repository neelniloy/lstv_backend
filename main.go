package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"time"
)

func runPipeline() error {
	fmt.Printf("\n--- Starting Pipeline: %s ---\n", time.Now().Format(time.RFC3339))

	// 0. Load Stability Data
	if err := LoadStability(); err != nil {
		fmt.Printf("Warning: failed to load stability data: %v\n", err)
	}

	// 1. Fetch Playlists
	playlists, err := FetchPlaylists("sources.txt")
	if err != nil {
		return fmt.Errorf("error fetching playlists: %w", err)
	}

	// 2. Parse and Normalize
	var allEntries []StreamEntry
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, content := range playlists {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			entries := ParseM3U(c)
			for i := range entries {
				entries[i].Quality = DetectQuality(entries[i].Name, entries[i].URL)
				entries[i].Name = NormalizeName(entries[i].Name)
				entries[i].Category = NormalizeCategory(entries[i].Category, entries[i].Name)
			}
			mu.Lock()
			allEntries = append(allEntries, entries...)
			mu.Unlock()
		}(content)
	}
	wg.Wait()
	fmt.Printf("Total entries parsed: %d\n", len(allEntries))

	// 2.5 Deduplicate by URL before Health Check to save time/bandwidth
	fmt.Println("Deduplicating entries before health check...")
	uniqueEntries := make([]StreamEntry, 0, len(allEntries))
	urlSeen := make(map[string]bool)
	for _, entry := range allEntries {
		if !urlSeen[entry.URL] {
			urlSeen[entry.URL] = true
			uniqueEntries = append(uniqueEntries, entry)
		}
	}
	fmt.Printf("Unique entries to check: %d\n", len(uniqueEntries))

	// 3. Health Check
	fmt.Println("Running health checks on streams...")
	healthyEntries := RunHealthChecks(uniqueEntries, 100)

	// 4. Save Stability Data
	if err := SaveStability(); err != nil {
		fmt.Printf("Warning: failed to save stability data: %v\n", err)
	}

	// 5. Generate JSON
	if err := GenerateJSON(healthyEntries, "channels.json"); err != nil {
		return fmt.Errorf("error generating output: %w", err)
	}

	fmt.Printf("--- Pipeline Completed: %v ---\n", time.Now().Format(time.RFC3339))
	return nil
}

func main() {
	cronMode := flag.Bool("cron", false, "Run pipeline once and exit (for GitHub Actions)")
	flag.Parse()

	if *cronMode {
		fmt.Println("Running in CRON mode (one-shot)")
		if err := runPipeline(); err != nil {
			fmt.Fprintf(os.Stderr, "Pipeline failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	fmt.Println("IPTV Backend Processor Started")

	// Run initially
	runPipeline()

	// Schedule every 30 minutes
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	fmt.Println("Scheduler started: Pipeline will run every 30 minutes.")

	for t := range ticker.C {
		fmt.Printf("Scheduled run at %v\n", t)
		runPipeline()
	}
}
