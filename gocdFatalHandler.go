package main

import (
	"os"

	"github.com/rs/zerolog/log"
)

// GocdFatalHandler has methods to shutdown the runner after a fatal or successful run
type GocdFatalHandler interface {
	handleGocdFatal(error, string)
}

type gocdFatalHandlerImpl struct {
}

// NewGocdFatalHandler returns a new GocdFatalHandler
func NewGocdFatalHandler() GocdFatalHandler {
	return &gocdFatalHandlerImpl{}
}

func (elh *gocdFatalHandlerImpl) handleGocdFatal(err error, message string) {

	log.Fatal().Err(err).Msg(message)
	os.Exit(1)
}
