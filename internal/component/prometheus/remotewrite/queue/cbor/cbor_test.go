package cbor

/*
func TestCBORSample(t *testing.T) {
	l := newCBORWrite(fakeQueue{}, 16*1024*1024, 30*time.Second, log2.NewNopLogger(), prometheus.NewRegistry())
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	ts := time.Now().Unix()
	err := l.AddMetric(lbls, nil, ts, 10, nil, nil, types.Sample)
	require.NoError(t, err)
	bb, err := l.serialize()
	require.NoError(t, err)

	metrics, err := Deserialize(bb, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	require.Len(t, metrics[0].SeriesLabels, 1)

	require.True(t, hasLabel(lbls, metrics, ts, 10))
}

func TestCBORSampleMultiple(t *testing.T) {
	l := newCBORWrite(fakeQueue{}, 16*1024*1024, 30*time.Second, log2.NewNopLogger(), prometheus.NewRegistry())
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	ts := time.Now().Unix()
	err := l.AddMetric(lbls, nil, ts, 10, nil, nil, types.Sample)
	require.NoError(t, err)

	lbls2 := labels.FromMap(map[string]string{
		"__name__": "test",
		"lbl":      "label_1",
	})

	err = l.AddMetric(lbls2, nil, ts, 11, nil, nil, types.Sample)
	require.NoError(t, err)

	bb, err := l.serialize()
	require.NoError(t, err)
	metrics, err := Deserialize(bb, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 2)

	require.True(t, hasLabel(lbls, metrics, ts, 10))
	require.True(t, hasLabel(lbls2, metrics, ts, 11))
}

func TestCBORSampleMultipleDifferent(t *testing.T) {
	l := newCBORWrite(fakeQueue{}, 16*1024*1024, 30*time.Second, log2.NewNopLogger(), prometheus.NewRegistry())
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
		"badlabel": "arrr",
	})
	ts := time.Now().Unix()
	err := l.AddMetric(lbls, nil, ts, 10, nil, nil, types.Sample)
	require.NoError(t, err)

	lbls2 := labels.FromMap(map[string]string{
		"__name__": "test1",
		"lbl":      "label_1",
		"bob":      "foo",
	})

	err = l.AddMetric(lbls2, nil, ts, 11, nil, nil, types.Sample)
	require.NoError(t, err)

	bb, err := l.serialize()
	require.NoError(t, err)
	metrics, err := Deserialize(bb, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 2)

	require.True(t, hasLabel(lbls, metrics, ts, 10))
	require.True(t, hasLabel(lbls2, metrics, ts, 11))
}

func TestCBORSampleTTL(t *testing.T) {
	l := newCBORWrite(fakeQueue{}, 16*1024*1024, 30*time.Second, log2.NewNopLogger(), prometheus.NewRegistry())

	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	ts := time.Now().Unix()
	err := l.AddMetric(lbls, nil, ts, 10, nil, nil, types.Sample)
	require.NoError(t, err)

	bb, err := l.serialize()
	require.NoError(t, err)
	time.Sleep(2 * time.Second)
	metrics, err := Deserialize(bb, 1)
	require.NoError(t, err)
	require.Len(t, metrics, 0)
}

func TestCBORExemplar(t *testing.T) {
	l := newCBORWrite(fakeQueue{}, 16*1024*1024, 30*time.Second, log2.NewNopLogger(), prometheus.NewRegistry())
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	ExemplarLabels := labels.FromMap(map[string]string{
		"ex": "one",
	})
	ts := time.Now().Unix()
	err := l.AddMetric(lbls, ExemplarLabels, ts, 10, nil, nil, types.Exemplar)
	require.NoError(t, err)

	bb, err := l.serialize()
	require.NoError(t, err)
	metrics, err := Deserialize(bb, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	require.True(t, metrics[0].Type == types.Exemplar)
	require.Len(t, metrics[0].SeriesLabels, 1)
	require.Len(t, metrics[0].ExemplarLabels, 1)

	require.True(t, metrics[0].SeriesLabels[0].Name == "__name__")
	require.True(t, metrics[0].SeriesLabels[0].Value == "test")

	require.True(t, metrics[0].ExemplarLabels[0].Name == "ex")
	require.True(t, metrics[0].ExemplarLabels[0].Value == "one")
}

func TestCBORMultipleExemplar(t *testing.T) {
	l := newCBORWrite(fakeQueue{}, 16*1024*1024, 30*time.Second, log2.NewNopLogger(), prometheus.NewRegistry())
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	ExemplarLabels := labels.FromMap(map[string]string{
		"ex": "one",
	})

	ts := time.Now().Unix()
	err := l.AddMetric(lbls, ExemplarLabels, ts, 10, nil, nil, types.Exemplar)
	require.NoError(t, err)

	lbls2 := labels.FromMap(map[string]string{
		"__name__": "test",
		"bob":      "arr",
	})
	ExemplarLabels2 := labels.FromMap(map[string]string{
		"ex":  "one",
		"ex2": "two",
	})
	l.AddMetric(lbls2, ExemplarLabels2, ts, 11, nil, nil, types.Exemplar)

	bb, err := l.serialize()
	require.NoError(t, err)
	metrics, err := Deserialize(bb, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 2)
	require.True(t, metrics[0].Type == types.Exemplar)
	require.Len(t, metrics[0].SeriesLabels, 1)
	require.Len(t, metrics[0].ExemplarLabels, 1)

	require.True(t, hasLabel(lbls, metrics, ts, 10))
	require.True(t, hasLabelsExemplar(ExemplarLabels, metrics, ts, 10))

	require.True(t, hasLabel(lbls2, metrics, ts, 11))
	require.True(t, hasLabelsExemplar(ExemplarLabels2, metrics, ts, 11))
}

func TestCBORExemplarNoTS(t *testing.T) {
	l := newCBORWrite(fakeQueue{}, 16*1024*1024, 30*time.Second, log2.NewNopLogger(), prometheus.NewRegistry())
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	ExemplarLabels := labels.FromMap(map[string]string{
		"ex": "one",
	})
	err := l.AddMetric(lbls, ExemplarLabels, 0, 10, nil, nil, types.Exemplar)
	require.NoError(t, err)

	bb, err := l.serialize()
	require.NoError(t, err)
	metrics, err := Deserialize(bb, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	require.True(t, metrics[0].Timestamp == 0)
	require.True(t, metrics[0].Type == types.Exemplar)
	require.Len(t, metrics[0].SeriesLabels, 1)
	require.Len(t, metrics[0].ExemplarLabels, 1)

	require.True(t, metrics[0].SeriesLabels[0].Name == "__name__")
	require.True(t, metrics[0].SeriesLabels[0].Value == "test")

	require.True(t, metrics[0].ExemplarLabels[0].Name == "ex")
	require.True(t, metrics[0].ExemplarLabels[0].Value == "one")
}

func TestCBORExemplarAndMetric(t *testing.T) {
	l := newCBORWrite(fakeQueue{}, 16*1024*1024, 30*time.Second, log2.NewNopLogger(), prometheus.NewRegistry())

	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	ExemplarLabels := labels.FromMap(map[string]string{
		"ex": "one",
	})
	err := l.AddMetric(lbls, ExemplarLabels, 0, 10, nil, nil, types.Exemplar)
	require.NoError(t, err)

	bb, err := l.serialize()
	require.NoError(t, err)

	metrics, err := Deserialize(bb, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	require.True(t, metrics[0].Timestamp == 0)
	require.True(t, metrics[0].Type == types.Exemplar)
	require.Len(t, metrics[0].SeriesLabels, 1)
	require.Len(t, metrics[0].ExemplarLabels, 1)

	require.True(t, metrics[0].SeriesLabels[0].Name == "__name__")
	require.True(t, metrics[0].SeriesLabels[0].Value == "test")

	require.True(t, metrics[0].ExemplarLabels[0].Name == "ex")
	require.True(t, metrics[0].ExemplarLabels[0].Value == "one")
}

func TestCBORHistogram(t *testing.T) {
	l := newCBORWrite(fakeQueue{}, 16*1024*1024, 30*time.Second, log2.NewNopLogger(), prometheus.NewRegistry())
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})

	// Note this histogram may not make logical sense, but the important thing is we get the same data in that we passed in.
	h := &histogram.Histogram{
		CounterResetHint: histogram.NotCounterReset,
		Schema:           2,
		ZeroThreshold:    1,
		ZeroCount:        1,
		Count:            10,
		Sum:              20,
		PositiveSpans: []histogram.Span{
			{
				Offset: 1,
				Length: 2,
			},
		},
		NegativeSpans: []histogram.Span{
			{
				Offset: 3,
				Length: 4,
			},
			{
				Offset: 5,
				Length: 6,
			},
		},
		PositiveBuckets: []int64{
			1,
			2,
			3,
		},
		NegativeBuckets: []int64{
			4,
			5,
			6,
		},
	}
	ts := time.Now().Unix()
	err := l.AddMetric(lbls, nil, ts, 0, h, nil, types.Histogram)
	require.NoError(t, err)

	bb, err := l.serialize()
	require.NoError(t, err)

	metrics, err := Deserialize(bb, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	m := metrics[0]
	require.True(t, m.Type == types.Histogram)
	require.Len(t, m.SeriesLabels, 1)
	require.True(t, h.Equals(m.Histogram))
}

func TestCBORypesFloatHistogram(t *testing.T) {
	l := newCBORWrite(fakeQueue{}, 16*1024*1024, 30*time.Second, log2.NewNopLogger(), prometheus.NewRegistry())
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})

	// Note this histogram may not make logical sense, but the important thing is we get the same data in that we passed in.
	h := &histogram.FloatHistogram{
		CounterResetHint: histogram.NotCounterReset,
		Schema:           2,
		ZeroThreshold:    1.1,
		ZeroCount:        1.2,
		Count:            10.6,
		Sum:              20.5,
		PositiveSpans: []histogram.Span{
			{
				Offset: 1,
				Length: 2,
			},
		},
		NegativeSpans: []histogram.Span{
			{
				Offset: 3,
				Length: 4,
			},
			{
				Offset: 5,
				Length: 6,
			},
		},
		PositiveBuckets: []float64{
			1.1,
			2.2,
			3.3,
		},
		NegativeBuckets: []float64{
			4.4,
			5.5,
			6.6,
		},
	}
	ts := time.Now().Unix()
	err := l.AddMetric(lbls, nil, ts, 0, nil, h, types.FloatHistogram)
	require.NoError(t, err)

	bb, err := l.serialize()
	require.NoError(t, err)
	metrics, err := Deserialize(bb, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	m := metrics[0]
	require.True(t, m.Type == types.FloatHistogram)
	require.Len(t, m.SeriesLabels, 1)
	require.True(t, h.Equals(m.FloatHistogram))
}

func hasLabel(lbls labels.Labels, metrics []*types.TimeSeries, ts int64, val float64) bool {
	for _, m := range metrics {
		if labels.Compare(m.SeriesLabels, lbls) == 0 {
			return ts == m.Timestamp && val == m.Value
		}
	}
	return false
}

func hasLabelsExemplar(lbls labels.Labels, metrics []*types.TimeSeries, ts int64, val float64) bool {
	for _, m := range metrics {
		if labels.Compare(m.ExemplarLabels, lbls) == 0 {
			return ts == m.Timestamp && val == m.Value
		}
	}
	return false
}

type fakeQueue struct{}

func (f fakeQueue) Add(data []byte) (string, error) {
	// TODO implement me
	panic("implement me")
}

func (f fakeQueue) Next(enc []byte) ([]byte, string, bool, bool) {
	// TODO implement me
	panic("implement me")
}

func (f fakeQueue) Name() string {
	return "test"
}
*/
