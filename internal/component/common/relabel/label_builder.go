package relabel

// LabelBuilder is an interface that can be used to change labels with relabel logic.
type LabelBuilder interface {
	// Get returns given label value. If label is not present, an empty string is returned.
	Get(label string) string
	Range(f func(label string, value string))
	// Set will set given label to given value. Setting to empty value is equivalent to deleting this label.
	Set(label string, val string)
	Del(ns ...string)
}
