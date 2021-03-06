// Copyright 2014-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//      http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package ipamd

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/aws/amazon-vpc-cni-k8s/pkg/networkutils"
	"github.com/aws/amazon-vpc-cni-k8s/pkg/utils"
)

const (
	// IntrospectionPort is the port for ipamd introspection
	IntrospectionPort = 61678
)

type rootResponse struct {
	AvailableCommands []string
}

// LoggingHandler is a object for handling http request
type LoggingHandler struct{ h http.Handler }

// NewLoggingHandler creates a new LoggingHandler object.
func NewLoggingHandler(handler http.Handler) LoggingHandler {
	return LoggingHandler{h: handler}
}

func (lh LoggingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Info("Handling http request", "method", r.Method, "from", r.RemoteAddr, "uri", r.RequestURI)
	lh.h.ServeHTTP(w, r)
}

// SetupHTTP sets up ipamd introspection service endpoint
func (c *IPAMContext) SetupHTTP() {
	server := c.setupServer()

	for {
		once := sync.Once{}
		utils.RetryWithBackoff(utils.NewSimpleBackoff(time.Second, time.Minute, 0.2, 2), func() error {
			// TODO, make this cancellable and use the passed in context; for
			// now, not critical if this gets interrupted
			err := server.ListenAndServe()
			once.Do(func() {
				log.Error("Error running http api", "err", err)
			})
			return err
		})
	}
}

func (c *IPAMContext) setupServer() *http.Server {
	serverFunctions := map[string]func(w http.ResponseWriter, r *http.Request){
		"/v1/enis":                      eniV1RequestHandler(c),
		"/v1/pods":                      podV1RequestHandler(c),
		"/v1/networkutils-env-settings": networkEnvV1RequestHandler(c),
		"/v1/ipamd-env-settings":        ipamdEnvV1RequestHandler(c),
		"/v1/eni-configs":               eniConfigRequestHandler(c),
	}
	paths := make([]string, 0, len(serverFunctions))
	for path := range serverFunctions {
		paths = append(paths, path)
	}
	availableCommands := &rootResponse{paths}
	// Autogenerated list of the above serverFunctions paths
	availableCommandResponse, err := json.Marshal(&availableCommands)

	if err != nil {
		log.Error("Failed to Marshal: %v", err)
	}

	defaultHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Write(availableCommandResponse)
	}

	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", defaultHandler)
	for key, fn := range serverFunctions {
		serveMux.HandleFunc(key, fn)
	}
	serveMux.Handle("/metrics", promhttp.Handler())

	// Log all requests and then pass through to serveMux
	loggingServeMux := http.NewServeMux()
	loggingServeMux.Handle("/", LoggingHandler{serveMux})

	server := &http.Server{
		Addr:         ":" + strconv.Itoa(IntrospectionPort),
		Handler:      loggingServeMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	return server
}

func eniV1RequestHandler(ipam *IPAMContext) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		responseJSON, err := json.Marshal(ipam.dataStore.GetENIInfos())
		if err != nil {
			log.Error("Failed to marshal ENI data: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Write(responseJSON)
	}
}

func podV1RequestHandler(ipam *IPAMContext) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		responseJSON, err := json.Marshal(ipam.dataStore.GetPodInfos())
		if err != nil {
			log.Error("Failed to marshal pod data: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Write(responseJSON)
	}
}

func eniConfigRequestHandler(ipam *IPAMContext) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		responseJSON, err := json.Marshal(ipam.eniConfig.Getter())
		if err != nil {
			log.Error("Failed to marshal pod data: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Write(responseJSON)
	}
}

func networkEnvV1RequestHandler(ipam *IPAMContext) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		responseJSON, err := json.Marshal(networkutils.GetConfigForDebug())
		if err != nil {
			log.Error("Failed to marshal env var data: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Write(responseJSON)
	}
}

func ipamdEnvV1RequestHandler(ipam *IPAMContext) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		responseJSON, err := json.Marshal(GetConfigForDebug())
		if err != nil {
			log.Error("Failed to marshal env var data: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Write(responseJSON)
	}
}

func metricsHandler(ipam *IPAMContext) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		promhttp.Handler()
	}
}
