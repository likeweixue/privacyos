package core

import (
	"context"
	"sort"
	"sync"
	"time"
)

// Job is a background task with progress reporting.
type Job struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"` // running | done | failed | canceled
	Progress   int    `json:"progress"`
	Message    string `json:"message"`
	Error      string `json:"error,omitempty"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at,omitempty"`
}

// Jobs is an in-memory job registry. It is intentionally ephemeral: long-lived
// results live in the database, jobs only report progress to the UI.
type Jobs struct {
	mu      sync.RWMutex
	items   map[string]*Job
	cancels map[string]context.CancelFunc
}

// NewJobs creates an empty registry.
func NewJobs() *Jobs {
	return &Jobs{items: map[string]*Job{}, cancels: map[string]context.CancelFunc{}}
}

// Start runs fn in the background and returns the job handle.
func (j *Jobs) Start(name string, fn func(ctx context.Context, report func(int, string)) error) *Job {
	job := &Job{
		ID:        newJobID(),
		Name:      name,
		Status:    "running",
		StartedAt: time.Now().UTC().Format(time.RFC3339),
	}
	ctx, cancel := context.WithCancel(context.Background())

	j.mu.Lock()
	j.items[job.ID] = job
	j.cancels[job.ID] = cancel
	j.pruneLocked()
	j.mu.Unlock()

	report := func(pct int, msg string) {
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		j.mu.Lock()
		job.Progress = pct
		if msg != "" {
			job.Message = msg
		}
		j.mu.Unlock()
	}

	go func() {
		defer cancel()
		err := fn(ctx, report)
		j.mu.Lock()
		job.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		switch {
		case err == context.Canceled:
			job.Status = "canceled"
			job.Message = "已取消"
		case err != nil:
			job.Status = "failed"
			job.Error = err.Error()
		default:
			job.Status = "done"
			job.Progress = 100
		}
		delete(j.cancels, job.ID)
		j.mu.Unlock()
	}()
	return job
}

// Get returns a job by ID.
func (j *Jobs) Get(id string) (*Job, bool) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	it, ok := j.items[id]
	if !ok {
		return nil, false
	}
	cp := *it
	return &cp, true
}

// List returns jobs, newest first.
func (j *Jobs) List(limit int) []Job {
	j.mu.RLock()
	defer j.mu.RUnlock()
	out := make([]Job, 0, len(j.items))
	for _, it := range j.items {
		out = append(out, *it)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].StartedAt > out[b].StartedAt })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// Cancel aborts a running job.
func (j *Jobs) Cancel(id string) bool {
	j.mu.RLock()
	cancel, ok := j.cancels[id]
	j.mu.RUnlock()
	if ok {
		cancel()
	}
	return ok
}

// pruneLocked drops finished jobs beyond the most recent 50.
func (j *Jobs) pruneLocked() {
	if len(j.items) <= 50 {
		return
	}
	type pair struct {
		id string
		at string
	}
	var finished []pair
	for id, it := range j.items {
		if it.Status != "running" {
			finished = append(finished, pair{id, it.StartedAt})
		}
	}
	sort.Slice(finished, func(a, b int) bool { return finished[a].at < finished[b].at })
	for i := 0; i < len(finished)-40; i++ {
		delete(j.items, finished[i].id)
	}
}

func newJobID() string {
	return "job-" + time.Now().UTC().Format("20060102150405") + "-" + randomSuffix(4)
}
