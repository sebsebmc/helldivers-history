// Command buildstats is a Go port of build_stats.py. It reads the last 1440
// committed versions of 801_full_v1.json and writes aggregates.json,
// recent_attacks.json and current_status.json with the same contents the
// Python script produced.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

func main() {
	repo := flag.String("repo", ".", "path to the git repository")
	file := flag.String("file", "801_full_v1.json", "snapshot file tracked in git")
	outDir := flag.String("out", "./docs/data", "directory to write the JSON outputs to")
	limit := flag.Int("limit", 1440, "number of most recent commits to read")
	workers := flag.Int("workers", runtime.NumCPU(), "number of parallel parse workers")
	flag.Parse()
	log.SetFlags(0)

	if err := run(*repo, *file, *outDir, *limit, *workers); err != nil {
		log.Fatal(err)
	}
}

func run(repo, file, outDir string, limit, workers int) error {
	start := time.Now()
	refs, err := listCommits(repo, file, limit)
	if err != nil {
		return err
	}
	log.Printf("Reading %d commits of %s with %d workers", len(refs), file, workers)

	blobs := make(chan blob, workers)
	streamErr := make(chan error, 1)
	go func() { streamErr <- streamBlobs(repo, file, refs, blobs) }()

	var (
		mu         sync.Mutex
		snaps      []Snapshot
		latest     *CurrentStatus
		latestSnap Snapshot
		fatal      error
		wg         sync.WaitGroup
	)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for b := range blobs {
				status, err := processBlob(b)
				if status == nil && err == nil {
					continue // skipped snapshot
				}
				var snap Snapshot
				if err == nil {
					snap, err = Summarize(b.Ref.Hash, b.Ref.LogOrder, status)
				}
				mu.Lock()
				if err != nil {
					if fatal == nil {
						fatal = fmt.Errorf("commit %s: %w", b.Ref.Hash, err)
					}
				} else {
					snaps = append(snaps, snap)
					// records[-1] after the stable sort: latest time, and the
					// oldest git log entry among equal times.
					if latest == nil || snap.At > latestSnap.At ||
						(snap.At == latestSnap.At && snap.LogOrder > latestSnap.LogOrder) {
						latest, latestSnap = status, snap
					}
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if err := <-streamErr; err != nil {
		return err
	}
	if fatal != nil {
		return fatal
	}
	log.Printf("Parsed %d valid snapshots in %s", len(snaps), time.Since(start).Round(time.Millisecond))

	SortSnapshots(snaps)
	steps, ranked, err := Aggregate(snaps)
	if err != nil {
		return err
	}

	if err := writeFile(filepath.Join(outDir, "aggregates.json"), func(w *bufio.Writer) error {
		WriteAggregates(w, steps)
		return nil
	}); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(outDir, "recent_attacks.json"), func(w *bufio.Writer) error {
		WriteRecentAttacks(w, ranked)
		return nil
	}); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(outDir, "current_status.json"), func(w *bufio.Writer) error {
		return WriteCurrentStatus(w, latest)
	}); err != nil {
		return err
	}
	log.Printf("Done in %s", time.Since(start).Round(time.Millisecond))
	return nil
}

// processBlob parses and converts one snapshot. It returns (nil, nil) for
// snapshots that build_stats.py skips: failed API fetches and anything that
// fails validation.
func processBlob(b blob) (*CurrentStatus, error) {
	if b.Missing {
		log.Printf("Commit %s has no %s, skipping", b.Ref.Hash, "snapshot file")
		return nil, nil
	}
	if strings.HasPrefix(b.Ref.Hash, "a514ea") {
		return nil, nil // known bad commit, skipped by build_stats.py
	}
	rec, err := ParseFullStatus(b.Data)
	if err != nil {
		if !isErrorResponse(b.Data) {
			log.Printf("Bad committed v1 data for commit %s: %s", b.Ref.Hash, oneLine(err))
		}
		return nil, nil
	}
	return Convert(rec, b.Ref.CommitTime*1000)
}

// isErrorResponse reports whether the document is an API error payload.
func isErrorResponse(data []byte) bool {
	var top map[string]json.RawMessage
	if json.Unmarshal(data, &top) != nil {
		return false
	}
	_, e1 := top["error"]
	_, e2 := top["errors"]
	return e1 || e2
}

func oneLine(err error) string {
	s := strings.ReplaceAll(err.Error(), "\n", "; ")
	if len(s) > 300 {
		s = s[:300] + "..."
	}
	return s
}

func writeFile(path string, write func(*bufio.Writer) error) error {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	if err := write(w); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
