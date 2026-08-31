package collector

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/grafana/alloy/internal/component/database_observability"
)

// This file parses SQL Server's "showplan" XML (the format captured by Query
// Store in sys.query_store_plan.query_plan) into the shared
// database_observability.ExplainPlanNode model. Unlike mysql/postgres's JSON
// plan formats, showplan nests each operator's children inside a wrapper
// element named after its physical operator category (e.g. <NestedLoops>,
// <Hash>, <IndexScan>) rather than under a uniform key, so the RelOp struct
// below declares one optional field per wrapper category we understand.
// Anything not modeled here falls back to ExplainPlanOutputOperationUnknown
// rather than being silently misclassified.

type xmlShowPlan struct {
	XMLName       xml.Name    `xml:"ShowPlanXML"`
	BatchSequence xmlBatchSeq `xml:"BatchSequence"`
}

type xmlBatchSeq struct {
	Batch []xmlBatch `xml:"Batch"`
}

type xmlBatch struct {
	Statements xmlStatements `xml:"Statements"`
}

type xmlStatements struct {
	StmtSimple []xmlStmtSimple `xml:"StmtSimple"`
}

type xmlStmtSimple struct {
	QueryPlan xmlQueryPlan `xml:"QueryPlan"`
}

type xmlQueryPlan struct {
	RelOp *xmlRelOp `xml:"RelOp"`
}

type xmlRelOp struct {
	PhysicalOp                string `xml:"PhysicalOp,attr"`
	LogicalOp                 string `xml:"LogicalOp,attr"`
	EstimateRows              string `xml:"EstimateRows,attr"`
	EstimatedTotalSubtreeCost string `xml:"EstimatedTotalSubtreeCost,attr"`

	Warnings *xmlWarnings `xml:"Warnings"`

	// One of the following is populated, depending on PhysicalOp. Each wrapper
	// element is named for its operator category, not for the specific
	// PhysicalOp string, so IndexScan covers Table Scan/Index Scan/Clustered
	// Index Scan/Index Seek/Clustered Index Seek/RID Lookup/Key Lookup alike.
	IndexScan       *xmlIndexScan  `xml:"IndexScan"`
	TableScan       *xmlIndexScan  `xml:"TableScan"`
	NestedLoops     *xmlOpChildren `xml:"NestedLoops"`
	Hash            *xmlHash       `xml:"Hash"`
	Merge           *xmlOpChildren `xml:"Merge"`
	StreamAggregate *xmlOpChildren `xml:"StreamAggregate"`
	Sort            *xmlSort       `xml:"Sort"`
	ComputeScalar   *xmlOpChildren `xml:"ComputeScalar"`
	Filter          *xmlFilter     `xml:"Filter"`
	Top             *xmlOpChildren `xml:"Top"`
	Concat          *xmlOpChildren `xml:"Concat"`
	Parallelism     *xmlOpChildren `xml:"Parallelism"`
	TableSpool      *xmlOpChildren `xml:"TableSpool"`
	IndexSpool      *xmlOpChildren `xml:"IndexSpool"`
}

// xmlOpChildren is reused by wrapper elements whose only content we care
// about is their nested RelOp children (join build/probe sides, the single
// child of a unary operator like Sort/Filter/Top, etc.).
type xmlOpChildren struct {
	RelOp []xmlRelOp `xml:"RelOp"`
}

type xmlIndexScan struct {
	Lookup    string        `xml:"Lookup,attr"`
	Object    xmlObject     `xml:"Object"`
	Predicate *xmlPredicate `xml:"Predicate"`
}

type xmlObject struct {
	Table string `xml:"Table,attr"`
	Index string `xml:"Index,attr"`
}

type xmlHash struct {
	xmlOpChildren
	GroupBy *xmlGroupBy `xml:"GroupBy"`
}

type xmlSort struct {
	xmlOpChildren
	OrderBy *xmlOrderBy `xml:"OrderBy"`
}

type xmlFilter struct {
	xmlOpChildren
	Predicate *xmlPredicate `xml:"Predicate"`
}

type xmlPredicate struct {
	ScalarOperator xmlScalarOperator `xml:"ScalarOperator"`
}

type xmlScalarOperator struct {
	ScalarString string `xml:"ScalarString,attr"`
}

type xmlGroupBy struct {
	ColumnReference []xmlColumnReference `xml:"ColumnReference"`
}

type xmlOrderBy struct {
	OrderByColumn []xmlOrderByColumn `xml:"OrderByColumn"`
}

type xmlOrderByColumn struct {
	ColumnReference xmlColumnReference `xml:"ColumnReference"`
}

type xmlColumnReference struct {
	Column string `xml:"Column,attr"`
}

type xmlWarnings struct {
	NoJoinPredicate         *struct{} `xml:"NoJoinPredicate"`
	ColumnsWithNoStatistics *struct{} `xml:"ColumnsWithNoStatistics"`
	SpillToTempDb           *struct{} `xml:"SpillToTempDb"`
}

