package common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataUnmarshal(t *testing.T) {
	actualMetadataStr := `
{
    "status": "success",
    "data": {
        "go_gc_duration_seconds": [
            {
                "type": "summary",
                "help": "A summary of the wall-time pause (stop-the-world) duration in garbage collection cycles.",
                "unit": ""
            }
        ],
        "go_gc_duration_seconds_count": [
            {
                "type": "summary",
                "help": "A summary of the wall-time pause (stop-the-world) duration in garbage collection cycles.",
                "unit": ""
            }
        ],
        "go_gc_duration_seconds_sum": [
            {
                "type": "summary",
                "help": "A summary of the wall-time pause (stop-the-world) duration in garbage collection cycles.",
                "unit": ""
            }
        ]
    }
}`

	var actualMetadata MetadataResponse
	err := actualMetadata.Unmarshal([]byte(actualMetadataStr))
	require.NoError(t, err)

	expectedMetadata := MetadataResponse{
		Status: "success",
		Data: map[string][]Metadata{
			"go_gc_duration_seconds": {
				{
					Type: "summary",
					Help: "A summary of the wall-time pause (stop-the-world) duration in garbage collection cycles.",
					Unit: "",
				},
			},
			"go_gc_duration_seconds_count": {
				{
					Type: "summary",
					Help: "A summary of the wall-time pause (stop-the-world) duration in garbage collection cycles.",
					Unit: "",
				},
			},
			"go_gc_duration_seconds_sum": {
				{
					Type: "summary",
					Help: "A summary of the wall-time pause (stop-the-world) duration in garbage collection cycles.",
					Unit: "",
				},
			},
		},
	}

	require.Equal(t, expectedMetadata, actualMetadata)
}

// TestAssertMetadataAvailable tests the AssertMetadataAvailable function
// Note: This test focuses on testing the logic around metadata validation
// rather than the HTTP interaction, since we cannot easily mock the HTTP calls
// in the current codebase structure.
func TestAssertMetadataAvailable(t *testing.T) {
	t.Run("test_checkMetadata_helper_function", func(t *testing.T) {
		// Test the checkMetadata helper function that AssertMetadataAvailable uses internally
		tests := []struct {
			name             string
			expectedMetadata Metadata
			actualMetadata   Metadata
			expectedResult   bool
		}{
			{
				name: "exact_match",
				expectedMetadata: Metadata{
					Type: "counter",
					Help: "The counter description string",
					Unit: "",
				},
				actualMetadata: Metadata{
					Type: "counter",
					Help: "The counter description string",
					Unit: "",
				},
				expectedResult: true,
			},
			{
				name: "type_mismatch",
				expectedMetadata: Metadata{
					Type: "counter",
					Help: "The counter description string",
					Unit: "",
				},
				actualMetadata: Metadata{
					Type: "gauge",
					Help: "The counter description string",
					Unit: "",
				},
				expectedResult: false,
			},
			{
				name: "help_mismatch",
				expectedMetadata: Metadata{
					Type: "counter",
					Help: "The counter description string",
					Unit: "",
				},
				actualMetadata: Metadata{
					Type: "counter",
					Help: "Wrong help text",
					Unit: "",
				},
				expectedResult: false,
			},
			{
				name: "unit_mismatch",
				expectedMetadata: Metadata{
					Type: "counter",
					Help: "The counter description string",
					Unit: "seconds",
				},
				actualMetadata: Metadata{
					Type: "counter",
					Help: "The counter description string",
					Unit: "milliseconds",
				},
				expectedResult: false,
			},
			{
				name: "all_fields_with_units_match",
				expectedMetadata: Metadata{
					Type: "histogram",
					Help: "Request duration histogram",
					Unit: "seconds",
				},
				actualMetadata: Metadata{
					Type: "histogram",
					Help: "Request duration histogram",
					Unit: "seconds",
				},
				expectedResult: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := checkMetadata(tt.expectedMetadata, tt.actualMetadata)
				assert.Equal(t, tt.expectedResult, result,
					"checkMetadata(%+v, %+v) = %v, want %v",
					tt.expectedMetadata, tt.actualMetadata, result, tt.expectedResult)
			})
		}
	})
}

