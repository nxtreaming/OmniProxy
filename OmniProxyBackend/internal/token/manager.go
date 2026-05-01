package token

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
	name, provider, credentialType, region, value, err := normalizeRequest(req)
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

	name, provider, credentialType, region, value, err := normalizeUpdateRequest(existing, req)
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
		TokenValue:     value,
	}
	name, provider, credentialType, region, normalizedValue, err := normalizeRequest(req)
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

func (m *Manager) Acquire(provider string, excluded map[string]bool) (Token, error) {
	return m.AcquireMatching(provider, "", excluded)
}

func (m *Manager) AcquireMatching(provider string, credentialType string, excluded map[string]bool) (Token, error) {
	return m.AcquirePreferredMatching(provider, credentialType, excluded, nil)
}

func (m *Manager) AcquireBalancedMatching(provider string, credentialType string, excluded map[string]bool) (Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	provider = NormalizeProvider(provider)
	credentialType = strings.TrimSpace(strings.ToLower(credentialType))
	if credentialType != "" {
		if _, normalizedCredentialType, err := NormalizeProviderAndCredential(provider, credentialType); err == nil {
			credentialType = normalizedCredentialType
		}
	}

	if token, ok := m.bestBalancedLocked(provider, credentialType, StatusActive, excluded); ok {
		return m.reserveLocked(token), nil
	}
	if token, ok := m.bestBalancedLocked(provider, credentialType, StatusLow, excluded); ok {
		return m.reserveLocked(token), nil
	}

	return Token{}, ErrNoActiveToken
}

func (m *Manager) AcquirePreferredMatching(provider string, credentialType string, excluded map[string]bool, preferred func(Token) bool) (Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	provider = NormalizeProvider(provider)
	credentialType = strings.TrimSpace(strings.ToLower(credentialType))
	if credentialType != "" {
		if _, normalizedCredentialType, err := NormalizeProviderAndCredential(provider, credentialType); err == nil {
			credentialType = normalizedCredentialType
		}
	}

	if preferred != nil {
		if token, ok := m.firstUsablePreferredLocked(provider, credentialType, StatusActive, excluded, preferred); ok {
			return m.reserveLocked(token), nil
		}
		if token, ok := m.firstUsablePreferredLocked(provider, credentialType, StatusLow, excluded, preferred); ok {
			return m.reserveLocked(token), nil
		}
	}
	if token, ok := m.firstUsableLocked(provider, credentialType, StatusActive, excluded); ok {
		return m.reserveLocked(token), nil
	}
	if token, ok := m.firstUsableLocked(provider, credentialType, StatusLow, excluded); ok {
		return m.reserveLocked(token), nil
	}

	return Token{}, ErrNoActiveToken
}

func (m *Manager) Release(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.inFlight[id] <= 1 {
		delete(m.inFlight, id)
		return
	}
	m.inFlight[id]--
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
		m.tokens[i].CooldownUntil = nil

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

		return m.schedulePersistLocked()
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
		m.tokens[i].LastError = ""
		m.tokens[i].CooldownUntil = nil

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
		} else if usage.BalanceUnit != "" {
			remaining := balanceRemainingPercent(usage)
			m.tokens[i].Remaining = remaining
			switch {
			case usage.BalanceRemaining <= 0:
				m.tokens[i].Status = StatusExhausted
			case remaining <= m.threshold:
				m.tokens[i].Status = StatusLow
			default:
				m.tokens[i].Status = StatusActive
			}
		}

		return m.schedulePersistLocked()
	}

	return ErrTokenNotFound
}

func balanceRemainingPercent(usage UsageInfo) int {
	if usage.BalanceTotal > 0 {
		value := int(math.Round((usage.BalanceRemaining / usage.BalanceTotal) * 100))
		if value < 0 {
			return 0
		}
		if value > 100 {
			return 100
		}
		return value
	}
	if usage.BalanceRemaining <= 0 {
		return 0
	}
	return 100
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

		return m.schedulePersistLocked()
	}

	return ErrTokenNotFound
}

func (m *Manager) MarkExhausted(id string, reason string) error {
	return m.MarkExhaustedUntil(id, reason, nil)
}

func (m *Manager) MarkExhaustedUntil(id string, reason string, until *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.tokens {
		if m.tokens[i].ID != id {
			continue
		}
		m.tokens[i].Status = StatusExhausted
		m.tokens[i].LastError = reason
		m.tokens[i].CooldownUntil = until
		if until != nil {
			m.tokens[i].Health.NextCheckAt = until
		}
		m.tokens[i].UpdatedAt = time.Now()
		return m.persistLocked()
	}

	return ErrTokenNotFound
}

func (m *Manager) MarkInvalid(id string, reason string) error {
	return m.setStatus(id, StatusInvalid, reason)
}

func (m *Manager) MarkActive(id string) error {
	return m.setStatus(id, StatusActive, "")
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
		if status != StatusExhausted {
			m.tokens[i].CooldownUntil = nil
		}
		m.tokens[i].UpdatedAt = time.Now()
		return m.persistLocked()
	}

	return ErrTokenNotFound
}

