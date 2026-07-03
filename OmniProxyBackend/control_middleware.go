package main

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"omniproxy/internal/token"
	"strings"
)

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			if !isAllowedControlOrigin(origin) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,"+controlTokenHeader)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withControlTokenAuth(expected string, next http.Handler) http.Handler {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || isControlTokenEndpoint(r) {
			next.ServeHTTP(w, r)
			return
		}
		if !validControlToken(r, expected) {
			w.Header().Set("Cache-Control", "no-store")
			writeError(w, http.StatusUnauthorized, "control token required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isControlTokenEndpoint(r *http.Request) bool {
	return r.URL != nil && r.URL.Path == "/api/control-token"
}

func validControlToken(r *http.Request, expected string) bool {
	value := strings.TrimSpace(r.Header.Get(controlTokenHeader))
	if value == "" {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		const bearerPrefix = "bearer "
		if len(auth) > len(bearerPrefix) && strings.EqualFold(auth[:len(bearerPrefix)], bearerPrefix) {
			value = strings.TrimSpace(auth[len(bearerPrefix):])
		}
	}
	if value == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(value), []byte(expected)) == 1
}

func isAllowedControlOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	scheme := strings.ToLower(parsed.Scheme)
	if scheme == "wails" && host == "wails.localhost" {
		return true
	}
	if host == "wails.localhost" {
		return true
	}
	if scheme != "http" && scheme != "https" {
		return false
	}
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func isTrustedControlTokenOrigin(origin string) bool {
	parsed, err := url.Parse(strings.TrimSpace(origin))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	scheme := strings.ToLower(parsed.Scheme)
	return host == "wails.localhost" && scheme == "wails"
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, token.ErrDuplicateName):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, token.ErrTokenNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func intToString(value int) string {
	return fmt.Sprintf("%d", value)
}
