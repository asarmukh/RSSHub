package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
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

type Aggregator interface {
	SetInterval(d time.Duration)
	Resize(workers int) error
	CurrentInterval() time.Duration
	CurrentWorkers() int
}

type Server struct {
	agg Aggregator
}

func NewServer(agg Aggregator) *Server { return &Server{agg: agg} }

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
	if req.Workers <= 0 {
		http.Error(w, "workers must be > 0", http.StatusBadRequest)
		return
	}
	old := s.agg.CurrentWorkers()
	if err := s.agg.Resize(req.Workers); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "old": old, "new": req.Workers})
}
