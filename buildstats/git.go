package main

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

type commitRef struct {
	Hash       string
	CommitTime int64 // committer time, unix seconds
	LogOrder   int
}

// listCommits returns the newest `limit` commits that touched path, newest
// first, like `git log --format=%H <path>` in build_stats.py.
func listCommits(repo, path string, limit int) ([]commitRef, error) {
	cmd := exec.Command("git", "-C", repo, "log", "--format=%H %ct", "-n", strconv.Itoa(limit), "--", path)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	var refs []commitRef
	for i, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		hash, ts, ok := strings.Cut(line, " ")
		if !ok {
			return nil, fmt.Errorf("git log: unexpected line %q", line)
		}
		t, err := strconv.ParseInt(ts, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("git log: bad timestamp in %q", line)
		}
		refs = append(refs, commitRef{Hash: hash, CommitTime: t, LogOrder: i})
	}
	return refs, nil
}

// blob is the content of path at one commit. Missing is set when the path
// doesn't exist in that commit.
type blob struct {
	Ref     commitRef
	Data    []byte
	Missing bool
}

// streamBlobs reads path at every commit through a single
// `git cat-file --batch` process and sends the results in order.
func streamBlobs(repo, path string, refs []commitRef, out chan<- blob) error {
	defer close(out)
	cmd := exec.Command("git", "-C", repo, "cat-file", "--batch")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("git cat-file: %w", err)
	}

	go func() {
		w := bufio.NewWriter(stdin)
		for _, r := range refs {
			fmt.Fprintf(w, "%s:%s\n", r.Hash, path)
		}
		w.Flush()
		stdin.Close()
	}()

	r := bufio.NewReaderSize(stdout, 1<<20)
	for _, ref := range refs {
		header, err := r.ReadString('\n')
		if err != nil {
			return fmt.Errorf("git cat-file: reading header for %s: %w", ref.Hash, err)
		}
		fields := strings.Fields(header)
		if len(fields) == 2 && fields[1] == "missing" {
			out <- blob{Ref: ref, Missing: true}
			continue
		}
		if len(fields) != 3 {
			return fmt.Errorf("git cat-file: unexpected header %q", header)
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil {
			return fmt.Errorf("git cat-file: bad size in %q", header)
		}
		data := make([]byte, size+1) // content plus trailing newline
		if _, err := io.ReadFull(r, data); err != nil {
			return fmt.Errorf("git cat-file: reading %s: %w", ref.Hash, err)
		}
		out <- blob{Ref: ref, Data: data[:size]}
	}
	return cmd.Wait()
}
