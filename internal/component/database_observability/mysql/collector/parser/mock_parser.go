package parser

import (
	"github.com/go-kit/log"
	"github.com/stretchr/testify/mock"
)

type MockParser struct {
	mock.Mock
}

func (m *MockParser) Parse(sql string) (any, error) {
	args := m.Called(sql)
	return args.Get(0), args.Error(1)
}

func (m *MockParser) Redact(sql string) (string, error) {
	args := m.Called(sql)
	return args.String(0), args.Error(1)
}

func (m *MockParser) StmtType(stmt any) StatementType {
	args := m.Called(stmt)
	return args.Get(0).(StatementType)
}

func (m *MockParser) ParseTableName(t any) string {
	args := m.Called(t)
	return args.String(0)
}

func (m *MockParser) ExtractTableNames(logger log.Logger, digest string, stmt any) []string {
	args := m.Called(logger, digest, stmt)
	return args.Get(0).([]string)
}

func (m *MockParser) CleanTruncatedText(sql string) (string, error) {
	args := m.Called(sql)
	return args.String(0), args.Error(1)
}
