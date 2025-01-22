package relabel

// LabelBuilder is an interface that can be used to change labels with relabel logic.
type LabelBuilder interface {
	Get(label string) string
	Range(f func(label string, value string))
	Set(label string, val string)
	Del(ns ...string)
}
