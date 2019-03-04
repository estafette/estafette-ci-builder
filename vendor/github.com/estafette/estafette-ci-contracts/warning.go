package contracts

// Warning is used to issue warnings for not following best practices
type Warning struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
