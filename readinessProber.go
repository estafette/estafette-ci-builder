package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

func waitForReadiness(protocol, host string, port int, path, hostname string, timeoutSeconds int) error {

	if protocol == "" {
		return fmt.Errorf("Protocol is empty, should be either http or https")
	}
	if host == "" {
		return fmt.Errorf("Host is empty, should be the name (or alias) of the service")
	}
	if port <= 0 {
		return fmt.Errorf("Port should be larger than zero")
	}
	if path == "" {
		return fmt.Errorf("Path is empty, should be at least / or a path to a readiness endpoint")
	}
	if timeoutSeconds <= 0 {
		return fmt.Errorf("Timeout should be larger than zero")
	}

	readinessURL := fmt.Sprintf("%v://%v:%v%v", protocol, host, port, path)

	log.Info().Msgf("Running readiness probe against %v with host header %v", readinessURL, hostname)

	// create http client and request
	var httpClient = &http.Client{
		Timeout: time.Second * 2,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	request, err := http.NewRequest("GET", readinessURL, nil)
	if err != nil {
		return err
	}
	if hostname != "" && hostname != "host" {
		request.Header.Add("Host", hostname)
	}

	ready := make(chan bool, 1)
	quit := make(chan bool)

	go func(request *http.Request) {

		// perform request
		resp, err := httpClient.Do(request)

		// keep sending request until it succeeds or the total timeout has send a quit signal
		for err != nil || resp.StatusCode != http.StatusOK {
			log.Warn().Err(err).Msgf("Readiness probe against %v failed", request.URL)
			time.Sleep(2 * time.Second)

			select {
			case <-quit:
				return
			default:
				resp, err = httpClient.Do(request)
			}
		}

		// successful request
		ready <- true
	}(request)

	select {
	case <-ready:
		log.Info().Msgf("Readiness probe against %v with host header %v succeeded in time", readinessURL, hostname)
	case <-time.After(time.Duration(timeoutSeconds) * time.Second):
		quit <- true
		return fmt.Errorf("Readiness probe against %v with host header %v did not succeed in %vs", readinessURL, hostname, timeoutSeconds)
	}

	return nil
}
