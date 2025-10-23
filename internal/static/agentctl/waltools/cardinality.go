package waltools

import (
	"github.com/prometheus/prometheus/tsdb/record"
	"github.com/prometheus/prometheus/tsdb/wlog"
)

// Cardinality represents some metric by name and the number of times that metric is used
// with a different combination of unique labels.
type Cardinality struct {
	Metric    string
	Instances int
}

// FindCardinality searches the WAL and returns the cardinality of all __name__
// series within the WAL for any given series with the label job=<job> and
// instance=<instance>. All other series are ignored.
func FindCardinality(walDir string, job string, instance string) ([]Cardinality, error) {
	w, err := wlog.Open(nil, walDir)
	if err != nil {
		return nil, err
	}
	defer w.Close()

	cardinality := map[string]int{}

	err = walIterate(w, func(r *wlog.Reader) error {
		return collectCardinality(r, job, instance, cardinality)
	})
	if err != nil {
		return nil, err
	}

	res := make([]Cardinality, 0, len(cardinality))
	for k, v := range cardinality {
		res = append(res, Cardinality{Metric: k, Instances: v})
	}
	return res, nil
}

func collectCardinality(r *wlog.Reader, job, instance string, cardinality map[string]int) error {
	var dec record.Decoder

	for r.Next() {
		rec := r.Record()

		switch dec.Type(rec) {
		case record.Series:
			series, err := dec.Series(rec, nil)
			if err != nil {
				return err
			}
			for _, s := range series {
				var (
					jobLabel      = s.Labels.Get("job")
					instanceLabel = s.Labels.Get("instance")
				)

				if jobLabel == job && instanceLabel == instance {
					cardinality[s.Labels.Get("__name__")]++
				}
			}
		}
	}

	return r.Err()
}
