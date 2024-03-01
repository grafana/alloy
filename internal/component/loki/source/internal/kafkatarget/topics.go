package kafkatarget

// This code is copied from Promtail (https://github.com/grafana/loki/commit/065bee7e72b00d800431f4b70f0d673d6e0e7a2b). The kafkatarget package is used to
// configure and run the targets that can read kafka entries and forward them
// to other loki components.

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
)

type topicClient interface {
	RefreshMetadata(topics ...string) error
	Topics() ([]string, error)
}

type topicManager struct {
	client topicClient

	patterns []*regexp.Regexp
	matches  []string
}

// newTopicManager fetches topics and returns matchings one based on list of requested topics.
// If a topic starts with a '^' it is treated as a regexp and can match multiple topics.
func newTopicManager(client topicClient, topics []string) (*topicManager, error) {
	var (
		patterns []*regexp.Regexp
		matches  []string
	)
	for _, t := range topics {
		if len(t) == 0 {
			return nil, errors.New("invalid empty topic")
		}
		if t[0] != '^' {
			matches = append(matches, t)
		}
		re, err := regexp.Compile(t)
		if err != nil {
			return nil, fmt.Errorf("invalid topic pattern: %w", err)
		}
		patterns = append(patterns, re)
	}
	return &topicManager{
		client:   client,
		patterns: patterns,
		matches:  matches,
	}, nil
}

func (tm *topicManager) Topics() ([]string, error) {
	if err := tm.client.RefreshMetadata(); err != nil {
		return nil, err
	}
	topics, err := tm.client.Topics()
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(topics))

Outer:
	for _, topic := range topics {
		for _, m := range tm.matches {
			if m == topic {
				result = append(result, topic)
				continue Outer
			}
		}
		for _, p := range tm.patterns {
			if p.MatchString(topic) {
				result = append(result, topic)
				continue Outer
			}
		}
	}

	sort.Strings(result)
	return result, nil
}
