//nolint:revive // suppress "var-naming: avoid meaningless package names"
package util

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	hexToDecimalBase        = 16
	hexToDecimalUIntBitSize = 64
)

var (
	matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllCap   = regexp.MustCompile("([a-z0-9])([A-Z])")
)

func ToSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")

	return strings.ToLower(snake)
}

func HexToDecimal(hex string) (float64, error) {
	s := hex
	s = strings.ReplaceAll(s, "0x", "")
	s = strings.ReplaceAll(s, "0X", "")
	parsed, err := strconv.ParseUint(s, hexToDecimalBase, hexToDecimalUIntBitSize)

	return float64(parsed), err
}
