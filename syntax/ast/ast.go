// Package ast exposes AST elements used by River.
//
// The various interfaces exposed by ast are all closed; only types within this
// package can satisfy an AST interface.
package ast

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/grafana/river/token"
)

// Node represents any node in the AST.
type Node interface {
	astNode()
}

// Stmt is a type of statement within the body of a file or block.
type Stmt interface {
	Node
	astStmt()
}

// Expr is an expression within the AST.
type Expr interface {
	Node
	astExpr()
}

// File is a parsed file.
type File struct {
	Name     string         // Filename provided to parser
	Body     Body           // Content of File
	Comments []CommentGroup // List of all comments in the File
}

// Body is a list of statements.
type Body []Stmt

// A CommentGroup represents a sequence of comments that are not separated by
// any empty lines or other non-comment tokens.
type CommentGroup []*Comment

// A Comment represents a single line or block comment.
//
// The Text field contains the comment text without any carriage returns (\r)
// that may have been present in the source. Since carriage returns get
// removed, EndPos will not be accurate for any comment which contained
// carriage returns.
type Comment struct {
	StartPos token.Pos // Starting position of comment
	// Text of the comment. Text will not contain '\n' for line comments.
	Text string
}

// AttributeStmt is a key-value pair being set in a Body or BlockStmt.
type AttributeStmt struct {
	Name  *Ident
	Value Expr
}

// BlockStmt declares a block.
type BlockStmt struct {
	Name     []string
	NamePos  token.Pos
	Label    string
	LabelPos token.Pos
	Body     Body

	LCurlyPos, RCurlyPos token.Pos
}

// Ident holds an identifier with its position.
type Ident struct {
	Name    string
	NamePos token.Pos
}

// IdentifierExpr refers to a named value.
type IdentifierExpr struct {
	Ident *Ident
}

// LiteralExpr is a constant value of a specific token kind.
type LiteralExpr struct {
	Kind     token.Token
	ValuePos token.Pos

	// Value holds the unparsed literal value. For example, if Kind ==
	// token.STRING, then Value would be wrapped in the original quotes (e.g.,
	// `"foobar"`).
	Value string
}

// ArrayExpr is an array of values.
type ArrayExpr struct {
	Elements             []Expr
	LBrackPos, RBrackPos token.Pos
}

// ObjectExpr declares an object of key-value pairs.
type ObjectExpr struct {
	Fields               []*ObjectField
	LCurlyPos, RCurlyPos token.Pos
}

// ObjectField defines an individual key-value pair within an object.
// ObjectField does not implement Node.
type ObjectField struct {
	Name   *Ident
	Quoted bool // True if the name was wrapped in quotes
	Value  Expr
}

// AccessExpr accesses a field in an object value by name.
type AccessExpr struct {
	Value Expr
	Name  *Ident
}

// IndexExpr accesses an index in an array value.
type IndexExpr struct {
	Value, Index         Expr
	LBrackPos, RBrackPos token.Pos
}

// CallExpr invokes a function value with a set of arguments.
type CallExpr struct {
	Value Expr
	Args  []Expr

	LParenPos, RParenPos token.Pos
}

// UnaryExpr performs a unary operation on a single value.
type UnaryExpr struct {
	Kind    token.Token
	KindPos token.Pos
	Value   Expr
}

// BinaryExpr performs a binary operation against two values.
type BinaryExpr struct {
	Kind        token.Token
	KindPos     token.Pos
	Left, Right Expr
}

// ParenExpr represents an expression wrapped in parentheses.
type ParenExpr struct {
	Inner                Expr
	LParenPos, RParenPos token.Pos
}

// Type assertions

var (
	_ Node = (*File)(nil)
	_ Node = (*Body)(nil)
	_ Node = (*AttributeStmt)(nil)
	_ Node = (*BlockStmt)(nil)
	_ Node = (*Ident)(nil)
	_ Node = (*IdentifierExpr)(nil)
	_ Node = (*LiteralExpr)(nil)
	_ Node = (*ArrayExpr)(nil)
	_ Node = (*ObjectExpr)(nil)
	_ Node = (*AccessExpr)(nil)
	_ Node = (*IndexExpr)(nil)
	_ Node = (*CallExpr)(nil)
	_ Node = (*UnaryExpr)(nil)
	_ Node = (*BinaryExpr)(nil)
	_ Node = (*ParenExpr)(nil)

	_ Stmt = (*AttributeStmt)(nil)
	_ Stmt = (*BlockStmt)(nil)

	_ Expr = (*IdentifierExpr)(nil)
	_ Expr = (*LiteralExpr)(nil)
	_ Expr = (*ArrayExpr)(nil)
	_ Expr = (*ObjectExpr)(nil)
	_ Expr = (*AccessExpr)(nil)
	_ Expr = (*IndexExpr)(nil)
	_ Expr = (*CallExpr)(nil)
	_ Expr = (*UnaryExpr)(nil)
	_ Expr = (*BinaryExpr)(nil)
	_ Expr = (*ParenExpr)(nil)
)

