package http

import (
	"crypto/subtle"
	"errors"
	"net/http"

	"github.com/grafana/alloy/syntax/alloytypes"
)

type AuthArguments struct {
	Basic *BasicAuthArguments `alloy:"basic,block,optional"`
	// Filter is used to apply authentication to matching api endpoints
	Filter []string `alloy:"filter,attr,optional"`
}

type BasicAuthArguments struct {
	Username string            `alloy:"username,attr"`
	Password alloytypes.Secret `alloy:"password,attr"`
}

func (a *AuthArguments) authenticator() authenticator {
	if a.Basic != nil {
		return basicAuthenticator(a.Basic.Username, string(a.Basic.Password))
	}
	return allowAuthenticator
}

type authenticator func(w http.ResponseWriter, r *http.Request) error

func allowAuthenticator(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func basicAuthenticator(expectedUsername, expectedPassword string) authenticator {
	return func(w http.ResponseWriter, r *http.Request) error {
		username, password, ok := r.BasicAuth()
		if !ok {
			return errors.New("unauthorized")
		}

		usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(expectedUsername)) == 1
		passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(expectedPassword)) == 1

		if !usernameMatch || !passwordMatch {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			return errors.New("unauthorized")
		}
		return nil
	}
}
