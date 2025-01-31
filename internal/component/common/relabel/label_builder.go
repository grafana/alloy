package relabel

type LabelBuilder interface {
	Get(label string) string
	// TODO(thampiotr): test that Set and Del can be called while iterating.
	Range(f func(label string, value string))
	Set(label string, val string)
	Del(ns ...string)
}
