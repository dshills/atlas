package indexer

import (
	"context"
	"os"
	"runtime"
	"sync"

	"github.com/dshills/atlas/internal/extractor"
	"github.com/dshills/atlas/internal/fswalk"
	"github.com/dshills/atlas/internal/hash"
)

// parseOutcome is what a worker produces for a single candidate. Exactly one
// of {skipped, readErr, res/extractErr} is meaningful per instance.
type parseOutcome struct {
	candidate   fswalk.FileCandidate
	contentHash string

	// skipped is true when the file's hash matches what's already in the DB —
	// the writer must not touch it.
	skipped bool
	// readErr is set if os.ReadFile failed; the writer records a diagnostic
	// and moves on without persisting.
	readErr error

	// res is the extractor output (nil if there's no extractor for this path).
	res *extractor.ExtractResult
	// hasExtractor records whether an extractor matched, distinguishing
	// "no extractor" (info diagnostic) from "extractor ran and returned nil".
	hasExtractor bool
	// extractErr is any error returned by extractor.Extract; the writer
	// promotes it to a diagnostic and marks parse_status=error.
	extractErr error
}

// workerCount returns the configured worker count, resolving 0 to NumCPU.
func (idx *Indexer) workerCount() int {
	n := idx.Config.Indexing.Workers
	if n <= 0 {
		n = runtime.NumCPU()
	}
	return n
}

// runParseWorkers spawns a worker pool that reads, hashes, and extracts each
// candidate in parallel. The results stream back on the returned channel,
// closed once all workers are done. existingHashes is read concurrently
// (writes only happen during setup, before this point).
func (idx *Indexer) runParseWorkers(ctx context.Context, candidates []fswalk.FileCandidate, existingHashes map[string]string) <-chan parseOutcome {
	jobs := make(chan fswalk.FileCandidate)
	results := make(chan parseOutcome, idx.workerCount()*2)

	go func() {
		defer close(jobs)
		for _, c := range candidates {
			select {
			case jobs <- c:
			case <-ctx.Done():
				return
			}
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < idx.workerCount(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range jobs {
				out := idx.parseOne(ctx, c, existingHashes)
				select {
				case results <- out:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

// parseOne is the pure CPU-bound half of processing: read + hash + extract.
// It touches no database state, so many of these can run in parallel.
func (idx *Indexer) parseOne(ctx context.Context, c fswalk.FileCandidate, existingHashes map[string]string) parseOutcome {
	out := parseOutcome{candidate: c}

	content, err := os.ReadFile(c.AbsPath)
	if err != nil {
		out.readErr = err
		return out
	}

	out.contentHash = hash.Hash(content)

	if existingHash, exists := existingHashes[c.Path]; exists && existingHash == out.contentHash {
		out.skipped = true
		return out
	}

	if idx.Registry == nil {
		return out
	}

	ext, err := idx.Registry.ForPath(c.Path)
	if err != nil {
		return out
	}
	out.hasExtractor = true

	req := extractor.ExtractRequest{
		FilePath:   c.Path,
		AbsPath:    c.AbsPath,
		Content:    content,
		RepoRoot:   idx.RepoRoot,
		ModulePath: idx.ModulePath,
	}

	res, err := ext.Extract(ctx, req)
	if err != nil {
		out.extractErr = err
		return out
	}
	out.res = res
	return out
}
