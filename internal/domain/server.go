package domain

import (
	"fmt"
	"time"
)

type ServerStatus string

const (
	StatusPending    ServerStatus = "pending"
	StatusRunning    ServerStatus = "running"
	StatusStopped    ServerStatus = "stopped"
	StatusRebooting  ServerStatus = "rebooting"
	StatusTerminated ServerStatus = "terminated"
)

type Server struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Status    ServerStatus `json:"status"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// State transition map
var validTransitions = map[ServerStatus][]ServerStatus{
	StatusPending:    {StatusStopped},
	StatusStopped:    {StatusRunning, StatusTerminated},
	StatusRunning:    {StatusStopped, StatusRebooting},
	StatusRebooting:  {StatusRunning},
	StatusTerminated: {},
}

// Checking if a state transition is valid
func (s ServerStatus) CanTransition(target ServerStatus) bool {
	allowedStates, exists := validTransitions[s]
	if !exists {
		return false
	}

	for _, allowed := range allowedStates {
		if allowed == target {
			return true
		}
	}
	return false
}

// Function attempts to change server status
func (server *Server) TransitionTo(newStatus ServerStatus) error {
	if !server.Status.CanTransition(newStatus) {
		return fmt.Errorf("invalid transition from %s to %s ", server.Status, newStatus)

	}
	server.Status = newStatus
	server.UpdatedAt = time.Now()
	return nil
}