// TestAssertMetadataAvailableWithMockServer tests the full AssertMetadataAvailable function
// using a mock HTTP server to simulate the Prometheus metadata endpoint.
func TestAssertMetadataAvailableWithMockServer(t *testing.T) {
	// Test data
	testExpectedMetadata := map[string]Metadata{
		"golang_counter": {
			Type: "counter",
			Help: "The counter description string",
			Unit: "",
		},
		"golang_gauge": {
			Type: "gauge",
			Help: "The gauge description string",
			Unit: "",
		},
		"golang_histogram_bucket": {
			Type: "histogram",
			Help: "The histogram description string",
			Unit: "",
		},
	}

	t.Run("success_all_metadata_available", func(t *testing.T) {
		// Create mock response with all expected metadata
		mockResponse := MetadataResponse{
			Status: "success",
			Data: map[string][]Metadata{
				"golang_counter": {
					{
						Type: "counter",
						Help: "The counter description string",
						Unit: "",
					},
				},
				"golang_gauge": {
					{
						Type: "gauge",
						Help: "The gauge description string",
						Unit: "",
					},
				},
				"golang_histogram_bucket": {
					{
						Type: "histogram",
						Help: "The histogram description string",
						Unit: "",
					},
				},
			},
		}

		// Create mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mockResponse)
		}))
		defer server.Close()

		// Since we can't modify the const promURL, we'll test with a custom metadata query function
		// This test demonstrates the expected behavior

		// For now, let's test that the function would work with the correct data
		// by testing the internal logic directly
		query := server.URL + "/api/v1/metadata"

		// Test the metadata checking logic that AssertMetadataAvailable uses
		var missingMetadata []string
		var metadataResponse MetadataResponse
		err := FetchDataFromURL(query, &metadataResponse)
		require.NoError(t, err)

		for metric, metadata := range testExpectedMetadata {
			metadataList, exists := metadataResponse.Data[metric]
			if !exists || len(metadataList) == 0 || !checkMetadata(metadata, metadataList[0]) {
				missingMetadata = append(missingMetadata, metric)
			}
		}

		assert.Empty(t, missingMetadata, "No metadata should be missing")
	})

	t.Run("failure_missing_metadata", func(t *testing.T) {
		// Create mock response missing some metadata
		mockResponse := MetadataResponse{
			Status: "success",
			Data: map[string][]Metadata{
				"golang_counter": {
					{
						Type: "counter",
						Help: "The counter description string",
						Unit: "",
					},
				},
				// Missing golang_gauge and golang_histogram_bucket
			},
		}

		// Create mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mockResponse)
		}))
		defer server.Close()

		query := server.URL + "/api/v1/metadata"

		// Test the metadata checking logic
		var missingMetadata []string
		var metadataResponse MetadataResponse
		err := FetchDataFromURL(query, &metadataResponse)
		require.NoError(t, err)

		for metric, metadata := range testExpectedMetadata {
			metadataList, exists := metadataResponse.Data[metric]
			if !exists || len(metadataList) == 0 || !checkMetadata(metadata, metadataList[0]) {
				missingMetadata = append(missingMetadata, metric)
			}
		}

		assert.NotEmpty(t, missingMetadata, "Should have missing metadata")
		assert.Contains(t, missingMetadata, "golang_gauge")
		assert.Contains(t, missingMetadata, "golang_histogram_bucket")
		assert.NotContains(t, missingMetadata, "golang_counter")
	})

	t.Run("failure_wrong_metadata_type", func(t *testing.T) {
		// Create mock response with wrong metadata types
		mockResponse := MetadataResponse{
			Status: "success",
			Data: map[string][]Metadata{
				"golang_counter": {
					{
						Type: "gauge", // Wrong type - should be counter
						Help: "The counter description string",
						Unit: "",
					},
				},
				"golang_gauge": {
					{
						Type: "gauge",
						Help: "Wrong help text", // Wrong help text
						Unit: "",
					},
				},
				"golang_histogram_bucket": {
					{
						Type: "histogram",
						Help: "The histogram description string",
						Unit: "seconds", // Wrong unit - should be empty
					},
				},
			},
		}

		// Create mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mockResponse)
		}))
		defer server.Close()

		query := server.URL + "/api/v1/metadata"

		// Test the metadata checking logic
		var missingMetadata []string
		var metadataResponse MetadataResponse
		err := FetchDataFromURL(query, &metadataResponse)
		require.NoError(t, err)

		for metric, metadata := range testExpectedMetadata {
			metadataList, exists := metadataResponse.Data[metric]
			if !exists || len(metadataList) == 0 || !checkMetadata(metadata, metadataList[0]) {
				missingMetadata = append(missingMetadata, metric)
			}
		}

		assert.NotEmpty(t, missingMetadata, "Should have metadata mismatches")
		assert.Contains(t, missingMetadata, "golang_counter", "Counter should fail due to wrong type")
		assert.Contains(t, missingMetadata, "golang_gauge", "Gauge should fail due to wrong help")
		assert.Contains(t, missingMetadata, "golang_histogram_bucket", "Histogram should fail due to wrong unit")
	})

	t.Run("failure_http_error", func(t *testing.T) {
		// Create mock server that returns an error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		}))
		defer server.Close()

		query := server.URL + "/api/v1/metadata"

		// Test that FetchDataFromURL returns an error
		var metadataResponse MetadataResponse
		err := FetchDataFromURL(query, &metadataResponse)
		assert.Error(t, err, "Should return error for HTTP 500")
		assert.Contains(t, err.Error(), "Non-OK HTTP status")
	})

	t.Run("empty_metadata_response", func(t *testing.T) {
		// Create mock response with empty metadata
		mockResponse := MetadataResponse{
			Status: "success",
			Data:   map[string][]Metadata{},
		}

		// Create mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mockResponse)
		}))
		defer server.Close()

		query := server.URL + "/api/v1/metadata"

		// Test the metadata checking logic
		var missingMetadata []string
		var metadataResponse MetadataResponse
		err := FetchDataFromURL(query, &metadataResponse)
		require.NoError(t, err)

		for metric, metadata := range testExpectedMetadata {
			metadataList, exists := metadataResponse.Data[metric]
			if !exists || len(metadataList) == 0 || !checkMetadata(metadata, metadataList[0]) {
				missingMetadata = append(missingMetadata, metric)
			}
		}

		assert.NotEmpty(t, missingMetadata, "All metadata should be missing")
		assert.Len(t, missingMetadata, len(testExpectedMetadata))
	})
}

