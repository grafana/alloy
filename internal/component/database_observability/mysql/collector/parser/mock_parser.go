package parser

import (
	"github.com/stretchr/testify/mock"
)

type MockParser struct {
	mock.Mock
}

func (m *MockParser) Parse(sql string) (StatementAstNode, error) {
	args := m.Called(sql)
	return args.Get(0).(StatementAstNode), args.Error(1)
}

func (m *MockParser) Redact(sql string) (string, error) {
	args := m.Called(sql)
	return args.String(0), args.Error(1)
}

func (m *MockParser) ExtractTableNames(stmt StatementAstNode) []string {
	args := m.Called(stmt)
	return args.Get(0).([]string)
}

func (m *MockParser) CleanTruncatedText(sql string) (string, error) {
	args := m.Called(sql)
	return args.String(0), args.Error(1)
}
