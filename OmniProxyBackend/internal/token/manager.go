package token

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrDuplicateName = errors.New("token name already exists")
	ErrNoActiveToken = errors.New("no active token available")
	ErrTokenNotFound = errors.New("token not found")
)

type Store interface {
	Load() ([]Token, error)
	Save([]Token) error
}

type Manager struct {
	mu        sync.RWMutex
	store     Store
	tokens    []Token
	threshold int
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
		store:     store,
		tokens:    tokens,
		threshold: threshold,
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
	name, provider, credentialType, value, err := normalizeRequest(req)
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
	name, provider, credentialType, value, err := normalizeRequest(req)
	if err != nil {
		return Token{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.nameExistsLocked(name, provider, id) {
		return Token{}, ErrDuplicateName
	}

	for i := range m.tokens {
		if m.tokens[i].ID != id {
			continue
		}
		m.tokens[i].Name = name
		m.tokens[i].Provider = provider
		m.tokens[i].CredentialType = credentialType
		m.tokens[i].TokenValue = value
		m.tokens[i].UpdatedAt = time.Now()
		if m.tokens[i].Status == "" {
			m.tokens[i].Status = StatusActive
		}
		return m.tokens[i], m.persistLocked()
	}

	return Token{}, ErrTokenNotFound
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

func (m *Manager) Acquire(provider string, excluded map[string]bool) (Token, error) {
	return m.AcquireMatching(provider, "", excluded)
}

func (m *Manager) AcquireMatching(provider string, credentialType string, excluded map[string]bool) (Token, error) {
	return m.AcquirePreferredMatching(provider, credentialType, excluded, nil)
}

func (m *Manager) AcquirePreferredMatching(provider string, credentialType string, excluded map[string]bool, preferred func(Token) bool) (Token, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	provider = NormalizeProvider(provider)
	credentialType = strings.TrimSpace(strings.ToLower(credentialType))
	if credentialType != "" {
		if _, normalizedCredentialType, err := NormalizeProviderAndCredential(provider, credentialType); err == nil {
			credentialType = normalizedCredentialType
		}
	}

	if preferred != nil {
		if token, ok := m.firstUsablePreferredLocked(provider, credentialType, StatusActive, excluded, preferred); ok {
			return token, nil
		}
		if token, ok := m.firstUsablePreferredLocked(provider, credentialType, StatusLow, excluded, preferred); ok {
			return token, nil
		}
	}
	if token, ok := m.firstUsableLocked(provider, credentialType, StatusActive, excluded); ok {
		return token, nil
	}
	if token, ok := m.firstUsableLocked(provider, credentialType, StatusLow, excluded); ok {
		return token, nil
	}

	return Token{}, ErrNoActiveToken
}

func (m *Manager) FindByName(provider string, name string) (Token, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	provider = NormalizeProvider(provider)
	name = strings.TrimSpace(name)
	for _, item := range m.tokens {
		if NormalizeProvider(item.Provider) != provider {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.Name), name) {
			return item, nil
		}
	}
	return Token{}, ErrTokenNotFound
}

func (m *Manager) RecordUsage(id string, remaining int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.tokens {
		if m.tokens[i].ID != id {
			continue
		}

		now := time.Now()
		m.tokens[i].LastUsedAt = &now
		m.tokens[i].UpdatedAt = now
		m.tokens[i].LastError = ""

		if remaining >= 0 {
			m.tokens[i].Remaining = remaining
			m.tokens[i].Usage.APIRemaining = remaining
			m.tokens[i].Usage.UpdatedAt = &now
			switch {
			case remaining <= 0:
				m.tokens[i].Status = StatusExhausted
			case remaining <= m.threshold:
				m.tokens[i].Status = StatusLow
			default:
				m.tokens[i].Status = StatusActive
			}
		}

		return m.persistLocked()
	}

	return ErrTokenNotFound
}

func (m *Manager) RecordUsageInfo(id string, usage UsageInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.tokens {
		if m.tokens[i].ID != id {
			continue
		}

		now := time.Now()
		usage.UpdatedAt = &now
		m.tokens[i].Usage = usage
		m.tokens[i].UpdatedAt = now

		if usage.SubscriptionQuotaAvailable {
			m.tokens[i].Remaining = usage.PrimaryRemainingPercent
			switch {
			case usage.LimitReached || usage.PrimaryRemainingPercent <= 0:
				m.tokens[i].Status = StatusExhausted
			case usage.PrimaryRemainingPercent <= m.threshold:
				m.tokens[i].Status = StatusLow
			default:
				m.tokens[i].Status = StatusActive
			}
		}

		return m.persistLocked()
	}

	return ErrTokenNotFound
}

func (m *Manager) RecordProxyUsage(id string, consumption TokenConsumption) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.tokens {
		if m.tokens[i].ID != id {
			continue
		}

		now := time.Now()
		consumption = normalizeConsumption(consumption)
		m.tokens[i].LastUsedAt = &now
		m.tokens[i].UpdatedAt = now
		m.tokens[i].LastError = ""
		m.tokens[i].Stats.RequestCount++
		m.tokens[i].Stats.InputTokens += int64(consumption.InputTokens)
		m.tokens[i].Stats.OutputTokens += int64(consumption.OutputTokens)
		m.tokens[i].Stats.TotalTokens += int64(consumption.TotalTokens)
		m.tokens[i].Stats.LastInputTokens = consumption.InputTokens
		m.tokens[i].Stats.LastOutputTokens = consumption.OutputTokens
		m.tokens[i].Stats.LastTotalTokens = consumption.TotalTokens
		m.tokens[i].Stats.UpdatedAt = &now
		m.tokens[i].Stats.Daily = recordDailyUsage(m.tokens[i].Stats.Daily, now, consumption)

		return m.persistLocked()
	}

	return ErrTokenNotFound
}

func (m *Manager) MarkExhausted(id string, reason string) error {
	return m.setStatus(id, StatusExhausted, reason)
}

func (m *Manager) MarkInvalid(id string, reason string) error {
	return m.setStatus(id, StatusInvalid, reason)
}

func (m *Manager) setStatus(id string, status Status, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.tokens {
		if m.tokens[i].ID != id {
			continue
		}
		m.tokens[i].Status = status
		m.tokens[i].LastError = reason
		m.tokens[i].UpdatedAt = time.Now()
		return m.persistLocked()
	}

	return ErrTokenNotFound
}

func (m *Manager) firstUsableLocked(provider string, credentialType string, status Status, excluded map[string]bool) (Token, bool) {
	for _, item := range m.tokens {
		if NormalizeProvider(item.Provider) != provider {
			continue
		}
		if credentialType != "" && item.CredentialType != credentialType {
			continue
		}
		if item.Status != status {
			continue
		}
		if excluded != nil && excluded[item.ID] {
			continue
		}
		if strings.TrimSpace(item.TokenValue) == "" {
			continue
		}
		return item, true
	}
	return Token{}, false
}

func (m *Manager) firstUsablePreferredLocked(provider string, credentialType string, status Status, excluded map[string]bool, preferred func(Token) bool) (Token, bool) {
	for _, item := range m.tokens {
		if NormalizeProvider(item.Provider) != provider {
			continue
		}
		if credentialType != "" && item.CredentialType != credentialType {
			continue
		}
		if item.Status != status {
			continue
		}
		if excluded != nil && excluded[item.ID] {
			continue
		}
		if strings.TrimSpace(item.TokenValue) == "" {
			continue
		}
		if !preferred(item) {
			continue
		}
		return item, true
	}
	return Token{}, false
}

func (m *Manager) nameExistsLocked(name string, provider string, exceptID string) bool {
	provider = NormalizeProvider(provider)
	for _, item := range m.tokens {
		if item.ID == exceptID {
			continue
		}
		if NormalizeProvider(item.Provider) != provider {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.Name), name) {
			return true
		}
	}
	return false
}

