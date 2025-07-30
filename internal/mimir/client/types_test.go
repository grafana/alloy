package client

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/prometheus/model/rulefmt"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRuleGroup_Marshal(t *testing.T) {
	type fields struct {
		RuleGroup     rulefmt.RuleGroup
		SourceTenants []string
	}

	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "serialises a rule group",
			fields: fields{
				SourceTenants: []string{"tenant1", "tenant2"},
				RuleGroup: rulefmt.RuleGroup{
					Name:     "group",
					Interval: 0,
					Limit:    0,
					Rules: []rulefmt.Rule{
						{
							Alert: "alert",
							Expr:  "expr",
						},
					},
				},
			},
			want: `name: group
rules:
    - alert: alert
      expr: expr
source_tenants:
    - tenant1
    - tenant2
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rg := MimirRuleGroup{
				RuleGroup:     tt.fields.RuleGroup,
				SourceTenants: tt.fields.SourceTenants,
			}

			got, err := yaml.Marshal(rg)
			require.NoError(t, err)

			if !cmp.Equal(string(got), tt.want) {
				t.Errorf("yaml.Marshal() = %v", cmp.Diff(string(got), tt.want))
			}
		})
	}
}
