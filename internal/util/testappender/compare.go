package testappender

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

// NOTE(rfratto): this file is only needed because client_golang's testutil
// package currently enforces the Prometheus text exposition format, due to
// there not being a OpenMetrics parser in prometheus/common yet.
//
// This means that, unlike with testutil, callers are forced to compare line
// order of comments and metrics. This can be a good thing, in some cases, but
// is fairly tedious if you don't care about the order.

// Comparer can compare a slice of dto.MetricFamily to an expected list of
// metrics.
type Comparer struct {
	// OpenMetrics indicates that the Comparer should test the OpenMetrics
	// representation instead of the Prometheus text exposition format.
	OpenMetrics bool
}

// Compare compares the text representation of families to an expected input
// string. If the OpenMetrics field of the Comparer is true, families is
// converted into the OpenMetrics text exposition format. Otherwise, families
// is converted into the Prometheus text exposition format.
//
// To make testing less error-prone, expect is cleaned by removing leading
// whitespace, trailing whitespace, and empty lines. The cleaned version of
// expect is then compared directly against the text representation of
// families.
func (c Comparer) Compare(families []*dto.MetricFamily, expect string) error {
	expect = cleanExpositionString(expect)

	var (
		enc expfmt.Encoder
		buf bytes.Buffer
	)
	if c.OpenMetrics {
		enc = expfmt.NewEncoder(&buf, expfmt.FmtOpenMetrics_1_0_0)
	} else {
		enc = expfmt.NewEncoder(&buf, expfmt.FmtText)
	}
	for _, f := range families {
		if err := enc.Encode(f); err != nil {
			return fmt.Errorf("error encoding family %s: %w", f.GetName(), err)
		}
	}

	if expect != buf.String() {
		diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(expect),
			B:        difflib.SplitLines(buf.String()),
			FromFile: "Expected",
			ToFile:   "Actual",
			Context:  1,
		})
		return fmt.Errorf("metric data does not match:\n\n%s", diff)
	}

	return nil
}

func cleanExpositionString(s string) string {
	scanner := bufio.NewScanner(strings.NewReader(s))

	var res strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		fmt.Fprint(&res, line, "\n")
	}

	return res.String()
}

// Compare compares the text representation of families to an expected input
// string. Families is converted into the Prometheus text exposition format.
//
// To make testing less error-prone, expect is cleaned by removing leading
// whitespace, trailing whitespace, and empty lines. The cleaned version of
// expect is then compared directly against the text representation of
// families.
func Compare(families []*dto.MetricFamily, expect string) error {
	var c Comparer
	return c.Compare(families, expect)
}
