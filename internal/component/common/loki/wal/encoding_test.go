package wal

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/tsdb/record"
	"github.com/stretchr/testify/require"
)

func Test_Encoding_Series(t *testing.T) {
	record := &Record{
		entryIndexMap: make(map[uint64]int),
		UserID:        "123",
		Series: []record.RefSeries{
			{
				Ref: 456,
				Labels: labels.FromMap(map[string]string{
					"foo":  "bar",
					"bazz": "buzz",
				}),
			},
			{
				Ref: 789,
				Labels: labels.FromMap(map[string]string{
					"abc": "123",
					"def": "456",
				}),
			},
		},
	}

	buf := record.EncodeSeries(nil)

	decoded := recordPool.GetRecord()

	err := DecodeRecord(buf, decoded)
	require.Nil(t, err)

	// Since we use a pool, there can be subtle differentiations between nil slices and len(0) slices.
	// Both are valid, so check length.
	require.Equal(t, 0, len(decoded.RefEntries))
	decoded.RefEntries = nil
	require.Equal(t, record, decoded)
}

func Test_Encoding_Entries(t *testing.T) {
	for _, tc := range []struct {
		desc    string
		rec     *Record
		version RecordType
	}{
		{
			desc: "v1",
			rec: &Record{
				entryIndexMap: make(map[uint64]int),
				UserID:        "123",
				RefEntries: []RefEntries{
					{
						Ref: 456,
						Entries: []push.Entry{
							{
								Timestamp: time.Unix(1000, 0),
								Line:      "first",
								StructuredMetadata: push.LabelsAdapter{
									{Name: "traceID", Value: "123"},
									{Name: "userID", Value: "a"},
								},
							},
							{
								Timestamp: time.Unix(2000, 0),
								Line:      "second",
								StructuredMetadata: push.LabelsAdapter{
									{Name: "traceID", Value: "456"},
									{Name: "userID", Value: "b"},
								},
							},
						},
					},
					{
						Ref: 789,
						Entries: []push.Entry{
							{
								Timestamp: time.Unix(3000, 0),
								Line:      "third",
								StructuredMetadata: push.LabelsAdapter{
									{Name: "traceID", Value: "789"},
									{Name: "userID", Value: "c"},
								},
							},
							{
								Timestamp: time.Unix(4000, 0),
								Line:      "fourth",
								StructuredMetadata: push.LabelsAdapter{
									{Name: "traceID", Value: "123"},
									{Name: "userID", Value: "d"},
								},
							},
						},
					},
				},
			},
			version: WALRecordEntriesV1,
		},
		{
			desc: "v2",
			rec: &Record{
				entryIndexMap: make(map[uint64]int),
				UserID:        "123",
				RefEntries: []RefEntries{
					{
						Ref:     456,
						Counter: 1, // v2 uses counter for WAL replay
						Entries: []push.Entry{
							{
								Timestamp: time.Unix(1000, 0),
								Line:      "first",
								StructuredMetadata: push.LabelsAdapter{
									{Name: "traceID", Value: "123"},
									{Name: "userID", Value: "a"},
								},
							},
							{
								Timestamp: time.Unix(2000, 0),
								Line:      "second",
								StructuredMetadata: push.LabelsAdapter{
									{Name: "traceID", Value: "456"},
									{Name: "userID", Value: "b"},
								},
							},
						},
					},
					{
						Ref:     789,
						Counter: 2, // v2 uses counter for WAL replay
						Entries: []push.Entry{
							{
								Timestamp: time.Unix(3000, 0),
								Line:      "third",
								StructuredMetadata: push.LabelsAdapter{
									{Name: "traceID", Value: "789"},
									{Name: "userID", Value: "c"},
								},
							},
							{
								Timestamp: time.Unix(4000, 0),
								Line:      "fourth",
								StructuredMetadata: push.LabelsAdapter{
									{Name: "traceID", Value: "123"},
									{Name: "userID", Value: "d"},
								},
							},
						},
					},
				},
			},
			version: WALRecordEntriesV2,
		},
		{
			desc: "v3",
			rec: &Record{
				entryIndexMap: make(map[uint64]int),
				UserID:        "123",
				RefEntries: []RefEntries{
					{
						Ref:     456,
						Counter: 1, // v2 uses counter for WAL replay
						Entries: []push.Entry{
							{
								Timestamp: time.Unix(1000, 0),
								Line:      "first",
								StructuredMetadata: push.LabelsAdapter{
									{Name: "traceID", Value: "123"},
									{Name: "userID", Value: "a"},
								},
							},
							{
								Timestamp: time.Unix(2000, 0),
								Line:      "second",
								StructuredMetadata: push.LabelsAdapter{
									{Name: "traceID", Value: "456"},
									{Name: "userID", Value: "b"},
								},
							},
						},
					},
					{
						Ref:     789,
						Counter: 2, // v2 uses counter for WAL replay
						Entries: []push.Entry{
							{
								Timestamp: time.Unix(3000, 0),
								Line:      "third",
								StructuredMetadata: push.LabelsAdapter{
									{Name: "traceID", Value: "789"},
									{Name: "userID", Value: "c"},
								},
							},
							{
								Timestamp: time.Unix(4000, 0),
								Line:      "fourth",
								StructuredMetadata: push.LabelsAdapter{
									{Name: "traceID", Value: "123"},
									{Name: "userID", Value: "d"},
								},
							},
						},
					},
				},
			},
			version: WALRecordEntriesV3,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			decoded := recordPool.GetRecord()
			buf := tc.rec.EncodeEntries(tc.version, nil)
			err := DecodeRecord(buf, decoded)
			require.Nil(t, err)

			// If the version is less than v3, we need to remove the structured metadata.
			expectedRecords := tc.rec
			if tc.version < WALRecordEntriesV3 {
				for i := range expectedRecords.RefEntries {
					for j := range expectedRecords.RefEntries[i].Entries {
						expectedRecords.RefEntries[i].Entries[j].StructuredMetadata = nil
					}
				}
			}

			require.Equal(t, expectedRecords, decoded)
		})
	}
}

