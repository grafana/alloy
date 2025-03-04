package common

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

type Unmarshaler interface {
	Unmarshal([]byte) error
}

const DefaultRetryInterval = 100 * time.Millisecond
const DefaultTimeout = 90 * time.Second

func FetchDataFromURL(url string, target Unmarshaler) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Non-OK HTTP status: %s, body: %s, url: %s", resp.Status, string(bodyBytes), url)
	}

	return target.Unmarshal(bodyBytes)
}