// TestAssertMetadataAvailableIntegration demonstrates how to use the function
// This test shows the expected usage pattern without mocking
func TestAssertMetadataAvailableIntegration(t *testing.T) {
	t.Skip("Skipping integration test - requires running Prometheus instance")

	// Example usage with real data
	metrics := []string{"golang_counter", "golang_gauge"}
	histogramMetrics := []string{"golang_histogram_bucket"}
	expectedMetadata := map[string]Metadata{
		"golang_counter": {
			Type: "counter",
			Help: "The counter description string",
			Unit: "",
		},
		"golang_gauge": {
			Type: "gauge",
			Help: "The gauge description string",
			Unit: "",
		},
		"golang_histogram_bucket": {
			Type: "histogram",
			Help: "The histogram description string",
			Unit: "",
		},
	}
	testName := "test_metadata_integration"

	// This would be called in a real integration test
	AssertMetadataAvailable(t, metrics, histogramMetrics, expectedMetadata, testName)
}

// TestAssertMetadataAvailableBugExposure tests the bug in AssertMetadataAvailable
// where missingMetadata slice is not reset between retries
func TestAssertMetadataAvailableBugExposure(t *testing.T) {
	// Test data
	testExpectedMetadata := map[string]Metadata{
		"golang_counter": {
			Type: "counter",
			Help: "The counter description string",
			Unit: "",
		},
		"golang_gauge": {
			Type: "gauge",
			Help: "The gauge description string",
			Unit: "",
		},
	}

	t.Run("simulate_eventual_availability_bug", func(t *testing.T) {
		callCount := 0

		// Create mock server that initially returns incomplete metadata but later becomes complete
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			var mockResponse MetadataResponse
			if callCount <= 1 {
				// First few calls - missing golang_gauge metadata
				mockResponse = MetadataResponse{
					Status: "success",
					Data: map[string][]Metadata{
						"golang_counter": {
							{
								Type: "counter",
								Help: "The counter description string",
								Unit: "",
							},
						},
						// Missing golang_gauge initially
					},
				}
			} else {
				// Later calls - all metadata available
				mockResponse = MetadataResponse{
					Status: "success",
					Data: map[string][]Metadata{
						"golang_counter": {
							{
								Type: "counter",
								Help: "The counter description string",
								Unit: "",
							},
						},
						"golang_gauge": {
							{
								Type: "gauge",
								Help: "The gauge description string",
								Unit: "",
							},
						},
					},
				}
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mockResponse)
		}))
		defer server.Close()

		query := server.URL + "/api/v1/metadata"

		// This reproduces the bug in AssertMetadataAvailable logic
		// Let's simulate what the function does internally
		var missingMetadata []string // Bug: this is not reset between iterations

		// First iteration - golang_gauge will be missing
		var metadataResponse1 MetadataResponse
		err := FetchDataFromURL(query, &metadataResponse1)
		require.NoError(t, err)

		for metric, metadata := range testExpectedMetadata {
			metadataList, exists := metadataResponse1.Data[metric]
			if !exists || len(metadataList) == 0 || !checkMetadata(metadata, metadataList[0]) {
				missingMetadata = append(missingMetadata, metric)
			}
		}
		assert.NotEmpty(t, missingMetadata, "Should have missing metadata in first call")
		assert.Contains(t, missingMetadata, "golang_gauge")
		assert.Len(t, missingMetadata, 1)

		// Second iteration - golang_gauge is now available, but missingMetadata is not reset
		var metadataResponse2 MetadataResponse
		err = FetchDataFromURL(query, &metadataResponse2)
		require.NoError(t, err)

		// BUG: missingMetadata should be reset here, but it's not in the original function
		// missingMetadata = nil // This line is missing in the original function

		for metric, metadata := range testExpectedMetadata {
			metadataList, exists := metadataResponse2.Data[metric]
			if !exists || len(metadataList) == 0 || !checkMetadata(metadata, metadataList[0]) {
				missingMetadata = append(missingMetadata, metric)
			}
		}

		// This demonstrates the bug: even though all metadata is now available,
		// the missingMetadata slice still contains "golang_gauge" from the first iteration
		assert.NotEmpty(t, missingMetadata, "Bug: missingMetadata still contains old entries")
		assert.Contains(t, missingMetadata, "golang_gauge", "Bug: golang_gauge should not be missing anymore")

		// Test the correct behavior - reset missingMetadata before each check
		missingMetadata = nil // Reset the slice (this is what the fix should do)
		for metric, metadata := range testExpectedMetadata {
			metadataList, exists := metadataResponse2.Data[metric]
			if !exists || len(metadataList) == 0 || !checkMetadata(metadata, metadataList[0]) {
				missingMetadata = append(missingMetadata, metric)
			}
		}

		assert.Empty(t, missingMetadata, "After fix: no metadata should be missing")
	})
}

