package common

import (
	"errors"
	"io"
	"net/http"
	"time"
)

type Unmarshaler interface {
	Unmarshal([]byte) error
}

const DefaultRetryInterval = 100 * time.Millisecond
const DefaultTimeout = time.Minute

func FetchDataFromURL(url string, target Unmarshaler) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("Non-OK HTTP status: " + resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return target.Unmarshal(bodyBytes)
}
