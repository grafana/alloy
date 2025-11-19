package database_observability

import "github.com/grafana/alloy/internal/component/common/relabel"

func GetRelabelingRules(serverID string) []*relabel.Config {
	r := relabel.DefaultRelabelConfig // use default to avoid defining all fields
	r.Replacement = serverID
	r.TargetLabel = "server_id"
	r.Action = relabel.Replace
	return []*relabel.Config{&r}
}