// newExplainPlanOutputFromShowPlanXML parses a single StmtSimple's showplan
// XML (as captured by sys.query_store_plan.query_plan) into the shared
// ExplainPlanNode model. It does not execute or re-plan anything - this is
// the plan SQL Server already compiled and cached in Query Store.
func newExplainPlanOutputFromShowPlanXML(showPlanXML []byte) (*database_observability.ExplainPlanNode, error) {
	var plan xmlShowPlan
	if err := xml.Unmarshal(showPlanXML, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse showplan xml: %w", err)
	}

	if len(plan.BatchSequence.Batch) == 0 || len(plan.BatchSequence.Batch[0].Statements.StmtSimple) == 0 {
		return nil, fmt.Errorf("showplan xml has no statements")
	}

	rootRelOp := plan.BatchSequence.Batch[0].Statements.StmtSimple[0].QueryPlan.RelOp
	if rootRelOp == nil {
		return nil, fmt.Errorf("showplan xml has no root RelOp")
	}

	node := relOpToExplainPlanNode(*rootRelOp)
	return &node, nil
}

func relOpToExplainPlanNode(op xmlRelOp) database_observability.ExplainPlanNode {
	node := database_observability.ExplainPlanNode{
		Details: database_observability.ExplainPlanNodeDetails{
			EstimatedRows: parseInt64OrZero(op.EstimateRows),
			EstimatedCost: parseFloatPtrOrNil(op.EstimatedTotalSubtreeCost),
		},
	}

	if op.Warnings != nil {
		node.Details.Warnings = op.Warnings.strings()
	}

	switch {
	case op.IndexScan != nil:
		node.Operation = database_observability.ExplainPlanOutputOperationIndexScan
		populateScanDetails(&node, *op.IndexScan)
	case op.TableScan != nil:
		node.Operation = database_observability.ExplainPlanOutputOperationTableScan
		populateScanDetails(&node, *op.TableScan)
	case op.NestedLoops != nil:
		node.Operation = database_observability.ExplainPlanOutputOperationNestedLoopJoin
		algo := database_observability.ExplainPlanJoinAlgorithmNestedLoop
		node.Details.JoinAlgorithm = &algo
		if op.LogicalOp != "" {
			node.Details.JoinType = &op.LogicalOp
		}
		node.Children = childrenOf(op.NestedLoops.RelOp)
	case op.Hash != nil:
		populateHashDetails(&node, op)
		node.Children = childrenOf(op.Hash.RelOp)
	case op.Merge != nil:
		node.Operation = database_observability.ExplainPlanOutputOperationMergeJoin
		algo := database_observability.ExplainPlanJoinAlgorithmMerge
		node.Details.JoinAlgorithm = &algo
		if op.LogicalOp != "" {
			node.Details.JoinType = &op.LogicalOp
		}
		node.Children = childrenOf(op.Merge.RelOp)
	case op.StreamAggregate != nil:
		node.Operation = database_observability.ExplainPlanOutputOperationGroupingOperation
		node.Children = childrenOf(op.StreamAggregate.RelOp)
	case op.Sort != nil:
		if strings.EqualFold(op.LogicalOp, "Distinct Sort") {
			node.Operation = database_observability.ExplainPlanOutputOperationDuplicatesRemoval
		} else {
			node.Operation = database_observability.ExplainPlanOutputOperationOrderingOperation
		}
		if op.Sort.OrderBy != nil {
			node.Details.SortKeys = orderByColumns(op.Sort.OrderBy)
		}
		node.Children = childrenOf(op.Sort.RelOp)
	case op.ComputeScalar != nil:
		node.Operation = database_observability.ExplainPlanOutputOperationComputeScalar
		node.Children = childrenOf(op.ComputeScalar.RelOp)
	case op.Filter != nil:
		node.Operation = database_observability.ExplainPlanOutputOperationFilter
		if op.Filter.Predicate != nil {
			node.Details.Condition = redactedCondition(op.Filter.Predicate)
		}
		node.Children = childrenOf(op.Filter.RelOp)
	case op.Top != nil:
		node.Operation = database_observability.ExplainPlanOutputOperationTop
		node.Children = childrenOf(op.Top.RelOp)
	case op.Concat != nil:
		node.Operation = database_observability.ExplainPlanOutputOperationUnion
		node.Children = childrenOf(op.Concat.RelOp)
	case op.Parallelism != nil:
		node.Operation = database_observability.ExplainPlanOutputOperationParallelism
		node.Children = childrenOf(op.Parallelism.RelOp)
	case op.TableSpool != nil:
		node.Operation = database_observability.ExplainPlanOutputOperationSpool
		node.Children = childrenOf(op.TableSpool.RelOp)
	case op.IndexSpool != nil:
		node.Operation = database_observability.ExplainPlanOutputOperationSpool
		node.Children = childrenOf(op.IndexSpool.RelOp)
	default:
		node.Operation = database_observability.ExplainPlanOutputOperationUnknown
	}

	return node
}