func (n *File) astNode()           {}
func (n Body) astNode()            {}
func (n CommentGroup) astNode()    {}
func (n *Comment) astNode()        {}
func (n *AttributeStmt) astNode()  {}
func (n *BlockStmt) astNode()      {}
func (n *Ident) astNode()          {}
func (n *IdentifierExpr) astNode() {}
func (n *LiteralExpr) astNode()    {}
func (n *ArrayExpr) astNode()      {}
func (n *ObjectExpr) astNode()     {}
func (n *AccessExpr) astNode()     {}
func (n *IndexExpr) astNode()      {}
func (n *CallExpr) astNode()       {}
func (n *UnaryExpr) astNode()      {}
func (n *BinaryExpr) astNode()     {}
func (n *ParenExpr) astNode()      {}

func (n *AttributeStmt) astStmt() {}
func (n *BlockStmt) astStmt()     {}

func (n *IdentifierExpr) astExpr() {}
func (n *LiteralExpr) astExpr()    {}
func (n *ArrayExpr) astExpr()      {}
func (n *ObjectExpr) astExpr()     {}
func (n *AccessExpr) astExpr()     {}
func (n *IndexExpr) astExpr()      {}
func (n *CallExpr) astExpr()       {}
func (n *UnaryExpr) astExpr()      {}
func (n *BinaryExpr) astExpr()     {}
func (n *ParenExpr) astExpr()      {}

// StartPos returns the position of the first character belonging to a Node.
func StartPos(n Node) token.Pos {
	if n == nil || reflect.ValueOf(n).IsZero() {
		return token.NoPos
	}
	switch n := n.(type) {
	case *File:
		return StartPos(n.Body)
	case Body:
		if len(n) == 0 {
			return token.NoPos
		}
		return StartPos(n[0])
	case CommentGroup:
		if len(n) == 0 {
			return token.NoPos
		}
		return StartPos(n[0])
	case *Comment:
		return n.StartPos
	case *AttributeStmt:
		return StartPos(n.Name)
	case *BlockStmt:
		return n.NamePos
	case *Ident:
		return n.NamePos
	case *IdentifierExpr:
		return StartPos(n.Ident)
	case *LiteralExpr:
		return n.ValuePos
	case *ArrayExpr:
		return n.LBrackPos
	case *ObjectExpr:
		return n.LCurlyPos
	case *AccessExpr:
		return StartPos(n.Value)
	case *IndexExpr:
		return StartPos(n.Value)
	case *CallExpr:
		return StartPos(n.Value)
	case *UnaryExpr:
		return n.KindPos
	case *BinaryExpr:
		return StartPos(n.Left)
	case *ParenExpr:
		return n.LParenPos
	default:
		panic(fmt.Sprintf("Unhandled Node type %T", n))
	}
}

// EndPos returns the position of the final character in a Node.
func EndPos(n Node) token.Pos {
	if n == nil || reflect.ValueOf(n).IsZero() {
		return token.NoPos
	}
	switch n := n.(type) {
	case *File:
		return EndPos(n.Body)
	case Body:
		if len(n) == 0 {
			return token.NoPos
		}
		return EndPos(n[len(n)-1])
	case CommentGroup:
		if len(n) == 0 {
			return token.NoPos
		}
		return EndPos(n[len(n)-1])
	case *Comment:
		return n.StartPos.Add(len(n.Text) - 1)
	case *AttributeStmt:
		return EndPos(n.Value)
	case *BlockStmt:
		return n.RCurlyPos
	case *Ident:
		return n.NamePos.Add(len(n.Name) - 1)
	case *IdentifierExpr:
		return EndPos(n.Ident)
	case *LiteralExpr:
		return n.ValuePos.Add(len(n.Value) - 1)
	case *ArrayExpr:
		return n.RBrackPos
	case *ObjectExpr:
		return n.RCurlyPos
	case *AccessExpr:
		return EndPos(n.Name)
	case *IndexExpr:
		return n.RBrackPos
	case *CallExpr:
		return n.RParenPos
	case *UnaryExpr:
		return EndPos(n.Value)
	case *BinaryExpr:
		return EndPos(n.Right)
	case *ParenExpr:
		return n.RParenPos
	default:
		panic(fmt.Sprintf("Unhandled Node type %T", n))
	}
}

// GetBlockName retrieves the "." delimited block name.
func (block *BlockStmt) GetBlockName() string {
	return strings.Join(block.Name, ".")
}
