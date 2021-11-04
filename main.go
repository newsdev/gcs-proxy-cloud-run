// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"context"
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/DomZippilli/gcs-proxy-cloud-function/common"
	"github.com/DomZippilli/gcs-proxy-cloud-function/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	storage "cloud.google.com/go/storage"
)

// https://stackoverflow.com/a/39591234
// BasicAuth wraps a handler requiring HTTP basic auth for it using the given
// username and password and the specified realm, which shouldn't contain quotes.
//
// Most web browser display a dialog with something like:
//
//    The website says: "<realm>"
//
// Which is really stupid so you may want to set the realm to a message rather than
// an actual realm.
//
// Parses a comma separated list of username:password, e.g. "mike:abc123,sam:def456"
func BasicAuth(handler http.HandlerFunc, usernamesPasswords, realm string) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		loginFound := false
		user, pass, ok := r.BasicAuth()

		eachUsernamePassword := strings.Split(usernamesPasswords, ",")
		for _, usernamePassword := range eachUsernamePassword {
			userPass := strings.Split(usernamePassword, ":")
			loginFound = subtle.ConstantTimeCompare([]byte(user), []byte(userPass[0])) == 1 && subtle.ConstantTimeCompare([]byte(pass), []byte(userPass[1])) == 1
			if loginFound {
				break
			}
		}

		if !ok || !loginFound {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			w.WriteHeader(401)
			w.Write([]byte("Unauthorised.\n"))
			return
		}

		handler(w, r)
	}
}

func setup() {
	// set the bucket name from environment variable
	common.BUCKET = os.Getenv("BUCKET_NAME")

	// initialize the client
	c, err := storage.NewClient(context.Background())
	if err != nil {
		log.Fatal().Msgf("main: %v", err)
	}
	common.GCS = c
}

func main() {
	// pretty print console log
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	// initialize
	log.Print("starting server...")
	setup()
	http.HandleFunc("/", BasicAuth(ProxyHTTPGCS, os.Getenv("USERPASS"), "Sign in"))

	// Determine port for HTTP service.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Warn().Msgf("defaulting to port %s", port)
	}

	// Start HTTP server.
	log.Printf("listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal().Msgf("main: %v", err)
	}
}

// ProxyHTTPGCS is the entry point for the cloud function, providing a proxy that
// permits HTTP protocol usage of a GCS bucket's contents.
func ProxyHTTPGCS(output http.ResponseWriter, input *http.Request) {
	ctx := context.Background()
	// route HTTP methods to appropriate handlers.
	// ===> Your filters go below here <===
	switch input.Method {
	case http.MethodGet:
		config.GET(ctx, output, input)
	default:
		http.Error(output, "405 - Method Not Allowed", http.StatusMethodNotAllowed)
	}
	return
}
