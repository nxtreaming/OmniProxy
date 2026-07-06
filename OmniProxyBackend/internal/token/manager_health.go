package token

import (
	"strings"
	"time"
)

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
		if item.Disabled {
			continue
		}
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

func (m *Manager) mutateTokenLocked(id string, mutate func(*Token)) (Token, error) {
	for i := range m.tokens {
		if m.tokens[i].ID != id {
			continue
		}
		mutate(&m.tokens[i])
		return m.tokens[i], nil
	}
	return Token{}, ErrTokenNotFound
}

func (m *Manager) selectedProviderTokenIDsLocked(provider string) (map[string]bool, bool) {
	ids := map[string]bool{}
	for _, item := range m.tokens {
		if item.Selected && NormalizeProvider(item.Provider) == provider {
			ids[item.ID] = true
		}
	}
	if len(ids) == 0 {
		return nil, false
	}
	return ids, true
}

func (m *Manager) firstUsableLocked(provider string, credentialType string, status Status, excluded map[string]bool, selectedIDs map[string]bool, hasSelection bool) (Token, bool) {
	var busy Token
	hasBusy := false
	for _, item := range m.tokens {
		if !usableTokenMatches(item, provider, credentialType, status, excluded, selectedIDs, hasSelection) {
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

func (m *Manager) firstUsablePreferredLocked(provider string, credentialType string, status Status, excluded map[string]bool, preferred func(Token) bool, selectedIDs map[string]bool, hasSelection bool) (Token, bool) {
	var busy Token
	hasBusy := false
	for _, item := range m.tokens {
		if !usableTokenMatches(item, provider, credentialType, status, excluded, selectedIDs, hasSelection) {
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

func usableTokenMatches(item Token, provider string, credentialType string, status Status, excluded map[string]bool, selectedIDs map[string]bool, hasSelection bool) bool {
	if hasSelection && !selectedIDs[item.ID] {
		return false
	}
	if item.Disabled {
		return false
	}
	if NormalizeProvider(item.Provider) != provider {
		return false
	}
	if credentialType != "" && item.CredentialType != credentialType {
		return false
	}
	if item.Status != status {
		return false
	}
	if excluded != nil && excluded[item.ID] {
		return false
	}
	return strings.TrimSpace(item.TokenValue) != ""
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

func (m *Manager) bestBalancedLocked(provider string, credentialType string, status Status, excluded map[string]bool, selectedIDs map[string]bool, hasSelection bool) (Token, bool) {
	var selected Token
	selectedIndex := -1
	found := false
	for index, item := range m.tokens {
		if !usableTokenMatches(item, provider, credentialType, status, excluded, selectedIDs, hasSelection) {
			continue
		}
		if !found || m.balancedTokenLessLocked(item, selected, index, selectedIndex) {
			selected = item
			selectedIndex = index
			found = true
		}
	}
	return selected, found
}

func (m *Manager) bestBalancedPreferredLocked(provider string, credentialType string, status Status, excluded map[string]bool, preferred func(Token) bool, selectedIDs map[string]bool, hasSelection bool) (Token, bool) {
	var selected Token
	selectedIndex := -1
	found := false
	for index, item := range m.tokens {
		if !usableTokenMatches(item, provider, credentialType, status, excluded, selectedIDs, hasSelection) {
			continue
		}
		if preferred != nil && !preferred(item) {
			continue
		}
		if !found || m.balancedTokenLessLocked(item, selected, index, selectedIndex) {
			selected = item
			selectedIndex = index
			found = true
		}
	}
	return selected, found
}

func (m *Manager) balancedTokenLessLocked(left Token, right Token, leftIndex int, rightIndex int) bool {
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
	if !left.CreatedAt.Equal(right.CreatedAt) {
		return left.CreatedAt.Before(right.CreatedAt)
	}
	return leftIndex > rightIndex
}

func (m *Manager) tokenIdentityExistsLocked(name string, provider string, credentialType string, value string, exceptID string) bool {
	provider = NormalizeProvider(provider)
	var incomingCodexFields CodexAuthFields
	incomingCodexOK := false
	if credentialType == CredentialTypeCodexAuthJSON {
		incomingCodexFields, incomingCodexOK = ExtractCodexAuthFields(value)
	}

	for _, item := range m.tokens {
		if item.ID == exceptID {
			continue
		}
		if NormalizeProvider(item.Provider) != provider {
			continue
		}
		if credentialType == CredentialTypeCodexAuthJSON && item.CredentialType == CredentialTypeCodexAuthJSON && incomingCodexOK {
			existingCodexFields, ok := ExtractCodexAuthFields(item.TokenValue)
			if ok && codexAuthSameIdentity(existingCodexFields, incomingCodexFields) {
				return true
			}
			if ok {
				continue
			}
		}
		if strings.EqualFold(strings.TrimSpace(item.Name), name) {
			return true
		}
	}
	return false
}