func normalizeStoredToken(item Token) Token {
	provider, credentialType, err := NormalizeProviderAndCredential(item.Provider, item.CredentialType)
	if err != nil {
		provider = ProviderOpenAI
		credentialType = CredentialTypeAPIKey
	}
	item.Provider = provider
	item.CredentialType = credentialType
	if item.Status == "" {
		item.Status = StatusActive
	}
	return item
}

func normalizeConsumption(consumption TokenConsumption) TokenConsumption {
	if consumption.InputTokens < 0 {
		consumption.InputTokens = 0
	}
	if consumption.OutputTokens < 0 {
		consumption.OutputTokens = 0
	}
	if consumption.TotalTokens < 0 {
		consumption.TotalTokens = 0
	}
	if consumption.TotalTokens == 0 && (consumption.InputTokens > 0 || consumption.OutputTokens > 0) {
		consumption.TotalTokens = consumption.InputTokens + consumption.OutputTokens
	}
	return consumption
}

func recordDailyUsage(existing []DailyTokenUsage, now time.Time, consumption TokenConsumption) []DailyTokenUsage {
	day := now.Format("2006-01-02")
	for i := range existing {
		if existing[i].Date != day {
			continue
		}
		existing[i].RequestCount++
		existing[i].InputTokens += int64(consumption.InputTokens)
		existing[i].OutputTokens += int64(consumption.OutputTokens)
		existing[i].TotalTokens += int64(consumption.TotalTokens)
		return trimDailyUsage(existing)
	}

	next := append(existing, DailyTokenUsage{
		Date:         day,
		RequestCount: 1,
		InputTokens:  int64(consumption.InputTokens),
		OutputTokens: int64(consumption.OutputTokens),
		TotalTokens:  int64(consumption.TotalTokens),
	})
	return trimDailyUsage(next)
}

