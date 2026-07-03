package token

import (
	"errors"
	"strings"
	"sync"
	"time"
)

var (
	ErrDuplicateName = errors.New("token name already exists")
	ErrNoActiveToken = errors.New("no active token available")
	ErrTokenNotFound = errors.New("token not found")
)

const defaultUsagePersistDelay = 250 * time.Millisecond

type Store interface {
	Load() ([]Token, error)
	Save([]Token) error
}

type Manager struct {
	mu        sync.RWMutex
	store     Store
	tokens    []Token
	threshold int
	inFlight  map[string]int

	persistDelay time.Duration
	saveTimer    *time.Timer
	dirty        bool
}

func NewManager(store Store, threshold int) (*Manager, error) {
	if threshold <= 0 {
		threshold = 15
	}

	tokens, err := store.Load()
	if err != nil {
		return nil, err
	}
	for i := range tokens {
		tokens[i] = normalizeStoredToken(tokens[i])
	}

	return &Manager{
		store:        store,
		tokens:       tokens,
		threshold:    threshold,
		inFlight:     map[string]int{},
		persistDelay: defaultUsagePersistDelay,
	}, nil
}

func (m *Manager) SetThreshold(threshold int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if threshold <= 0 {
		threshold = 15
	}
	m.threshold = threshold
}

func (m *Manager) List() []Token {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]Token, len(m.tokens))
	copy(out, m.tokens)
	return out
}

func (m *Manager) Get(id string) (Token, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, item := range m.tokens {
		if item.ID == id {
			return item, nil
		}
	}

	return Token{}, ErrTokenNotFound
}