func (m *Manager) RecordHealthCheck(id string, ok bool, status int, message string, nextCheckAt *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.tokens {
		if m.tokens[i].ID != id {
			continue
		}
		now := time.Now()
		m.tokens[i].Health.LastCheckedAt = &now
		m.tokens[i].Health.LastStatus = status
		m.tokens[i].Health.LastMessage = strings.TrimSpace(message)
		m.tokens[i].Health.NextCheckAt = nextCheckAt
		if ok {
			m.tokens[i].Health.ConsecutiveErrors = 0
			m.tokens[i].CooldownUntil = nil
			m.tokens[i].LastError = ""
		} else {
			m.tokens[i].Health.ConsecutiveErrors++
		}
		m.tokens[i].UpdatedAt = now
		return m.schedulePersistLocked()
	}

	return ErrTokenNotFound
}

func (m *Manager) HealthCheckCandidates(now time.Time, activeInterval time.Duration, retryInterval time.Duration) []Token {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if activeInterval <= 0 {
		activeInterval = 15 * time.Minute
	}
	if retryInterval <= 0 {
		retryInterval = time.Minute
	}

	out := []Token{}
	for _, item := range m.tokens {
		if strings.TrimSpace(item.TokenValue) == "" {
			continue
		}
		if item.Health.NextCheckAt != nil && now.Before(*item.Health.NextCheckAt) {
			continue
		}
		if item.CooldownUntil != nil && now.Before(*item.CooldownUntil) {
			continue
		}

		interval := activeInterval
		if item.Status == StatusExhausted || item.Status == StatusInvalid {
			interval = retryInterval
		}
		if item.Health.LastCheckedAt != nil && now.Sub(*item.Health.LastCheckedAt) < interval {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (m *Manager) firstUsableLocked(provider string, credentialType string, status Status, excluded map[string]bool) (Token, bool) {
	var busy Token
	hasBusy := false
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
		if m.inFlight[item.ID] == 0 {
			return item, true
		}
		if !hasBusy {
			busy = item
			hasBusy = true
		}
	}
	return busy, hasBusy
}

func (m *Manager) firstUsablePreferredLocked(provider string, credentialType string, status Status, excluded map[string]bool, preferred func(Token) bool) (Token, bool) {
	var busy Token
	hasBusy := false
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
		if m.inFlight[item.ID] == 0 {
			return item, true
		}
		if !hasBusy {
			busy = item
			hasBusy = true
		}
	}
	return busy, hasBusy
}

func (m *Manager) reserveLocked(item Token) Token {
	now := time.Now()
	m.inFlight[item.ID]++
	for i := range m.tokens {
		if m.tokens[i].ID != item.ID {
			continue
		}
		m.tokens[i].LastUsedAt = &now
		item = m.tokens[i]
		return item
	}
	item.LastUsedAt = &now
	return item
}

func (m *Manager) bestBalancedLocked(provider string, credentialType string, status Status, excluded map[string]bool) (Token, bool) {
	var selected Token
	found := false
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
		if !found || m.balancedTokenLessLocked(item, selected) {
			selected = item
			found = true
		}
	}
	return selected, found
}

func (m *Manager) balancedTokenLessLocked(left Token, right Token) bool {
	leftInFlight := m.inFlight[left.ID]
	rightInFlight := m.inFlight[right.ID]
	if leftInFlight != rightInFlight {
		return leftInFlight < rightInFlight
	}
	if left.Remaining != right.Remaining {
		return left.Remaining > right.Remaining
	}
	if left.LastUsedAt == nil && right.LastUsedAt != nil {
		return true
	}
	if left.LastUsedAt != nil && right.LastUsedAt == nil {
		return false
	}
	if left.LastUsedAt != nil && right.LastUsedAt != nil && !left.LastUsedAt.Equal(*right.LastUsedAt) {
		return left.LastUsedAt.Before(*right.LastUsedAt)
	}
	if left.Stats.RequestCount != right.Stats.RequestCount {
		return left.Stats.RequestCount < right.Stats.RequestCount
	}
	return left.CreatedAt.Before(right.CreatedAt)
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
	item.Region = normalizeStoredRegion(provider, credentialType, item.Region)
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
	if m.saveTimer != nil {
		m.saveTimer.Stop()
		m.saveTimer = nil
	}
	snapshot := make([]Token, len(m.tokens))
	copy(snapshot, m.tokens)
	err := m.store.Save(snapshot)
	m.dirty = err != nil
	return err
}

func (m *Manager) schedulePersistLocked() error {
	m.dirty = true
	if m.persistDelay <= 0 {
		return m.persistLocked()
	}
	if m.saveTimer == nil {
		m.saveTimer = time.AfterFunc(m.persistDelay, func() {
			_ = m.Flush()
		})
	}
	return nil
}

func (m *Manager) Flush() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.saveTimer != nil {
		m.saveTimer.Stop()
		m.saveTimer = nil
	}
	if !m.dirty {
		return nil
	}
	return m.persistLocked()
}