func Benchmark_EncodeEntries(b *testing.B) {
	for _, withStructuredMetadata := range []bool{true, false} {
		b.Run(fmt.Sprintf("structuredMetadata=%t", withStructuredMetadata), func(b *testing.B) {
			var entries []push.Entry
			for i := int64(0); i < 10000; i++ {
				entry := push.Entry{
					Timestamp: time.Unix(0, i),
					Line:      fmt.Sprintf("long line with a lot of data like a log %d", i),
				}

				if withStructuredMetadata {
					entry.StructuredMetadata = push.LabelsAdapter{
						{Name: "traceID", Value: strings.Repeat(fmt.Sprintf("%d", i), 10)},
						{Name: "userID", Value: strings.Repeat(fmt.Sprintf("%d", i), 10)},
					}
				}

				entries = append(entries, entry)
			}
			record := &Record{
				entryIndexMap: make(map[uint64]int),
				UserID:        "123",
				RefEntries: []RefEntries{
					{
						Ref:     456,
						Entries: entries,
					},
					{
						Ref:     789,
						Entries: entries,
					},
				},
			}
			b.ReportAllocs()
			b.ResetTimer()
			buf := recordPool.GetBytes()
			defer recordPool.PutBytes(buf)

			for n := 0; n < b.N; n++ {
				*buf = record.EncodeEntries(CurrentEntriesRec, *buf)
			}
		})
	}
}

func Benchmark_DecodeWAL(b *testing.B) {
	for _, withStructuredMetadata := range []bool{true, false} {
		b.Run(fmt.Sprintf("structuredMetadata=%t", withStructuredMetadata), func(b *testing.B) {
			var entries []push.Entry
			for i := int64(0); i < 10000; i++ {
				entry := push.Entry{
					Timestamp: time.Unix(0, i),
					Line:      fmt.Sprintf("long line with a lot of data like a log %d", i),
				}

				if withStructuredMetadata {
					entry.StructuredMetadata = push.LabelsAdapter{
						{Name: "traceID", Value: strings.Repeat(fmt.Sprintf("%d", i), 10)},
						{Name: "userID", Value: strings.Repeat(fmt.Sprintf("%d", i), 10)},
					}
				}

				entries = append(entries, entry)
			}
			record := &Record{
				entryIndexMap: make(map[uint64]int),
				UserID:        "123",
				RefEntries: []RefEntries{
					{
						Ref:     456,
						Entries: entries,
					},
					{
						Ref:     789,
						Entries: entries,
					},
				},
			}

			buf := record.EncodeEntries(CurrentEntriesRec, nil)
			rec := recordPool.GetRecord()
			b.ReportAllocs()
			b.ResetTimer()

			for n := 0; n < b.N; n++ {
				require.NoError(b, DecodeRecord(buf, rec))
			}
		})
	}
}
