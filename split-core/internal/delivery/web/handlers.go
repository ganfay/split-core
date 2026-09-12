package web

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type Request struct {
	Pong   bool `json:"pong"`
}

func (s *ServerHandler) Ping(w http.ResponseWriter, _ *http.Request) {
	var req Request

	req = Request{
		Pong:   true,
	}

	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(req)
	if err != nil {
		slog.Error("Failed to encode json ", err)
		return
	}
}
