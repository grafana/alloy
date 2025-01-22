package relabel

// TODO(thampiotr): write comment
// TODO(thampiotr): port the test

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/prometheus/common/model"
)

type LabelBuilder interface {
	Get(label string) string
	// TODO(thampiotr): test that Set and Del can be called while iterating.
	Range(f func(label string, value string))
	Set(label string, val string)
	Del(ns ...string)
}

// ProcessBuilder is like Process, but the caller passes a labels.Builder
// containing the initial set of labels, which is mutated by the rules.
func ProcessBuilder(lb LabelBuilder, cfgs ...*Config) (keep bool) {
	for _, cfg := range cfgs {
		keep = doRelabel(cfg, lb)
		if !keep {
			return false
		}
	}
	return true
}

func doRelabel(cfg *Config, lb LabelBuilder) (keep bool) {
	var va [16]string
	values := va[:0]
	if len(cfg.SourceLabels) > cap(values) {
		values = make([]string, 0, len(cfg.SourceLabels))
	}
	for _, ln := range cfg.SourceLabels {
		values = append(values, lb.Get(string(ln)))
	}
	val := strings.Join(values, cfg.Separator)

	switch cfg.Action {
	case Drop:
		if cfg.Regex.MatchString(val) {
			return false
		}
	case Keep:
		if !cfg.Regex.MatchString(val) {
			return false
		}
	case DropEqual:
		if lb.Get(cfg.TargetLabel) == val {
			return false
		}
	case KeepEqual:
		if lb.Get(cfg.TargetLabel) != val {
			return false
		}
	case Replace:
		indexes := cfg.Regex.FindStringSubmatchIndex(val)
		// If there is no match no replacement must take place.
		if indexes == nil {
			break
		}
		target := model.LabelName(cfg.Regex.ExpandString([]byte{}, cfg.TargetLabel, val, indexes))
		if !target.IsValid() {
			break
		}
		res := cfg.Regex.ExpandString([]byte{}, cfg.Replacement, val, indexes)
		if len(res) == 0 {
			lb.Del(string(target))
			break
		}
		lb.Set(string(target), string(res))
	case Lowercase:
		lb.Set(cfg.TargetLabel, strings.ToLower(val))
	case Uppercase:
		lb.Set(cfg.TargetLabel, strings.ToUpper(val))
	case HashMod:
		hash := md5.Sum([]byte(val))
		// Use only the last 8 bytes of the hash to give the same result as earlier versions of this code.
		mod := binary.BigEndian.Uint64(hash[8:]) % cfg.Modulus
		lb.Set(cfg.TargetLabel, fmt.Sprintf("%d", mod))
	case LabelMap:
		lb.Range(func(name, value string) {
			if cfg.Regex.MatchString(name) {
				res := cfg.Regex.ReplaceAllString(name, cfg.Replacement)
				lb.Set(res, value)
			}
		})
	case LabelDrop:
		lb.Range(func(name, value string) {
			if cfg.Regex.MatchString(name) {
				lb.Del(name)
			}
		})
	case LabelKeep:
		lb.Range(func(name, value string) {
			if !cfg.Regex.MatchString(name) {
				lb.Del(value)
			}
		})
	default:
		panic(fmt.Errorf("relabel: unknown relabel action type %q", cfg.Action))
	}

	return true
}
