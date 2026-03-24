//go:build windows

// This code is copied from Promtail v1.6.2-0.20231004111112-07cbef92268a with minor changes.

package windowsevent

import (
	"fmt"
	"path/filepath"
	"syscall"

	jsoniter "github.com/json-iterator/go"
	"golang.org/x/sys/windows"

	"github.com/grafana/alloy/internal/component/loki/source/windowsevent/win_eventlog"
	"github.com/grafana/alloy/internal/loki/promtail/scrapeconfig"
)

type Event struct {
	Source   string `json:"source,omitempty"`
	Channel  string `json:"channel,omitempty"`
	Computer string `json:"computer,omitempty"`
	EventID  int    `json:"event_id,omitempty"`
	Version  int    `json:"version,omitempty"`

	Level  int `json:"level,omitempty"`
	Task   int `json:"task,omitempty"`
	Opcode int `json:"opCode,omitempty"`

	LevelText  string `json:"levelText,omitempty"`
	TaskText   string `json:"taskText,omitempty"`
	OpcodeText string `json:"opCodeText,omitempty"`

	Keywords      string       `json:"keywords,omitempty"`
	TimeCreated   string       `json:"timeCreated,omitempty"`
	EventRecordID int          `json:"eventRecordID,omitempty"`
	Correlation   *Correlation `json:"correlation,omitempty"`
	Execution     *Execution   `json:"execution,omitempty"`

	Security  *Security `json:"security,omitempty"`
	UserData  string    `json:"user_data,omitempty"`
	EventData string    `json:"event_data,omitempty"`
	Message   string    `json:"message,omitempty"`
}

type Security struct {
	UserID   string `json:"userId,omitempty"`
	UserName string `json:"userName,omitempty"`
}

type Execution struct {
	ProcessID   uint32 `json:"processId,omitempty"`
	ThreadID    uint32 `json:"threadId,omitempty"`
	ProcessName string `json:"processName,omitempty"`
}

type Correlation struct {
	ActivityID        string `json:"activityID,omitempty"`
	RelatedActivityID string `json:"relatedActivityID,omitempty"`
}

// formatLine format a Loki log line from a windows event.
func formatLine(cfg *scrapeconfig.WindowsEventsTargetConfig, event win_eventlog.Event) (string, error) {
	structuredEvent := Event{
		Source:        event.Source.Name,
		Channel:       event.Channel,
		Computer:      event.Computer,
		EventID:       event.EventID,
		Version:       event.Version,
		Level:         event.Level,
		Task:          event.Task,
		Opcode:        event.Opcode,
		LevelText:     event.LevelText,
		TaskText:      event.TaskText,
		OpcodeText:    event.OpcodeText,
		Keywords:      event.Keywords,
		TimeCreated:   event.TimeCreated.SystemTime,
		EventRecordID: event.EventRecordID,
	}

	if !cfg.ExcludeEventData {
		structuredEvent.EventData = string(event.EventData.InnerXML)
	}
	if !cfg.ExcludeUserData {
		structuredEvent.UserData = string(event.UserData.InnerXML)
	}
	if !cfg.ExcludeEventMessage {
		structuredEvent.Message = event.Message
	}
	if event.Correlation.ActivityID != "" || event.Correlation.RelatedActivityID != "" {
		structuredEvent.Correlation = &Correlation{
			ActivityID:        event.Correlation.ActivityID,
			RelatedActivityID: event.Correlation.RelatedActivityID,
		}
	}
	// best effort to get the username of the event.
	if event.Security.UserID != "" {
		var userName string
		usid, err := syscall.StringToSid(event.Security.UserID)
		if err == nil {
			username, domain, _, err := usid.LookupAccount("")
			if err == nil {
				userName = fmt.Sprint(domain, "\\", username)
			}
		}
		structuredEvent.Security = &Security{
			UserID:   event.Security.UserID,
			UserName: userName,
		}
	}
	if event.Execution.ProcessID != 0 {
		structuredEvent.Execution = &Execution{
			ProcessID: event.Execution.ProcessID,
			ThreadID:  event.Execution.ThreadID,
		}

		processName, err := GetProcessName(event.Execution.ProcessID)
		if err == nil {
			structuredEvent.Execution.ProcessName = processName
		}
	}
	return jsoniter.MarshalToString(structuredEvent)
}

// This function was tested via manual testing on Windows machines at scale and by changing the
// size of the buffer to ensure that the dynamic resizing works as expected.
// TODO: would be better to have a unit test for this (not easy to setup)
func GetProcessName(pid uint32) (string, error) {
	// PID 4 is always "System"
	if pid == 4 {
		return "System", nil
	}

	// PID 0 is always "Idle Process"
	if pid == 0 {
		return "Idle Process", nil
	}

	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", fmt.Errorf("failed to open process %d: %w", pid, err)
	}
	defer windows.CloseHandle(handle)

	// Support Windows long paths by dynamically resizing the buffer.
	size := uint32(512)
	maxSize := uint32(64 * 1024)
	for {
		buf := make([]uint16, size)
		err = windows.QueryFullProcessImageName(handle, 0, &buf[0], &size)
		if err == nil {
			return filepath.Base(windows.UTF16ToString(buf[:size])), nil
		}
		if err == windows.ERROR_INSUFFICIENT_BUFFER {
			if size >= maxSize {
				return "", fmt.Errorf("failed to get process name for %d: buffer size exceeded maximum limit", pid)
			}
			size *= 2
			continue
		}
		return "", fmt.Errorf("failed to get process name for %d: %w", pid, err)
	}
}
