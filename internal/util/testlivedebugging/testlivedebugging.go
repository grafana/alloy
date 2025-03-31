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

func (h *FakeServiceHost) ListComponents(moduleID string, opts component.InfoOptions) ([]*component.Info, error) {
	if moduleID != "" {
		for key := range h.ComponentsInfo {
			if key.ModuleID == moduleID {
				return h.getComponentsInModule(moduleID), nil
			}
		}
		return nil, component.ErrModuleNotFound
	}
	return h.getComponentsInModule(""), nil
}

func (h *FakeServiceHost) getComponentsInModule(module string) []*component.Info {
	detail := make([]*component.Info, 0, len(h.ComponentsInfo))
	for key, cp := range h.ComponentsInfo {
		if key.ModuleID == module {
			detail = append(detail, &component.Info{ID: key, ComponentName: cp.ComponentName, Component: cp.Component})
		}
	}
	return detail
}

type FakeComponentLiveDebugging struct{}

func (f *FakeComponentLiveDebugging) LiveDebugging() {}

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
