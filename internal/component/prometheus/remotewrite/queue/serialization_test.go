package queue

import (
	"bytes"
	log2 "github.com/go-kit/log"
	"github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/compress/snappy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
	"time"
)

func TestSerialization(t *testing.T) {
	l := newParquetWrite(fakeQueue{}, 512*1024*1024, 30*time.Second, log2.NewNopLogger(), prometheus.NewRegistry())
	for i := 0; i < 1_000_000; i++ {
		m := make(map[string]string)
		for j := 0; j < 10; j++ {
			m["series_name_"+strconv.Itoa(j)] = "series_value" + strconv.Itoa(j)
		}
		lbls := labels.FromMap(m)
		ts := time.Now().Unix()
		err := l.AddMetric(lbls, nil, ts, 10, nil, nil, tSample)
		require.NoError(t, err)
	}
	bb, err := l.serialize()
	require.NoError(t, err)
	t.Log("size", len(bb))
}

func TestSerializationSimple(t *testing.T) {
	type TT struct {
		Name string `parquet:"name"`
		Age  int    `parquet:"age,delta"`
	}
	bb := &bytes.Buffer{}
	write := parquet.NewSortingWriter[*TT](bb, 100000, parquet.SortingWriterConfig(
		parquet.SortingColumns(
			parquet.Descending("name"),
		),
	), parquet.Compression(&snappy.Codec{}))
	//write := parquet.NewGenericWriter[*TT](bb, parquet.Compression(&snappy.Codec{}))
	arr := make([]*TT, 1_000_000)
	for i := 0; i < 1_000_000; i++ {
		arr[i] = &TT{
			Name: "value" + strconv.Itoa(i%5),
			Age:  i,
		}
	}
	write.Write(arr)
	write.Close()

	t.Log("Written schema:", write.Schema().String())
	t.Log("size", bb.Len())
}
