package testlivedebugging

import (
	"context"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service"
)

type FakeInfo struct {
	ComponentName string
	Component     component.Component
}

type FakeServiceHost struct {
	service.Host
	ComponentsInfo map[component.ID]FakeInfo
}

func (h *FakeServiceHost) GetComponent(id component.ID, opts component.InfoOptions) (*component.Info, error) {
	info, exist := h.ComponentsInfo[id]
	if exist {
		return &component.Info{ID: id, ComponentName: info.ComponentName, Component: info.Component}, nil
	}

	return nil, component.ErrComponentNotFound
}

type FakeComponentLiveDebugging struct {
	ConsumersCount int
}

func (f *FakeComponentLiveDebugging) LiveDebugging(consumers int) {
	f.ConsumersCount = consumers
}

func (f *FakeComponentLiveDebugging) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (f *FakeComponentLiveDebugging) Update(_ component.Arguments) error {
	return nil
}

type FakeComponentNoLiveDebugging struct {
}

func (f *FakeComponentNoLiveDebugging) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (f *FakeComponentNoLiveDebugging) Update(_ component.Arguments) error {
	return nil
}

type Log struct {
	m    sync.Mutex
	logs []string
}

func NewLog() *Log {
	return &Log{
		logs: []string{},
	}
}

func (l *Log) Append(log string) {
	l.m.Lock()
	defer l.m.Unlock()
	l.logs = append(l.logs, log)
}

func (l *Log) Get() []string {
	l.m.Lock()
	defer l.m.Unlock()
	return append([]string{}, l.logs...)
}
