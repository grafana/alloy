package model

import (
	"fmt"
	"io"
	"strconv"
	"time"
)

// Duration is a GraphQL duration encoded using time.Duration's string format.
type Duration time.Duration

func (d Duration) MarshalGQL(w io.Writer) {
	_, _ = io.WriteString(w, strconv.Quote(time.Duration(d).String()))
}

func (d *Duration) UnmarshalGQL(value any) error {
	text, ok := value.(string)
	if !ok {
		return fmt.Errorf("duration must be a string")
	}
	parsed, err := time.ParseDuration(text)
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}
