package store

import "errors"

// ErrInvalidTransition: intento de salto no permitido por la matriz.
var ErrInvalidTransition = errors.New("transición inválida")

// Transiciones válidas entre estados (equivalente a web/src/models/transitions.ts)
var Transitions = map[string][]string{
	"open":        {"in_progress", "closed"},
	"in_progress": {"resolved", "open"},
	"resolved":    {"closed", "reopened"},
	"closed":      {"reopened"},
	"reopened":    {"in_progress", "closed"},
}

var Statuses = map[string]bool{
	"open": true, "in_progress": true, "resolved": true, "closed": true, "reopened": true,
}

var Priorities = map[string]bool{
	"low": true, "medium": true, "high": true, "critical": true,
}

func ValidStatus(s string) bool { return Statuses[s] }
func ValidPriority(p string) bool { return Priorities[p] }

func AllowedTransition(from, to string) bool {
	for _, t := range Transitions[from] {
		if t == to {
			return true
		}
	}
	return false
}
