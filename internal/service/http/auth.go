package http

import (
	"crypto/sha256"
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

func basicAuthenticator(username, password string) authenticator {
	// We hash both expected and incoming data to prevent timing attacks, otherwise
	// a caller can figure out the length of both password and username.
	expectedUsername := sha256.Sum256([]byte(username))
	expectedPassword := sha256.Sum256([]byte(password))

	return func(w http.ResponseWriter, r *http.Request) error {
		username, password, ok := r.BasicAuth()
		if !ok {
			return errors.New("unauthorized")
		}

		usernameHash := sha256.Sum256([]byte(username))
		passwordHash := sha256.Sum256([]byte(password))

		usernameMatch := subtle.ConstantTimeCompare(usernameHash[:], expectedUsername[:]) == 1
		passwordMatch := subtle.ConstantTimeCompare(passwordHash[:], expectedPassword[:]) == 1

		if !usernameMatch || !passwordMatch {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			return errors.New("unauthorized")
		}

		return nil
	}
}
