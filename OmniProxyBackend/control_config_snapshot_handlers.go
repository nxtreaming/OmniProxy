package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type configSnapshotCreateRequest struct {
	Name string `json:"name"`
}

func (a *appServer) handleConfigSnapshots(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := a.listConfigSnapshots()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var req configSnapshotCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !isEmptyJSONBody(err) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		item, err := a.createConfigSnapshot(req.Name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *appServer) handleConfigSnapshotByID(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/config/snapshots/"), "/")
	if id == "" {
		writeError(w, http.StatusNotFound, "snapshot not found")
		return
	}
	switch r.Method {
	case http.MethodPut:
		cfg, err := a.restoreConfigSnapshot(id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, cfg)
	case http.MethodDelete:
		if err := a.deleteConfigSnapshot(id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *appServer) handleConfigExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, a.configExportBundle())
}

func (a *appServer) handleConfigImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2*1024*1024)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := a.importConfigBundleBytes(data)
	if err != nil {
		if isConfigValidationError(err) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func isEmptyJSONBody(err error) bool {
	return err == io.EOF
}
