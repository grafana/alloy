package api

import (
	"errors"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/scrape"
	"github.com/grafana/alloy/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Simple stub implementation of service.Host
type StubHost struct {
	// Data to return from ListComponents
	components []*component.Info
	listError  error

	// Map of component IDs to their info and errors
	componentInfos  map[string]*component.Info
	componentErrors map[string]error
}

// Create a new stub with sensible defaults
func NewStubHost() *StubHost {
	return &StubHost{
		components:      []*component.Info{},
		componentInfos:  make(map[string]*component.Info),
		componentErrors: make(map[string]error),
	}
}

// Only implement the methods actually used by getLocalTargetsDebugInfo
func (s *StubHost) ListComponents(moduleID string, opts component.InfoOptions) ([]*component.Info, error) {
	return s.components, s.listError
}

func (s *StubHost) GetComponent(id component.ID, opts component.InfoOptions) (*component.Info, error) {
	info, exists := s.componentInfos[id.String()]
	if !exists {
		return nil, errors.New("component not found")
	}
	return info, s.componentErrors[id.String()]
}

// Stub implementations of other required methods
func (s *StubHost) GetService(name string) (service.Service, bool) {
	return nil, false
}

func (s *StubHost) GetServiceConsumers(name string) []component.Consumer {
	return nil
}

func (s *StubHost) ListServices() map[string]service.Service {
	return nil
}

func (s *StubHost) ListModules() []*component.Module {
	return nil
}

func (s *StubHost) Name() string {
	return "stub-host"
}

func TestGetLocalTargetsDebugInfo(t *testing.T) {
	// Create sample test data
	scrapeTime := time.Now()
	scrapeDuration := 100 * time.Millisecond

	// Define test cases
	testCases := []struct {
		name          string
		setupStub     func() *StubHost
		query         string
		expected      PrometheusTargetDebugResponse
		expectedError string
	}{
		{
			name: "returns all targets with empty query",
			setupStub: func() *StubHost {
				stub := NewStubHost()

				// Create components
				compIDA := component.ParseID("prometheus.scrape/component_a")
				compIDB := component.ParseID("prometheus.scrape/component_b")

				// Create target statuses
				targetA := scrape.TargetStatus{
					JobName: "job_a",
					URL:     "http://example.com:9090/metrics",
					Health:  "up",
					Labels: map[string]string{
						"instance": "example.com:9090",
						"env":      "production",
					},
					LastScrape:         scrapeTime,
					LastScrapeDuration: scrapeDuration,
				}

				targetB := scrape.TargetStatus{
					JobName: "job_b",
					URL:     "http://test.com:9100/metrics",
					Health:  "up",
					Labels: map[string]string{
						"instance": "test.com:9100",
						"env":      "staging",
					},
					LastError:          "timeout",
					LastScrape:         scrapeTime,
					LastScrapeDuration: 2 * scrapeDuration,
				}

				// Create component infos
				infoA := &component.Info{
					ID:            compIDA,
					ComponentName: "prometheus.scrape",
					DebugInfo:     scrape.ScraperStatus{TargetStatus: []scrape.TargetStatus{targetA}},
				}

				infoB := &component.Info{
					ID:            compIDB,
					ComponentName: "prometheus.scrape",
					DebugInfo:     scrape.ScraperStatus{TargetStatus: []scrape.TargetStatus{targetB}},
				}

				// Configure stub
				stub.components = []*component.Info{infoA, infoB}
				stub.componentInfos = map[string]*component.Info{
					compIDA.String(): infoA,
					compIDB.String(): infoB,
				}

				return stub
			},
			query: "",
			expected: PrometheusTargetDebugResponse{
				Components: map[string]ComponentDebugInfo{
					"prometheus.scrape/component_a": {
						TargetsStatus: []TargetStatus{
							{
								JobName: "job_a",
								URL:     "http://example.com:9090/metrics",
								Health:  "up",
								Labels: map[string]string{
									"instance": "example.com:9090",
									"env":      "production",
								},
								LastError:          "",
								LastScrape:         scrapeTime.Format(time.RFC3339),
								LastScrapeDuration: scrapeDuration.String(),
							},
						},
					},
					"prometheus.scrape/component_b": {
						TargetsStatus: []TargetStatus{
							{
								JobName: "job_b",
								URL:     "http://test.com:9100/metrics",
								Health:  "up",
								Labels: map[string]string{
									"instance": "test.com:9100",
									"env":      "staging",
								},
								LastError:          "timeout",
								LastScrape:         scrapeTime.Format(time.RFC3339),
								LastScrapeDuration: (2 * scrapeDuration).String(),
							},
						},
					},
				},
			},
		},
		{
			name: "filters by URL",
			setupStub: func() *StubHost {
				stub := NewStubHost()

				// Create components with different targets
				compID := component.ParseID("prometheus.scrape/comp")

				// Create target statuses
				targetA := scrape.TargetStatus{
					JobName:    "job_a",
					URL:        "http://example.com:9090/metrics",
					Health:     "up",
					Labels:     map[string]string{"env": "production"},
					LastScrape: scrapeTime,
				}

				targetB := scrape.TargetStatus{
					JobName:    "job_b",
					URL:        "http://test.com:9100/metrics",
					Health:     "up",
					Labels:     map[string]string{"env": "staging"},
					LastScrape: scrapeTime,
				}

				// Create component info
				info := &component.Info{
					ID:            compID,
					ComponentName: "prometheus.scrape",
					DebugInfo:     scrape.ScraperStatus{TargetStatus: []scrape.TargetStatus{targetA, targetB}},
				}

				// Configure stub
				stub.components = []*component.Info{info}
				stub.componentInfos = map[string]*component.Info{
					compID.String(): info,
				}

				return stub
			},
			query: "test.com",
			expected: PrometheusTargetDebugResponse{
				Components: map[string]ComponentDebugInfo{
					"prometheus.scrape/comp": {
						TargetsStatus: []TargetStatus{
							{
								JobName:            "job_b",
								URL:                "http://test.com:9100/metrics",
								Health:             "up",
								Labels:             map[string]string{"env": "staging"},
								LastScrape:         scrapeTime.Format(time.RFC3339),
								LastScrapeDuration: "0s",
							},
						},
					},
				},
			},
		},
		{
			name: "filters by label",
			setupStub: func() *StubHost {
				stub := NewStubHost()

				// Create components with targets having different labels
				compID := component.ParseID("prometheus.scrape/comp")

				// Create target statuses
				targetA := scrape.TargetStatus{
					JobName:    "job_a",
					URL:        "http://example.com:9090/metrics",
					Health:     "up",
					Labels:     map[string]string{"env": "production"},
					LastScrape: scrapeTime,
				}

				targetB := scrape.TargetStatus{
					JobName:    "job_b",
					URL:        "http://test.com:9100/metrics",
					Health:     "up",
					Labels:     map[string]string{"env": "development"},
					LastScrape: scrapeTime,
				}

				// Create component info
				info := &component.Info{
					ID:            compID,
					ComponentName: "prometheus.scrape",
					DebugInfo:     scrape.ScraperStatus{TargetStatus: []scrape.TargetStatus{targetA, targetB}},
				}

				// Configure stub
				stub.components = []*component.Info{info}
				stub.componentInfos = map[string]*component.Info{
					compID.String(): info,
				}

				return stub
			},
			query: "development",
			expected: PrometheusTargetDebugResponse{
				Components: map[string]ComponentDebugInfo{
					"prometheus.scrape/comp": {
						TargetsStatus: []TargetStatus{
							{
								JobName:            "job_b",
								URL:                "http://test.com:9100/metrics",
								Health:             "up",
								Labels:             map[string]string{"env": "development"},
								LastScrape:         scrapeTime.Format(time.RFC3339),
								LastScrapeDuration: "0s",
							},
						},
					},
				},
			},
		},
		{
			name: "handles error listing components",
			setupStub: func() *StubHost {
				stub := NewStubHost()
				stub.listError = errors.New("failed to list components")
				return stub
			},
			query:         "",
			expectedError: "failed to list Prometheus components: failed to list components",
			expected:      PrometheusTargetDebugResponse{Components: map[string]ComponentDebugInfo{}},
		},
		{
			name: "handles error getting component info",
			setupStub: func() *StubHost {
				stub := NewStubHost()

				compID := component.ParseID("prometheus.scrape/comp")
				info := &component.Info{
					ID:            compID,
					ComponentName: "prometheus.scrape",
				}

				stub.components = []*component.Info{info}
				stub.componentInfos = map[string]*component.Info{
					compID.String(): info,
				}
				stub.componentErrors = map[string]error{
					compID.String(): errors.New("failed to get component info"),
				}

				return stub
			},
			query: "",
			expected: PrometheusTargetDebugResponse{
				Components: map[string]ComponentDebugInfo{},
				Errors:     []string{"failed to get info for component prometheus.scrape/comp: failed to get component info"},
			},
		},
		{
			name: "handles invalid debug info type",
			setupStub: func() *StubHost {
				stub := NewStubHost()

				compID := component.ParseID("prometheus.scrape/comp")
				info := &component.Info{
					ID:            compID,
					ComponentName: "prometheus.scrape",
					DebugInfo:     "not a scraper status", // Invalid debug info type
				}

				stub.components = []*component.Info{info}
				stub.componentInfos = map[string]*component.Info{
					compID.String(): info,
				}

				return stub
			},
			query: "",
			expected: PrometheusTargetDebugResponse{
				Components: map[string]ComponentDebugInfo{},
				Errors:     []string{"component prometheus.scrape/comp does not have expected scrape debug info"},
			},
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup the stub
			stubHost := tc.setupStub()

			// Call the function
			result, err := getLocalTargetsDebugInfo(stubHost, tc.query)

			// Check error
			if tc.expectedError != "" {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError, err.Error())
			} else {
				require.NoError(t, err)
			}

			// Check result
			assert.Equal(t, tc.expected, result)
		})
	}
}
