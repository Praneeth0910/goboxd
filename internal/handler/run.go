package handler

import (
	"encoding/json"
	"net/http"
)

// RunRequest is the POST /run request body.
type RunRequest struct {
	Language string `json:"language"`
	Source   string `json:"source"`
}

// RunResponse is the POST /run response body.
type RunResponse struct {
	Status string `json:"status"`
}

// Run handles POST /run requests.
func Run(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(RunResponse{Status: "not_implemented"})
}
