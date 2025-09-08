package api

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"virtualservers/internal/repository"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	Store *repository.Store
}
type actionReq struct {
	Action string `json:"action"`
}

type createReq struct {
	Name   string `json:"name"`
	Region string `json:"region"`
	Type   string `json:"type"`
}

func (h *Handler) ListServers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	f := repository.ListFilters{
		Region: q.Get("region"),
		Status: strings.TrimSpace(q.Get("status")),
		Type:   q.Get("type"),
		Limit:  limit,
		Offset: offset,
	}
	items, total, err := h.Store.ListServers(r.Context(), f)
	if err != nil {
		log.Printf("ListServers error: %v", err) // <-- add this
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	resp := map[string]any{
		"items":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	srv, err := h.Store.GetServerByID(r.Context(), id)
	if err != nil {
		log.Printf("GetServer error:%v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if srv == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(srv)
}

func (h *Handler) ServerAction(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req actionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	newStatus, err := h.Store.ApplyAction(r.Context(), id, action)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if strings.Contains(err.Error(), "invalid transition") {
			http.Error(w, "invalid transition", http.StatusConflict)
			return
		}
		log.Printf("ServerAction error:%v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	resp := map[string]string{
		"id":     id,
		"status": newStatus,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetServerLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	events, err := h.Store.GetServerLogs(r.Context(), id)
	if err != nil {
		log.Printf("GetServerLogs error:%v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

func (h *Handler) CreateServer(w http.ResponseWriter, r *http.Request) {
	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Region == "" || req.Type == "" {
		http.Error(w, "missing fields (name ,type required)", http.StatusBadRequest)
		return
	}
	id, err := h.Store.CreateServer(r.Context(), req.Name, req.Region, req.Type)
	if err != nil {
		log.Printf("CreateServer error :%v", err)
		http.Error(w, "could not create server", http.StatusInternalServerError)
		return
	}
	resp := map[string]string{
		"id":     id,
		"status": "STOPPED",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
