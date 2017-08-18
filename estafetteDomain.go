package main

// EstafetteCiBuilderEvent represents a finished estafette build
type EstafetteCiBuilderEvent struct {
	JobName string `json:"job_name"`
}
