package logs

import (
	"sort"
	"sync"
	"time"

	"OmniProxyBackend/internal/sanitize"
)

type Level string

const (
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

type Entry struct {
	ID         int64     `json:"id"`
	Time       time.Time `json:"time"`
	Level      Level     `json:"level"`
	Method     string    `json:"method,omitempty"`
	Path       string    `json:"path,omitempty"`
	Model      string    `json:"model,omitempty"`
	ClientKey  string    `json:"clientKey,omitempty"`
	ClientName string    `json:"clientName,omitempty"`
	Status     int       `json:"status,omitempty"`
	Duration   int64     `json:"durationMs,omitempty"`
	TokenName  string    `json:"tokenName,omitempty"`
	Message    string    `json:"message"`
}

type Recorder struct {
	mu      sync.RWMutex
	max     int
	nextID  int64
	entries []Entry
}

func NewRecorder(max int) *Recorder {
	if max <= 0 {
		max = 300
	}
	return &Recorder{max: max}
}

func (r *Recorder) Add(entry Entry) Entry {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextID++
	entry.ID = r.nextID
	if entry.Time.IsZero() {
		entry.Time = time.Now()
	}
	entry.Path = sanitize.Text(entry.Path)
	entry.Message = sanitize.Text(entry.Message)
	r.entries = append(r.entries, entry)
	if len(r.entries) > r.max {
		r.entries = r.entries[len(r.entries)-r.max:]
	}
	return entry
}

func (r *Recorder) List() []Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Entry, len(r.entries))
	copy(out, r.entries)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ID > out[j].ID
	})
	return out
}
