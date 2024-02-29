package kafkatarget

// This code is copied from Promtail (https://github.com/grafana/loki/commit/065bee7e72b00d800431f4b70f0d673d6e0e7a2b). The kafkatarget package is used to
// configure and run the targets that can read kafka entries and forward them
// to other loki components.

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockKafkaClient struct {
	topics []string
	err    error

	mut sync.RWMutex
}

func (m *mockKafkaClient) RefreshMetadata(topics ...string) error {
	return nil
}

func (m *mockKafkaClient) Topics() ([]string, error) {
	m.mut.RLock()
	defer m.mut.RUnlock()
	return m.topics, m.err
}

func (m *mockKafkaClient) UpdateTopics(topics []string) {
	m.mut.Lock()
	defer m.mut.Unlock()
	m.topics = topics
}

func Test_NewTopicManager(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		in          []string
		expectedErr bool
	}{
		{
			[]string{""},
			true,
		},
		{
			[]string{"^("},
			true,
		},
		{
			[]string{"foo"},
			false,
		},
		{
			[]string{"foo", "^foo.*"},
			false,
		},
	} {
		tt := tt
		t.Run(strings.Join(tt.in, ","), func(t *testing.T) {
			t.Parallel()
			_, err := newTopicManager(&mockKafkaClient{}, tt.in)
			if tt.expectedErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func Test_Topics(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		manager     *topicManager
		expected    []string
		expectedErr bool
	}{
		{
			mustNewTopicsManager(&mockKafkaClient{err: errors.New("")}, []string{"foo"}),
			[]string{},
			true,
		},
		{
			mustNewTopicsManager(&mockKafkaClient{topics: []string{"foo", "foobar", "buzz"}}, []string{"^foo"}),
			[]string{"foo", "foobar"},
			false,
		},
		{
			mustNewTopicsManager(&mockKafkaClient{topics: []string{"foo", "foobar", "buzz"}}, []string{"^foo.*", "buzz"}),
			[]string{"buzz", "foo", "foobar"},
			false,
		},
	} {
		tt := tt
		t.Run("", func(t *testing.T) {
			t.Parallel()

			actual, err := tt.manager.Topics()
			if tt.expectedErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func mustNewTopicsManager(client topicClient, topics []string) *topicManager {
	t, err := newTopicManager(client, topics)
	if err != nil {
		panic(err)
	}
	return t
}
