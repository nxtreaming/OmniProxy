package history

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultPersistDelay = 500 * time.Millisecond

type Store interface {
	Load() ([]Entry, error)
	Save([]Entry) error
}

type Entry struct {
	ID                int64          `json:"id"`
	Time              time.Time      `json:"time"`
	Level             string         `json:"level"`
	Method            string         `json:"method,omitempty"`
	Path              string         `json:"path,omitempty"`
	Provider          string         `json:"provider,omitempty"`
	Protocol          string         `json:"protocol,omitempty"`
	Model             string         `json:"model,omitempty"`
	Status            int            `json:"status,omitempty"`
	Duration          int64          `json:"durationMs,omitempty"`
	TokenID           string         `json:"tokenId,omitempty"`
	TokenName         string         `json:"tokenName,omitempty"`
	InputTokens       int            `json:"inputTokens,omitempty"`
	OutputTokens      int            `json:"outputTokens,omitempty"`
	TotalTokens       int            `json:"totalTokens,omitempty"`
	CooldownTriggered bool           `json:"cooldownTriggered,omitempty"`
	RetryChain        []RetryAttempt `json:"retryChain,omitempty"`
	Message           string         `json:"message"`
}

type RetryAttempt struct {
	Attempt           int    `json:"attempt"`
	Provider          string `json:"provider,omitempty"`
	Protocol          string `json:"protocol,omitempty"`
	Model             string `json:"model,omitempty"`
	Status            int    `json:"status,omitempty"`
	Duration          int64  `json:"durationMs,omitempty"`
	TokenID           string `json:"tokenId,omitempty"`
	TokenName         string `json:"tokenName,omitempty"`
	CooldownTriggered bool   `json:"cooldownTriggered,omitempty"`
	Message           string `json:"message,omitempty"`
}

type Filter struct {
	Provider string `json:"provider,omitempty"`
	Level    string `json:"level,omitempty"`
	Status   string `json:"status,omitempty"`
	Model    string `json:"model,omitempty"`
	Token    string `json:"token,omitempty"`
	Search   string `json:"search,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

type Recorder struct {
	mu           sync.RWMutex
	store        Store
	max          int
	nextID       int64
	entries      []Entry
	persistDelay time.Duration
	saveTimer    *time.Timer
	dirty        bool
}

func NewRecorder(store Store, max int) (*Recorder, error) {
	if max <= 0 {
		max = 5000
	}

	entries, err := store.Load()
	if err != nil {
		return nil, err
	}
	if len(entries) > max {
		entries = entries[len(entries)-max:]
	}

	var nextID int64
	for _, entry := range entries {
		if entry.ID > nextID {
			nextID = entry.ID
		}
	}

	return &Recorder{
		store:        store,
		max:          max,
		nextID:       nextID,
		entries:      entries,
		persistDelay: defaultPersistDelay,
	}, nil
}

func (r *Recorder) Add(entry Entry) Entry {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextID++
	entry.ID = r.nextID
	if entry.Time.IsZero() {
		entry.Time = time.Now()
	}
	if entry.Level == "" {
		entry.Level = "info"
	}
	r.entries = append(r.entries, entry)
	if len(r.entries) > r.max {
		r.entries = r.entries[len(r.entries)-r.max:]
	}
	_ = r.schedulePersistLocked()
	return entry
}

func (r *Recorder) List(filter Filter) []Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 || limit > r.max {
		limit = r.max
	}

	out := make([]Entry, 0, len(r.entries))
	for _, entry := range r.entries {
		if !matches(entry, filter) {
			continue
		}
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ID > out[j].ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (r *Recorder) Flush() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.saveTimer != nil {
		r.saveTimer.Stop()
		r.saveTimer = nil
	}
	if !r.dirty {
		return nil
	}
	return r.persistLocked()
}

func (r *Recorder) schedulePersistLocked() error {
	r.dirty = true
	if r.persistDelay <= 0 {
		return r.persistLocked()
	}
	if r.saveTimer == nil {
		r.saveTimer = time.AfterFunc(r.persistDelay, func() {
			_ = r.Flush()
		})
	}
	return nil
}

func (r *Recorder) persistLocked() error {
	snapshot := make([]Entry, len(r.entries))
	copy(snapshot, r.entries)
	err := r.store.Save(snapshot)
	r.dirty = err != nil
	return err
}

func matches(entry Entry, filter Filter) bool {
	if filter.Provider != "" && filter.Provider != "all" && !strings.EqualFold(entry.Provider, filter.Provider) {
		return false
	}
	if filter.Level != "" && filter.Level != "all" && !strings.EqualFold(entry.Level, filter.Level) {
		return false
	}
	if filter.Status != "" && filter.Status != "all" && !matchesStatus(entry.Status, filter.Status) {
		return false
	}
	if filter.Model != "" && !strings.Contains(strings.ToLower(entry.Model), strings.ToLower(filter.Model)) {
		return false
	}
	if filter.Token != "" && !strings.Contains(strings.ToLower(entry.TokenName), strings.ToLower(filter.Token)) {
		return false
	}
	if filter.Search != "" {
		needle := strings.ToLower(filter.Search)
		haystack := strings.ToLower(strings.Join([]string{
			entry.Method,
			entry.Path,
			entry.Provider,
			entry.Protocol,
			entry.Model,
			entry.TokenName,
			entry.Message,
			strconv.Itoa(entry.Status),
		}, " "))
		if !strings.Contains(haystack, needle) {
			return false
		}
	}
	return true
}

func matchesStatus(status int, filter string) bool {
	switch strings.ToLower(strings.TrimSpace(filter)) {
	case "success":
		return status >= 200 && status < 400
	case "error":
		return status == 0 || status >= 400
	default:
		parsed, err := strconv.Atoi(filter)
		return err == nil && status == parsed
	}
}
