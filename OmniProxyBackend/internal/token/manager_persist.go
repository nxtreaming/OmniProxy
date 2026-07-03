package token

import "time"

func normalizeStoredToken(item Token) Token {
	provider, credentialType, err := NormalizeProviderAndCredential(item.Provider, item.CredentialType)
	if err != nil {
		provider = ProviderOpenAI
		credentialType = CredentialTypeAPIKey
	}
	item.Provider = provider
	item.CredentialType = credentialType
	item.Region = normalizeStoredRegion(provider, credentialType, item.Region)
	item.BaseURL = normalizeStoredBaseURL(provider, item.BaseURL)
	if item.Status == "" {
		item.Status = StatusActive
	}
	if item.Pinned {
		item.Selected = true
		item.Pinned = false
	}
	if item.Disabled {
		item.Selected = false
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
	if consumption.CacheCreationTokens < 0 {
		consumption.CacheCreationTokens = 0
	}
	if consumption.CacheReadTokens < 0 {
		consumption.CacheReadTokens = 0
	}
	if consumption.TotalTokens == 0 && (consumption.InputTokens > 0 || consumption.OutputTokens > 0) {
		consumption.TotalTokens = consumption.InputTokens + consumption.OutputTokens
	}
	if consumption.TotalTokens == 0 && (consumption.CacheCreationTokens > 0 || consumption.CacheReadTokens > 0) {
		consumption.TotalTokens = consumption.CacheCreationTokens + consumption.CacheReadTokens
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
		existing[i].CacheCreationTokens += int64(consumption.CacheCreationTokens)
		existing[i].CacheReadTokens += int64(consumption.CacheReadTokens)
		return trimDailyUsage(existing)
	}

	next := append(existing, DailyTokenUsage{
		Date:                day,
		RequestCount:        1,
		InputTokens:         int64(consumption.InputTokens),
		OutputTokens:        int64(consumption.OutputTokens),
		TotalTokens:         int64(consumption.TotalTokens),
		CacheCreationTokens: int64(consumption.CacheCreationTokens),
		CacheReadTokens:     int64(consumption.CacheReadTokens),
	})
	return trimDailyUsage(next)
}

func trimDailyUsage(existing []DailyTokenUsage) []DailyTokenUsage {
	const maxDays = 365
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
