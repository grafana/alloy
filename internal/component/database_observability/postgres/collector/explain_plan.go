package collector

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/grafana/alloy/internal/component/database_observability"
)

type PgSQLExplainplan struct {
	Plan PlanNode `json:"Plan"`
}

type PlanNode struct {
	NodeType           string     `json:"Node Type"`
	Alias              string     `json:"Alias"`
	RelationName       string     `json:"Relation Name"`
	ParentRelationship string     `json:"Parent Relationship"`
	PartialMode        string     `json:"Partial Mode"`
	Strategy           string     `json:"Strategy"`
	ParallelAware      bool       `json:"Parallel Aware"`
	AsyncCapable       bool       `json:"Async Capable"`
	JoinType           string     `json:"Join Type"`
	InnerUnique        bool       `json:"Inner Unique"`
	HashCond           string     `json:"Hash Cond"`
	Filter             string     `json:"Filter"`
	StartupCost        float64    `json:"Startup Cost"`
	TotalCost          float64    `json:"Total Cost"`
	PlanRows           int64      `json:"Plan Rows"`
	PlanWidth          int64      `json:"Plan Width"`
	GroupKey           []string   `json:"Group Key"`
	SortKey            []string   `json:"Sort Key"`
	WorkersPlanned     int64      `json:"Workers Planned"`
	PlannedPartitions  int64      `json:"Planned Partitions"`
	Plans              []PlanNode `json:"Plans"`
	IndexName          string     `json:"Index Name"`
}

func newExplainPlanOutput(dbVersion string, queryId string, explainJson []byte, generatedAt string) (*database_observability.ExplainPlanOutput, error) {
	var planNodes []PgSQLExplainplan
	if err := json.Unmarshal(explainJson, &planNodes); err != nil {
		return nil, err
	}

	planNode, err := planNodes[0].Plan.ToExplainPlanOutputNode()
	if err != nil {
		return nil, err
	}

	output := &database_observability.ExplainPlanOutput{
		Metadata: database_observability.ExplainPlanMetadataInfo{
			DatabaseEngine:  "PostgreSQL",
			DatabaseVersion: dbVersion,
			QueryIdentifier: queryId,
			GeneratedAt:     generatedAt,
		},
		Plan: planNode,
	}

	return output, nil
}

func (p *PlanNode) ToExplainPlanOutputNode() (database_observability.ExplainPlanNode, error) {
	cost := p.totalCost()
	output := database_observability.ExplainPlanNode{
		Operation: p.explainPlanNodeOperation(),
		Details: database_observability.ExplainPlanNodeDetails{
			EstimatedRows: p.PlanRows,
			EstimatedCost: cost,
		},
	}

	if len(p.GroupKey) > 0 {
		output.Details.GroupByKeys = p.GroupKey
	}

	if len(p.SortKey) > 0 {
		output.Details.SortKeys = p.SortKey
	}

	if !strings.EqualFold(p.JoinType, "") {
		output.Details.JoinType = &p.JoinType
	}

	if !strings.EqualFold(p.Filter, "") {
		redacted := redact(p.Filter)
		output.Details.Condition = &redacted
	}

	if !strings.EqualFold(p.Alias, "") {
		output.Details.Alias = &p.Alias
	}

	if !strings.EqualFold(p.IndexName, "") {
		output.Details.KeyUsed = &p.IndexName
	}

	if strings.EqualFold(p.NodeType, "Hash Join") {
		algo := database_observability.ExplainPlanJoinAlgorithmHash
		output.Details.JoinAlgorithm = &algo
	}

	for _, child := range p.Plans {
		childNode, err := child.ToExplainPlanOutputNode()
		if err != nil {
			return output, err
		}
		output.Children = append(output.Children, childNode)
	}

	return output, nil
}

func (p *PlanNode) explainPlanNodeOperation() database_observability.ExplainPlanOutputOperation {
	stringbuilder := strings.Builder{}
	if !strings.EqualFold(p.PartialMode, "") {
		stringbuilder.WriteString(p.PartialMode)
		stringbuilder.WriteString(" ")
	}

	if !strings.EqualFold(p.Strategy, "") {
		switch p.Strategy {
		case "Sorted":
			stringbuilder.WriteString("Group ")
		case "Plain":
			break
		default:
			stringbuilder.WriteString(p.Strategy)
			stringbuilder.WriteString(" ")
		}
	}

	if p.ParallelAware {
		stringbuilder.WriteString("Parallel ")
	}

	stringbuilder.WriteString(p.NodeType)
	return database_observability.ExplainPlanOutputOperation(stringbuilder.String())
}

func (p *PlanNode) totalCost() *float64 {
	var result float64
	result = p.TotalCost
	for _, plan := range p.Plans {
		result -= plan.TotalCost
	}
	result = math.Round(result*100) / 100
	if result < 0 {
		result = 0
	}
	return &result
}
