package handlers

import (
	"net/http"

	"example.com/highload/myproject/internal/store"
)

type Stats struct {
	Users    int `json:"users"`
	Profiles int `json:"profiles"`
}

type ReplicaPool = *store.ReplicaPool

type StatsHandler struct {
	replica ReplicaPool
}

func NewStatsHandler(replica ReplicaPool) *StatsHandler {
	return &StatsHandler{replica: replica}
}

func (h *StatsHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	var usersCount int
	if err := h.replica.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&usersCount); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var profilesCount int
	if err := h.replica.QueryRow(ctx, "SELECT COUNT(*) FROM profile").Scan(&profilesCount); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stats := Stats{Users: usersCount, Profiles: profilesCount}
	writeJSON(w, http.StatusOK, stats)
}
