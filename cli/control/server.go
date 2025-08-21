package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"rsshub/domain"
	"time"
)

var ErrAlreadyRunning = errors.New("already running")

// TryListen tries to bind the control address. If it's already in use, we assume an instance is running.
func TryListen(addr string) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, ErrAlreadyRunning
	}
	return ln, nil
}

type Server struct {
	agg domain.Aggregator
}

func NewServer(agg domain.Aggregator) *Server { return &Server{agg: agg} }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/set-interval":
		s.handleSetInterval(w, r)
		return
	case r.Method == http.MethodPost && r.URL.Path == "/set-workers":
		s.handleSetWorkers(w, r)
		return
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleSetInterval(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Duration string `json:"duration"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	d, err := time.ParseDuration(req.Duration)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid duration: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("duration: %s", req.Duration)

	old := s.agg.CurrentInterval()
	s.agg.SetInterval(d)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "old": old.String(), "new": d.String()})
}

func (s *Server) handleSetWorkers(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Workers int `json:"workers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	old := s.agg.CurrentWorkers()
	if err := s.agg.Resize(req.Workers); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "old": old, "new": req.Workers})
}
