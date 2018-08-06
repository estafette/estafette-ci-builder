package contracts

// GitCommit represents a commit summary
type GitCommit struct {
	Message string    `json:"message"`
	Author  GitAuthor `json:"author"`
}

// GitAuthor represents the author of a commmit
type GitAuthor struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Username string `json:"username"`
}
