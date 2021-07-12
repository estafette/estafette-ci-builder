package builder

import (
	"os"

	"github.com/rs/zerolog/log"
)

// LocalFatalHandler has methods to shutdown the runner after a fatal or successful run
type LocalFatalHandler interface {
	HandleFatal(error, string)
}

type localFatalHandler struct {
}

// NewLocalFatalHandler returns a new LocalFatalHandler
func NewLocalFatalHandler() LocalFatalHandler {
	return &localFatalHandler{}
}

func (elh *localFatalHandler) HandleFatal(err error, message string) {
	log.Fatal().Err(err).Msg(message)
	os.Exit(1)
}
