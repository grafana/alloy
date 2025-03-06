package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearchTargetsInSection(t *testing.T) {
	// Parse the JSON into a map
	var compInfo map[string]interface{}
	err := json.Unmarshal([]byte(testComponentJSON), &compInfo)
	require.NoError(t, err, "Failed to parse test JSON")

	tests := []struct {
		name        string
		query       string
		section     string
		wantMatches []map[string]string
		wantEmpty   bool
	}{
		{
			name:    "Match on job label in arguments section",
			query:   "prometheus",
			section: "arguments",
			wantMatches: []map[string]string{
				{
					"job":      "prometheus",
					"instance": "localhost:9090",
				},
			},
		},
		{
			name:    "Match on label value in exports",
			query:   "us-west-1",
			section: "exports",
			wantMatches: []map[string]string{
				{
					"job":        "mysql",
					"instance":   "db.example.com:3306",
					"env":        "production",
					"datacenter": "us-west-1",
				},
			},
		},
		{
			name:    "Match on export key",
			query:   "redis",
			section: "exports",
			wantMatches: []map[string]string{
				{
					"job": "redis",
				},
			},
		},
		{
			name:    "Match multiple instances with regex in arguments",
			query:   "local.*:9\\d+",
			section: "arguments",
			wantMatches: []map[string]string{
				{
					"job":      "prometheus",
					"instance": "localhost:9090",
				},
				{
					"job":      "node_exporter",
					"instance": "localhost:9100",
				},
			},
		},
		{
			name:      "No match in unknown section",
			query:     "nonexistent_value",
			section:   "wrong_section",
			wantEmpty: true,
		},
		{
			name:      "No match in arguments",
			query:     "nonexistent_value",
			section:   "arguments",
			wantEmpty: true,
		},
		{
			name:      "No match in exports",
			query:     "nonexistent_value",
			section:   "exports",
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := searchTargetsInSection(tt.query, compInfo, tt.section)

			if tt.wantEmpty {
				require.Empty(t, got, "searchTargetsInSection() should return empty slice")
				return
			}

			require.Len(t, got, len(tt.wantMatches), "searchTargetsInSection() returned wrong number of matches")

			// Check each match against expected content
			for i, expectedMatch := range tt.wantMatches {
				require.Equal(t, expectedMatch, got[i], "Match content differs from expected at index %d", i)
			}
		})
	}
}

// testComponentJSON contains a sample component JSON structure for testing
var testComponentJSON = `{
  "name": "discovery.relabel",
  "type": "block",
  "localID": "discovery.relabel.foo",
  "moduleID": "test",
  "label": "foo",
  "referencesTo": [],
  "referencedBy": [],
  "health": {
    "state": "healthy",
    "message": "started component",
    "updatedTime": "2025-03-04T16:40:02.213303Z"
  },
  "original": "",
  "arguments": [
    {
      "name": "targets",
      "type": "attr",
      "value": {
        "type": "array",
        "value": [
          {
            "type": "object",
            "value": [
              {
                "key": "job",
                "value": {
                  "type": "string",
                  "value": "prometheus"
                }
              },
              {
                "key": "instance",
                "value": {
                  "type": "string",
                  "value": "localhost:9090"
                }
              }
            ]
          },
          {
            "type": "object",
            "value": [
              {
                "key": "job",
                "value": {
                  "type": "string",
                  "value": "node_exporter"
                }
              },
              {
                "key": "instance",
                "value": {
                  "type": "string",
                  "value": "localhost:9100"
                }
              }
            ]
          }
        ]
      }
    }
  ],
  "exports": [
    {
      "name": "output",
      "type": "attr",
      "value": {
        "type": "array",
        "value": [
          {
            "type": "object",
            "value": [
              {
                "key": "job",
                "value": {
                  "type": "string",
                  "value": "mysql"
                }
              },
              {
                "key": "instance",
                "value": {
                  "type": "string",
                  "value": "db.example.com:3306"
                }
              },
              {
                "key": "env",
                "value": {
                  "type": "string",
                  "value": "production"
                }
              },
              {
                "key": "datacenter",
                "value": {
                  "type": "string",
                  "value": "us-west-1"
                }
              }
            ]
          },
	      {
            "type": "object",
            "value": [
              {
                "key": "job",
                "value": {
                  "type": "string",
                  "value": "redis"
                }
              }
            ]
          }
        ]
      }
    }
  ]
}`