// populateHashDetails disambiguates SQL Server's "Hash Match" operator, which
// serves as a join, an aggregate, or a set operation depending on LogicalOp -
// PhysicalOp alone is not enough to classify it.
func populateHashDetails(node *database_observability.ExplainPlanNode, op xmlRelOp) {
	switch {
	case strings.Contains(op.LogicalOp, "Join") || strings.Contains(op.LogicalOp, "Semi"):
		node.Operation = database_observability.ExplainPlanOutputOperationHashJoin
		algo := database_observability.ExplainPlanJoinAlgorithmHash
		node.Details.JoinAlgorithm = &algo
		joinType := op.LogicalOp
		node.Details.JoinType = &joinType
	case strings.EqualFold(op.LogicalOp, "Aggregate"):
		node.Operation = database_observability.ExplainPlanOutputOperationGroupingOperation
		if op.Hash.GroupBy != nil {
			node.Details.GroupByKeys = columnReferences(op.Hash.GroupBy.ColumnReference)
		}
	case strings.EqualFold(op.LogicalOp, "Union"):
		node.Operation = database_observability.ExplainPlanOutputOperationUnion
	case strings.EqualFold(op.LogicalOp, "Distinct"):
		node.Operation = database_observability.ExplainPlanOutputOperationDuplicatesRemoval
	default:
		node.Operation = database_observability.ExplainPlanOutputOperationUnknown
	}
}

func populateScanDetails(node *database_observability.ExplainPlanNode, scan xmlIndexScan) {
	if table := stripBrackets(scan.Object.Table); table != "" {
		node.Details.TableName = &table
	}
	if index := stripBrackets(scan.Object.Index); index != "" {
		node.Details.KeyUsed = &index
	}

	accessType := scanAccessType(node.Operation, scan)
	node.Details.AccessType = &accessType

	if scan.Predicate != nil {
		node.Details.Condition = redactedCondition(scan.Predicate)
	}
}

func scanAccessType(operation database_observability.ExplainPlanOutputOperation, scan xmlIndexScan) database_observability.ExplainPlanAccessType {
	switch {
	case scan.Lookup == "true" || scan.Lookup == "1":
		return database_observability.ExplainPlanAccessTypeEqRef
	case operation == database_observability.ExplainPlanOutputOperationTableScan:
		return database_observability.ExplainPlanAccessTypeAll
	case scan.Object.Index != "":
		return database_observability.ExplainPlanAccessTypeIndex
	default:
		return database_observability.ExplainPlanAccessTypeAll
	}
}

func redactedCondition(predicate *xmlPredicate) *string {
	if predicate.ScalarOperator.ScalarString == "" {
		return nil
	}
	redacted := database_observability.RedactSql(predicate.ScalarOperator.ScalarString)
	return &redacted
}

func childrenOf(relOps []xmlRelOp) []database_observability.ExplainPlanNode {
	if len(relOps) == 0 {
		return nil
	}
	children := make([]database_observability.ExplainPlanNode, 0, len(relOps))
	for _, child := range relOps {
		children = append(children, relOpToExplainPlanNode(child))
	}
	return children
}

func columnReferences(refs []xmlColumnReference) []string {
	if len(refs) == 0 {
		return nil
	}
	cols := make([]string, 0, len(refs))
	for _, ref := range refs {
		if col := stripBrackets(ref.Column); col != "" {
			cols = append(cols, col)
		}
	}
	return cols
}

func orderByColumns(orderBy *xmlOrderBy) []string {
	if orderBy == nil || len(orderBy.OrderByColumn) == 0 {
		return nil
	}
	cols := make([]string, 0, len(orderBy.OrderByColumn))
	for _, col := range orderBy.OrderByColumn {
		if name := stripBrackets(col.ColumnReference.Column); name != "" {
			cols = append(cols, name)
		}
	}
	return cols
}

func (w *xmlWarnings) strings() []string {
	var warnings []string
	if w.NoJoinPredicate != nil {
		warnings = append(warnings, "no join predicate (cartesian product)")
	}
	if w.ColumnsWithNoStatistics != nil {
		warnings = append(warnings, "columns with no statistics")
	}
	if w.SpillToTempDb != nil {
		warnings = append(warnings, "spilled to tempdb")
	}
	return warnings
}

// stripBrackets removes the [schema]/[object] bracket-quoting SQL Server uses
// for identifiers in showplan XML attributes (e.g. "[dbo].[Orders]").
func stripBrackets(identifier string) string {
	return strings.NewReplacer("[", "", "]", "").Replace(identifier)
}

func parseInt64OrZero(s string) int64 {
	var v int64
	if s == "" {
		return 0
	}
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
		return 0
	}
	return v
}

func parseFloatPtrOrNil(s string) *float64 {
	if s == "" {
		return nil
	}
	var v float64
	if _, err := fmt.Sscanf(s, "%g", &v); err != nil {
		return nil
	}
	return &v
}