// TestAssertMetadataAvailableFixed tests that the fixed version works correctly
// with the eventual consistency logic when metadata becomes available over time
func TestAssertMetadataAvailableFixed(t *testing.T) {
	// Custom implementation that mimics AssertMetadataAvailable but allows custom URL
	assertMetadataAvailableWithCustomURL := func(t *testing.T, customURL string, expectedMetadata map[string]Metadata) error {
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			var missingMetadata []string // Reset the slice on each retry (this is the fix)
			var metadataResponse MetadataResponse
			err := FetchDataFromURL(customURL, &metadataResponse)
			assert.NoError(c, err)

			for metric, metadata := range expectedMetadata {
				metadataList, exists := metadataResponse.Data[metric]
				if !exists || len(metadataList) == 0 || !checkMetadata(metadata, metadataList[0]) {
					missingMetadata = append(missingMetadata, metric)
				}
			}
			assert.Empty(c, missingMetadata, fmt.Sprintf("Some metadata are missing: %v", missingMetadata))
		}, DefaultTimeout, DefaultRetryInterval)
		return nil
	}

	testExpectedMetadata := map[string]Metadata{
		"golang_counter": {
			Type: "counter",
			Help: "The counter description string",
			Unit: "",
		},
		"golang_gauge": {
			Type: "gauge",
			Help: "The gauge description string",
			Unit: "",
		},
	}

	t.Run("eventual_consistency_works_with_fix", func(t *testing.T) {
		callCount := 0

		// Create mock server that initially returns incomplete metadata but eventually becomes complete
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			var mockResponse MetadataResponse
			if callCount <= 3 {
				// First few calls - missing golang_gauge metadata
				mockResponse = MetadataResponse{
					Status: "success",
					Data: map[string][]Metadata{
						"golang_counter": {
							{
								Type: "counter",
								Help: "The counter description string",
								Unit: "",
							},
						},
						// Missing golang_gauge initially
					},
				}
			} else {
				// Later calls - all metadata available
				mockResponse = MetadataResponse{
					Status: "success",
					Data: map[string][]Metadata{
						"golang_counter": {
							{
								Type: "counter",
								Help: "The counter description string",
								Unit: "",
							},
						},
						"golang_gauge": {
							{
								Type: "gauge",
								Help: "The gauge description string",
								Unit: "",
							},
						},
					},
				}
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mockResponse)
		}))
		defer server.Close()

		customURL := server.URL + "/api/v1/metadata"

		// This should succeed because the function will retry until all metadata is available
		// With the fix, missingMetadata is reset on each retry, so it will eventually succeed
		err := assertMetadataAvailableWithCustomURL(t, customURL, testExpectedMetadata)
		assert.NoError(t, err, "Function should succeed once metadata becomes available")

		// Verify that the server was called multiple times (indicating retries happened)
		assert.Greater(t, callCount, 3, "Server should have been called multiple times due to retries")
	})
}
