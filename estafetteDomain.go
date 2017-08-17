package main

// EstafetteBuildFinishedEvent represents a finished estafette build
type EstafetteBuildFinishedEvent struct {
	JobName string `json:"job_name"`
}