func normalizeRequest(req UpsertRequest) (string, string, string, string, string, error) {
	name := strings.TrimSpace(req.Name)
	provider := strings.TrimSpace(strings.ToLower(req.Provider))
	credentialType := strings.TrimSpace(strings.ToLower(req.CredentialType))
	value := strings.TrimSpace(req.TokenValue)

	provider, credentialType, err := NormalizeProviderAndCredential(provider, credentialType)
	if err != nil {
		return "", "", "", "", "", err
	}
	region, err := NormalizeRegion(provider, credentialType, req.Region)
	if err != nil {
		return "", "", "", "", "", err
	}

	if credentialType == CredentialTypeCodexAuthJSON {
		if !json.Valid([]byte(value)) {
			return "", "", "", "", "", errors.New("codex auth.json must be valid JSON")
		}
		email, ok := ExtractCodexEmail(value)
		if !ok {
			return "", "", "", "", "", errors.New("codex auth.json does not contain an email in tokens.id_token")
		}
		name = email
	} else if credentialType == CredentialTypeClaudeOAuth {
		if !json.Valid([]byte(value)) {
			return "", "", "", "", "", errors.New("claude OAuth JSON must be valid JSON")
		}
		fields, ok := ExtractClaudeOAuthFields(value)
		if !ok {
			return "", "", "", "", "", errors.New("claude OAuth JSON must contain access_token or refresh_token")
		}
		if fields.Email != "" {
			name = fields.Email
		}
	} else if provider == ProviderXiaomi && credentialType == CredentialTypeAPIKey && !strings.HasPrefix(value, "sk-") {
		return "", "", "", "", "", errors.New("xiaomi pay-as-you-go API key must start with sk-")
	} else if provider == ProviderXiaomi && credentialType == CredentialTypeMimoTokenPlan && !strings.HasPrefix(value, "tp-") {
		return "", "", "", "", "", errors.New("xiaomi token plan API key must start with tp-")
	} else if len(value) < 12 {
		return "", "", "", "", "", errors.New("token value is too short")
	}

	if name == "" {
		return "", "", "", "", "", errors.New("token name is required")
	}

	return name, provider, credentialType, region, value, nil
}

func normalizeUpdateRequest(existing Token, req UpsertRequest) (string, string, string, string, string, error) {
	if strings.TrimSpace(req.TokenValue) != "" {
		return normalizeRequest(req)
	}

	provider, credentialType, err := NormalizeProviderAndCredential(req.Provider, req.CredentialType)
	if err != nil {
		return "", "", "", "", "", err
	}
	if provider != NormalizeProvider(existing.Provider) || credentialType != existing.CredentialType {
		return "", "", "", "", "", errors.New("token value is required when changing provider or credential type")
	}
	region, err := NormalizeRegion(provider, credentialType, req.Region)
	if err != nil {
		return "", "", "", "", "", err
	}

	name := strings.TrimSpace(req.Name)
	if credentialType == CredentialTypeCodexAuthJSON || credentialType == CredentialTypeClaudeOAuth {
		name = existing.Name
	}
	if name == "" {
		return "", "", "", "", "", errors.New("token name is required")
	}

	value := strings.TrimSpace(existing.TokenValue)
	if value == "" {
		return "", "", "", "", "", errors.New("existing token value is empty")
	}
	return name, provider, credentialType, region, value, nil
}

func NormalizeProvider(provider string) string {
	normalized, _, err := NormalizeProviderAndCredential(provider, "")
	if err != nil {
		return ProviderOpenAI
	}
	return normalized
}

func NormalizeRegion(provider string, credentialType string, region string) (string, error) {
	provider, credentialType, err := NormalizeProviderAndCredential(provider, credentialType)
	if err != nil {
		return "", err
	}
	if provider != ProviderXiaomi || credentialType != CredentialTypeMimoTokenPlan {
		return "", nil
	}

	switch strings.ToLower(strings.TrimSpace(region)) {
	case "", MimoRegionCN, "china", "cn-mainland", "mainland":
		return MimoRegionCN, nil
	case MimoRegionSGP, "global", "overseas", "foreign", "international", "intl":
		return MimoRegionSGP, nil
	default:
		return "", errors.New("unsupported xiaomi token plan region")
	}
}

func normalizeStoredRegion(provider string, credentialType string, region string) string {
	normalized, err := NormalizeRegion(provider, credentialType, region)
	if err != nil {
		if provider == ProviderXiaomi && credentialType == CredentialTypeMimoTokenPlan {
			return MimoRegionCN
		}
		return ""
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
	case ProviderAnthropic:
		if credentialType == "" {
			credentialType = CredentialTypeAPIKey
		}
		if credentialType != CredentialTypeAPIKey && credentialType != CredentialTypeClaudeOAuth {
			return "", "", errors.New("anthropic supports API key or Claude OAuth JSON only")
		}
	case ProviderZhipu:
		if credentialType == "" {
			credentialType = CredentialTypeAPIKey
		}
		if credentialType != CredentialTypeAPIKey && credentialType != CredentialTypeCodingPlan {
			return "", "", errors.New("zhipu supports API key or Coding Plan key only")
		}
	case ProviderDeepSeek, ProviderKimi, ProviderMiniMax, ProviderGemini, ProviderCustom:
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
