package contracts

//Label represents a key/value pair as set in a build manifest
type Label struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
