package system

import rego.v1

import data.kubernetes.admission

main := {
	"apiVersion": "admission.k8s.io/v1",
	"kind": "AdmissionReview",
	"response": response,
}

default uid := ""

uid := input.request.uid

response := {
	"allowed": false,
	"uid": uid,
	"status": {"message": reason},
} if {
	reason = concat(", ", admission.deny)
	reason != ""
}

else := {"allowed": true, "uid": uid}
