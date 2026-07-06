package main

import (
	"context"
	"fmt"

	"omniproxy/internal/logs"
	"omniproxy/internal/token"
)

func (a *appServer) createToken(ctx context.Context, req token.UpsertRequest) (tokenResponse, error) {
	item, err := a.tokens.Add(req)
	if err != nil {
		return tokenResponse{}, err
	}
	a.logs.Add(logs.Entry{Level: logs.LevelInfo, TokenName: a.tokenDisplayName(item), Message: "token added"})
	if isRefreshableAuthToken(item) {
		result, err := a.validateAndRecordToken(ctx, item)
		a.recordTokenMaintenanceHistory(historyEventCodexRefreshAdd, item, result, err)
		if err != nil {
			a.logs.Add(logs.Entry{Level: logs.LevelWarn, TokenName: a.tokenDisplayName(item), Message: fmt.Sprintf("OAuth validation failed after add: %v", err)})
		}
		if updated, err := a.tokens.Get(item.ID); err == nil {
			item = updated
		}
	}
	a.ensurePremProxyForToken(item, "Prem account added")
	return tokenResponseFor(item), nil
}

func (a *appServer) updateToken(id string, req token.UpsertRequest) (tokenResponse, error) {
	item, err := a.tokens.Update(id, req)
	if err != nil {
		return tokenResponse{}, err
	}
	a.logs.Add(logs.Entry{Level: logs.LevelInfo, TokenName: a.tokenDisplayName(item), Message: "token updated"})
	a.ensurePremProxyForToken(item, "Prem account updated")
	return tokenResponseFor(item), nil
}

func (a *appServer) deleteToken(id string) error {
	if err := a.tokens.Delete(id); err != nil {
		return err
	}
	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "token deleted"})
	a.stopPremProxyIfUnused()
	return nil
}

func (a *appServer) setTokenDisabled(id string, disabled bool) (tokenResponse, error) {
	item, err := a.tokens.SetDisabled(id, disabled)
	if err != nil {
		return tokenResponse{}, err
	}
	message := "token enabled"
	if item.Disabled {
		message = "token disabled"
	}
	a.logs.Add(logs.Entry{Level: logs.LevelInfo, TokenName: a.tokenDisplayName(item), Message: message})
	if item.Disabled {
		a.stopPremProxyIfUnused()
	} else {
		a.ensurePremProxyForToken(item, "Prem account enabled")
	}
	return tokenResponseFor(item), nil
}

func (a *appServer) useOnlyToken(id string) ([]tokenResponse, error) {
	item, err := a.tokens.Get(id)
	if err != nil {
		return nil, err
	}
	items, err := a.tokens.SelectOnly(id)
	if err != nil {
		return nil, err
	}
	a.logs.Add(logs.Entry{Level: logs.LevelInfo, TokenName: a.tokenDisplayName(item), Message: "token selected as only provider account"})
	return tokenResponses(items), nil
}

func (a *appServer) cancelUseOnlyToken(id string) ([]tokenResponse, error) {
	item, err := a.tokens.Get(id)
	if err != nil {
		return nil, err
	}
	items, err := a.tokens.ClearProviderSelectionForToken(id)
	if err != nil {
		return nil, err
	}
	a.logs.Add(logs.Entry{Level: logs.LevelInfo, TokenName: a.tokenDisplayName(item), Message: "provider account selection cleared"})
	return tokenResponses(items), nil
}

func (a *appServer) setTokenSelected(id string, selected bool) ([]tokenResponse, error) {
	item, err := a.tokens.Get(id)
	if err != nil {
		return nil, err
	}
	items, err := a.tokens.SetSelected(id, selected)
	if err != nil {
		return nil, err
	}
	message := "token removed from provider selection"
	if selected {
		message = "token added to provider selection"
	}
	a.logs.Add(logs.Entry{Level: logs.LevelInfo, TokenName: a.tokenDisplayName(item), Message: message})
	return tokenResponses(items), nil
}

func (a *appServer) validateToken(ctx context.Context, id string) (validationResponse, error) {
	selected, err := a.tokens.Get(id)
	if err != nil {
		return validationResponse{}, err
	}

	result, err := a.validateAndRecordToken(ctx, selected)
	a.recordTokenMaintenanceHistory(historyEventManualValidation, selected, result, err)

	level := logs.LevelInfo
	if err != nil || !result.OK {
		level = logs.LevelWarn
	}
	a.logs.Add(logs.Entry{
		Level:     level,
		Status:    result.Status,
		Duration:  result.Duration,
		TokenName: a.tokenDisplayName(selected),
		Message:   "token validation completed",
	})
	return validationResponseFor(result), err
}

func (a *appServer) refreshAuthTokenResponse(ctx context.Context, id string) (tokenResponse, error) {
	item, err := a.refreshStoredAuthToken(ctx, id)
	if err != nil {
		return tokenResponse{}, err
	}
	return tokenResponseFor(item), nil
}
