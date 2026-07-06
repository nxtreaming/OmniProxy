package main

import (
	"strings"

	"omniproxy/internal/token"
)

func (a *appServer) tokenDisplayName(item token.Token) string {
	if a != nil && a.tokens != nil && strings.TrimSpace(item.ID) != "" {
		if latest, err := a.tokens.Get(item.ID); err == nil {
			item = latest
		}
	}
	return token.DisplayName(item)
}
