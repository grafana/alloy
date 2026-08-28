package argument

const BlockName = "argument"

type Arguments struct {
	Optional bool   `alloy:"optional,attr,optional"`
	Default  any    `alloy:"default,attr,optional"`
	Comment  string `alloy:"comment,attr,optional"`
}
