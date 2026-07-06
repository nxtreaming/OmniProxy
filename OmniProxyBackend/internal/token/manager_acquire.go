package token

import "strings"

func (m *Manager) Acquire(provider string, excluded map[string]bool) (Token, error) {
	return m.AcquireMatching(provider, "", excluded)
}

func (m *Manager) AcquireMatching(provider string, credentialType string, excluded map[string]bool) (Token, error) {
	return m.AcquirePreferredMatching(provider, credentialType, excluded, nil)
}

func (m *Manager) AcquireBalancedMatching(provider string, credentialType string, excluded map[string]bool) (Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.acquireBalancedMatchingLocked(provider, credentialType, excluded, nil)
}

func (m *Manager) AcquireBalancedPreferredMatching(provider string, credentialType string, excluded map[string]bool, preferred func(Token) bool) (Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.acquireBalancedMatchingLocked(provider, credentialType, excluded, preferred)
}

func (m *Manager) acquireBalancedMatchingLocked(provider string, credentialType string, excluded map[string]bool, preferred func(Token) bool) (Token, error) {
	provider = NormalizeProvider(provider)
	credentialType = strings.TrimSpace(strings.ToLower(credentialType))
	if credentialType != "" {
		if _, normalizedCredentialType, err := NormalizeProviderAndCredential(provider, credentialType); err == nil {
			credentialType = normalizedCredentialType
		}
	}

	selectedIDs, hasSelection := m.selectedProviderTokenIDsLocked(provider)
	if preferred != nil {
		if token, ok := m.reserveBalancedCandidateLocked(provider, credentialType, StatusActive, excluded, preferred, selectedIDs, hasSelection); ok {
			return token, nil
		}
	}
	if token, ok := m.reserveBalancedCandidateLocked(provider, credentialType, StatusActive, excluded, nil, selectedIDs, hasSelection); ok {
		return token, nil
	}
	if preferred != nil {
		if token, ok := m.reserveBalancedCandidateLocked(provider, credentialType, StatusLow, excluded, preferred, selectedIDs, hasSelection); ok {
			return token, nil
		}
	}
	if token, ok := m.reserveBalancedCandidateLocked(provider, credentialType, StatusLow, excluded, nil, selectedIDs, hasSelection); ok {
		return token, nil
	}

	return Token{}, ErrNoActiveToken
}

func (m *Manager) reserveBalancedCandidateLocked(provider string, credentialType string, status Status, excluded map[string]bool, preferred func(Token) bool, selectedIDs map[string]bool, hasSelection bool) (Token, bool) {
	if token, ok := m.bestBalancedCandidateLocked(provider, credentialType, status, excluded, preferred, selectedIDs, hasSelection); ok {
		return m.reserveLocked(token), true
	}
	if hasSelection {
		if token, ok := m.bestBalancedCandidateLocked(provider, credentialType, status, excluded, preferred, nil, false); ok {
			return m.reserveLocked(token), true
		}
	}
	return Token{}, false
}

func (m *Manager) bestBalancedCandidateLocked(provider string, credentialType string, status Status, excluded map[string]bool, preferred func(Token) bool, selectedIDs map[string]bool, hasSelection bool) (Token, bool) {
	if preferred != nil {
		return m.bestBalancedPreferredLocked(provider, credentialType, status, excluded, preferred, selectedIDs, hasSelection)
	}
	return m.bestBalancedLocked(provider, credentialType, status, excluded, selectedIDs, hasSelection)
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

	selectedIDs, hasSelection := m.selectedProviderTokenIDsLocked(provider)
	if preferred != nil {
		if token, ok := m.firstUsablePreferredLocked(provider, credentialType, StatusActive, excluded, preferred, selectedIDs, hasSelection); ok {
			return m.reserveLocked(token), nil
		}
		if hasSelection {
			if token, ok := m.firstUsablePreferredLocked(provider, credentialType, StatusActive, excluded, preferred, nil, false); ok {
				return m.reserveLocked(token), nil
			}
		}
	}
	if token, ok := m.firstUsableLocked(provider, credentialType, StatusActive, excluded, selectedIDs, hasSelection); ok {
		return m.reserveLocked(token), nil
	}
	if hasSelection {
		if token, ok := m.firstUsableLocked(provider, credentialType, StatusActive, excluded, nil, false); ok {
			return m.reserveLocked(token), nil
		}
	}
	if preferred != nil {
		if token, ok := m.firstUsablePreferredLocked(provider, credentialType, StatusLow, excluded, preferred, selectedIDs, hasSelection); ok {
			return m.reserveLocked(token), nil
		}
		if hasSelection {
			if token, ok := m.firstUsablePreferredLocked(provider, credentialType, StatusLow, excluded, preferred, nil, false); ok {
				return m.reserveLocked(token), nil
			}
		}
	}
	if token, ok := m.firstUsableLocked(provider, credentialType, StatusLow, excluded, selectedIDs, hasSelection); ok {
		return m.reserveLocked(token), nil
	}
	if hasSelection {
		if token, ok := m.firstUsableLocked(provider, credentialType, StatusLow, excluded, nil, false); ok {
			return m.reserveLocked(token), nil
		}
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

func (m *Manager) FindCodexAuth(provider string, fields CodexAuthFields) (Token, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	provider = NormalizeProvider(provider)
	for _, item := range m.tokens {
		if NormalizeProvider(item.Provider) != provider || item.CredentialType != CredentialTypeCodexAuthJSON {
			continue
		}
		existingFields, ok := ExtractCodexAuthFields(item.TokenValue)
		if !ok {
			continue
		}
		if codexAuthSameIdentity(existingFields, fields) {
			return item, nil
		}
	}
	return Token{}, ErrTokenNotFound
}