func trimDailyUsage(existing []DailyTokenUsage) []DailyTokenUsage {
	const maxDays = 90
	if len(existing) <= maxDays {
		return existing
	}
	return existing[len(existing)-maxDays:]
}

func (m *Manager) persistLocked() error {
	snapshot := make([]Token, len(m.tokens))
	copy(snapshot, m.tokens)
	return m.store.Save(snapshot)
}

func normalizeRequest(req UpsertRequest) (string, string, string, string, error) {
	name := strings.TrimSpace(req.Name)
	provider := strings.TrimSpace(strings.ToLower(req.Provider))
	credentialType := strings.TrimSpace(strings.ToLower(req.CredentialType))
	value := strings.TrimSpace(req.TokenValue)

	provider, credentialType, err := NormalizeProviderAndCredential(provider, credentialType)
	if err != nil {
		return "", "", "", "", err
	}

	if credentialType == CredentialTypeCodexAuthJSON {
		if !json.Valid([]byte(value)) {
			return "", "", "", "", errors.New("codex auth.json must be valid JSON")
		}
		email, ok := ExtractCodexEmail(value)
		if !ok {
			return "", "", "", "", errors.New("codex auth.json does not contain an email in tokens.id_token")
		}
		name = email
	} else if provider == ProviderXiaomi && credentialType == CredentialTypeAPIKey && !strings.HasPrefix(value, "sk-") {
		return "", "", "", "", errors.New("xiaomi pay-as-you-go API key must start with sk-")
	} else if provider == ProviderXiaomi && credentialType == CredentialTypeMimoTokenPlan && !strings.HasPrefix(value, "tp-") {
		return "", "", "", "", errors.New("xiaomi token plan API key must start with tp-")
	} else if len(value) < 12 {
		return "", "", "", "", errors.New("token value is too short")
	}

	if name == "" {
		return "", "", "", "", errors.New("token name is required")
	}

	return name, provider, credentialType, value, nil
}

func NormalizeProvider(provider string) string {
	normalized, _, err := NormalizeProviderAndCredential(provider, "")
	if err != nil {
		return ProviderOpenAI
	}
	return normalized
}

func NormalizeProviderAndCredential(provider string, credentialType string) (string, string, error) {
	provider = strings.TrimSpace(strings.ToLower(provider))
	credentialType = strings.TrimSpace(strings.ToLower(credentialType))

	if provider == "" {
		provider = ProviderOpenAI
	}
	if provider == "codex" {
		provider = ProviderOpenAI
		if credentialType == "" {
			credentialType = CredentialTypeCodexAuthJSON
		}
	}

	switch provider {
	case ProviderOpenAI:
		if credentialType == "" {
			credentialType = CredentialTypeAPIKey
		}
		if credentialType != CredentialTypeAPIKey && credentialType != CredentialTypeCodexAuthJSON {
			return "", "", errors.New("unsupported OpenAI credential type")
		}
	case ProviderAnthropic, ProviderDeepSeek, ProviderKimi:
		if credentialType == "" {
			credentialType = CredentialTypeAPIKey
		}
		if credentialType != CredentialTypeAPIKey {
			return "", "", errors.New("this provider currently supports API key only")
		}
	case ProviderXiaomi:
		if credentialType == "" {
			credentialType = CredentialTypeAPIKey
		}
		if credentialType != CredentialTypeAPIKey && credentialType != CredentialTypeMimoTokenPlan {
			return "", "", errors.New("xiaomi supports API key or Token Plan key only")
		}
	default:
		return "", "", errors.New("unsupported provider")
	}

	return provider, credentialType, nil
}

func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
