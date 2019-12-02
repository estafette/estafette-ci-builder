package builder

type Builder interface {
}

type builderImpl struct {
}

// NewBuilder returns a new Builder
func NewBuilder() Builder {
	return &builderImpl{}
}
