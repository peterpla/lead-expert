package memory

import "github.com/peterpla/lead-expert/pkg/request"

// Memory storage keeps data in memory
type Storage struct {
	// In-memory representation of incoming request queue
	requests []request.Request
}

// Add saves the request to the repository
func (m *Storage) AddRequest(req request.Request) error {
	// log.Printf("memory.AddRequest - enter\n")

	// req has been validated previously, so can simply add it

	m.requests = append(m.requests, req)
	// log.Printf("memory.AddRequest exiting, requests: %+v\n", m.requests)

	return nil
}
