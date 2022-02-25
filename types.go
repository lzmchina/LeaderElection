package main

const (
	DefaultPort = 4040
)

// LeaderData represents information about the current leader
type LeaderData struct {
	Name string `json:"name"`
}
