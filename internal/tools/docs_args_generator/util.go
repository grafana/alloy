package main

const (
	requiredYes = "yes"
	requiredNo  = "no"
)

func printRequired(required bool) string {
	if required {
		return requiredYes
	}
	return requiredNo
}