func (m *Manager) Add(req UpsertRequest) (Token, error) {
	name, provider, credentialType, region, baseURL, value, err := normalizeRequest(req)
	if err != nil {
		return Token{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.nameExistsLocked(name, provider, "") {
		return Token{}, ErrDuplicateName
	}

	now := time.Now()
	item := Token{
		ID:             newID(),
		Name:           name,
		Provider:       provider,
		CredentialType: credentialType,
		Region:         region,
		BaseURL:        baseURL,
		TokenValue:     value,
		Remaining:      100,
		Status:         StatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	m.tokens = append([]Token{item}, m.tokens...)
	return item, m.persistLocked()
}

func (m *Manager) Update(id string, req UpsertRequest) (Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	index := -1
	for i := range m.tokens {
		if m.tokens[i].ID == id {
			index = i
			break
		}
	}
	if index == -1 {
		return Token{}, ErrTokenNotFound
	}

	existing := m.tokens[index]
	if strings.TrimSpace(req.Provider) == "" {
		req.Provider = existing.Provider
	}
	if strings.TrimSpace(req.CredentialType) == "" {
		req.CredentialType = existing.CredentialType
	}
	if strings.TrimSpace(req.Region) == "" {
		req.Region = existing.Region
	}
	if strings.TrimSpace(req.BaseURL) == "" {
		req.BaseURL = existing.BaseURL
	}

	name, provider, credentialType, region, baseURL, value, err := normalizeUpdateRequest(existing, req)
	if err != nil {
		return Token{}, err
	}

	if m.nameExistsLocked(name, provider, id) {
		return Token{}, ErrDuplicateName
	}

	m.tokens[index].Name = name
	m.tokens[index].Provider = provider
	m.tokens[index].CredentialType = credentialType
	m.tokens[index].Region = region
	m.tokens[index].BaseURL = baseURL
	m.tokens[index].TokenValue = value
	m.tokens[index].UpdatedAt = time.Now()
	if m.tokens[index].Status == "" {
		m.tokens[index].Status = StatusActive
	}
	return m.tokens[index], m.persistLocked()
}

func (m *Manager) UpdateTokenValue(id string, value string) (Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	index := -1
	for i := range m.tokens {
		if m.tokens[i].ID == id {
			index = i
			break
		}
	}
	if index == -1 {
		return Token{}, ErrTokenNotFound
	}

	existing := m.tokens[index]
	req := UpsertRequest{
		Name:           existing.Name,
		Provider:       existing.Provider,
		CredentialType: existing.CredentialType,
		Region:         existing.Region,
		BaseURL:        existing.BaseURL,
		TokenValue:     value,
	}
	name, provider, credentialType, region, baseURL, normalizedValue, err := normalizeRequest(req)
	if err != nil {
		return Token{}, err
	}
	if m.nameExistsLocked(name, provider, id) {
		return Token{}, ErrDuplicateName
	}

	m.tokens[index].Name = name
	m.tokens[index].Provider = provider
	m.tokens[index].CredentialType = credentialType
	m.tokens[index].Region = region
	m.tokens[index].BaseURL = baseURL
	m.tokens[index].TokenValue = normalizedValue
	m.tokens[index].UpdatedAt = time.Now()
	if m.tokens[index].Status == "" || m.tokens[index].Status == StatusInvalid {
		m.tokens[index].Status = StatusActive
	}
	m.tokens[index].LastError = ""
	return m.tokens[index], m.persistLocked()
}

func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.tokens {
		if m.tokens[i].ID != id {
			continue
		}
		m.tokens = append(m.tokens[:i], m.tokens[i+1:]...)
		return m.persistLocked()
	}

	return ErrTokenNotFound
}

func (m *Manager) SetDisabled(id string, disabled bool) (Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, err := m.mutateTokenLocked(id, func(item *Token) {
		item.Disabled = disabled
		if disabled {
			item.Selected = false
		}
		item.UpdatedAt = time.Now()
	})
	if err != nil {
		return Token{}, err
	}
	return item, m.persistLocked()
}

func (m *Manager) EnableOnly(id string) ([]Token, error) {
	return m.SelectOnly(id)
}

func (m *Manager) SelectOnly(id string) ([]Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	targetIndex := -1
	for i := range m.tokens {
		if m.tokens[i].ID == id {
			targetIndex = i
			break
		}
	}
	if targetIndex == -1 {
		return nil, ErrTokenNotFound
	}

	provider := NormalizeProvider(m.tokens[targetIndex].Provider)
	now := time.Now()
	for i := range m.tokens {
		if NormalizeProvider(m.tokens[i].Provider) != provider {
			continue
		}
		nextSelected := m.tokens[i].ID == id
		changed := m.tokens[i].Selected != nextSelected
		m.tokens[i].Selected = nextSelected
		if nextSelected && m.tokens[i].Disabled {
			m.tokens[i].Disabled = false
			changed = true
		}
		if !changed {
			continue
		}
		m.tokens[i].UpdatedAt = now
	}

	out := make([]Token, len(m.tokens))
	copy(out, m.tokens)
	return out, m.persistLocked()
}

func (m *Manager) EnableProviderForToken(id string) ([]Token, error) {
	return m.ClearProviderSelectionForToken(id)
}

func (m *Manager) ClearProviderSelectionForToken(id string) ([]Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	targetIndex := -1
	for i := range m.tokens {
		if m.tokens[i].ID == id {
			targetIndex = i
			break
		}
	}
	if targetIndex == -1 {
		return nil, ErrTokenNotFound
	}

	provider := NormalizeProvider(m.tokens[targetIndex].Provider)
	now := time.Now()
	for i := range m.tokens {
		if NormalizeProvider(m.tokens[i].Provider) != provider {
			continue
		}
		if !m.tokens[i].Selected {
			continue
		}
		m.tokens[i].Selected = false
		m.tokens[i].UpdatedAt = now
	}

	out := make([]Token, len(m.tokens))
	copy(out, m.tokens)
	return out, m.persistLocked()
}

func (m *Manager) SetSelected(id string, selected bool) ([]Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	found := false
	now := time.Now()
	for i := range m.tokens {
		if m.tokens[i].ID != id {
			continue
		}
		found = true
		if m.tokens[i].Disabled && selected {
			return nil, ErrNoActiveToken
		}
		if m.tokens[i].Selected != selected {
			m.tokens[i].Selected = selected
			m.tokens[i].UpdatedAt = now
		}
		break
	}
	if !found {
		return nil, ErrTokenNotFound
	}

	out := make([]Token, len(m.tokens))
	copy(out, m.tokens)
	return out, m.persistLocked()
}
