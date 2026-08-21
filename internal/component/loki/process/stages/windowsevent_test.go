package stages

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

func TestWindowsEventStage(t *testing.T) {
	now := time.Now()

	type testCase struct {
		name     string
		source   string
		config   string
		entry    Entry
		expected Entry
	}

	tests := []testCase{
		{
			name:   "CustomSource",
			source: "CustomSource",
			config: `
			stage.windowsevent {
				source = "CustomSource"
			}
			`,
			entry: newEntry(map[string]any{
				"CustomSource": "This is a test message.\r\n\r\nKey1: Value 1\r\nKey2: Value 2",
			}, model.LabelSet{}, "not important", now),
			expected: newEntry(map[string]any{
				"Description": "This is a test message.",
				"Key1":        "Value 1",
				"Key2":        "Value 2",
			}, model.LabelSet{}, "not important", now),
		},
		{
			name: "DropInvalid",
			config: `
			stage.windowsevent {
				drop_invalid_labels = true
			}
			`,
			entry: newEntry(map[string]any{
				"message": "This is a test message.\r\n\r\n\xff\xfe\xfd: Value 1\r\nKey2: Value 2",
			}, model.LabelSet{}, "not important", now),
			expected: newEntry(map[string]any{
				"Description": "This is a test message.",
				"Key2":        "Value 2",
			}, model.LabelSet{}, "not important", now),
		},
		{
			name: "OverrideExisting",
			config: `
			stage.windowsevent {
				overwrite_existing = true
			}
			`,
			entry: newEntry(map[string]any{
				"message":      "This is a test message.\r\n\r\ntestOverride: newValue\r\nKey2: Value 2",
				"testOverride": "initial",
			}, model.LabelSet{}, "not important", now),
			expected: newEntry(map[string]any{
				"Description":  "This is a test message.",
				"Key2":         "Value 2",
				"testOverride": "newValue",
			}, model.LabelSet{}, "not important", now),
		},
		{
			name: "DontOverride",
			config: `
			stage.windowsevent {}
			`,
			entry: newEntry(map[string]any{
				"message":      "This is a test message.\r\n\r\ntestOverride: newValue\r\nKey2: Value 2",
				"testOverride": "initial",
			}, model.LabelSet{}, "not important", now),
			expected: newEntry(map[string]any{
				"Description":            "This is a test message.",
				"Key2":                   "Value 2",
				"testOverride":           "initial",
				"testOverride_extracted": "newValue",
			}, model.LabelSet{}, "not important", now),
		},
		{
			name: "System",
			config: `
			stage.windowsevent {}
			`,
			entry: newEntry(map[string]any{
				"message": "Windows Update started downloading an update.",
			}, model.LabelSet{}, "not important", now),
			expected: newEntry(map[string]any{
				"Description": "Windows Update started downloading an update.",
			}, model.LabelSet{}, "not important", now),
		},
		{
			name: "Setup",
			config: `
			stage.windowsevent {}
			`,
			entry: newEntry(map[string]any{
				"message": "Initiating changes for package KB5044285. Current state is Superseded. Target state is Absent. Client id: Arbiter.",
			}, model.LabelSet{}, "not important", now),
			expected: newEntry(map[string]any{
				"Description": "Initiating changes for package KB5044285. Current state is Superseded. Target state is Absent. Client id: Arbiter.",
			}, model.LabelSet{}, "not important", now),
		},
		{
			name: "Security1",
			config: `
			stage.windowsevent {}
			`,
			entry: newEntry(map[string]any{
				"message": "Credential Manager credentials were read.\r\n\r\nSubject:\r\n\tSecurity ID:\t\tS-1-5-21-1111111-1111111-1111111-1111\r\n\tAccount Name:\t\tBob\r\n\tAccount Domain:\t\tDESKTOP-AAAAAA\r\n\tLogon ID:\t\t0x11111111\r\n\tRead Operation:\t\tEnumerate Credentials\r\n\r\nThis event occurs when a user performs a read operation on stored credentials in Credential Manager.",
			}, model.LabelSet{}, "not important", now),
			expected: newEntry(map[string]any{
				"Description":           "Credential Manager credentials were read.",
				"Subject_SecurityID":    "S-1-5-21-1111111-1111111-1111111-1111",
				"Subject_AccountName":   "Bob",
				"Subject_AccountDomain": "DESKTOP-AAAAAA",
				"Subject_LogonID":       "0x11111111",
				"Subject_ReadOperation": "Enumerate Credentials",
			}, model.LabelSet{}, "not important", now),
		},
		{
			name: "Security2",
			config: `
			stage.windowsevent {}
			`,
			entry: newEntry(map[string]any{
				"message": "An account was successfully logged on.\r\n\r\nSubject:\r\n\tSecurity ID:\t\tS-1-1-1\r\n\tAccount Name:\t\tDESKTOP-AAAAA$\r\n\tAccount Domain:\t\tWORKGROUP\r\n\tLogon ID:\t\t0xAAA\r\n\r\nLogon Information:\r\n\tLogon Type:\t\t5\r\n\tRestricted Admin Mode:\t-\r\n\tRemote Credential Guard:\t-\r\n\tVirtual Account:\t\tNo\r\n\tElevated Token:\t\tYes\r\n\r\nImpersonation Level:\t\tImpersonation\r\n\r\nNew Logon:\r\n\tSecurity ID:\t\tS-1-1-1\r\n\tAccount Name:\t\tSYSTEM\r\n\tAccount Domain:\t\tNT AUTHORITY\r\n\tLogon ID:\t\t0xAAA\r\n\tLinked Logon ID:\t\t0x0\r\n\tNetwork Account Name:\t-\r\n\tNetwork Account Domain:\t-\r\n\tLogon GUID:\t\t{00000000-0000-0000-0000-000000000000}\r\n\r\nProcess Information:\r\n\tProcess ID:\t\t0x4c0\r\n\tProcess Name:\t\tC:\\Windows\\System32\\services.exe\r\n\r\nNetwork Information:\r\n\tWorkstation Name:\t-\r\n\tSource Network Address:\t-\r\n\tSource Port:\t\t-\r\n\r\nDetailed Authentication Information:\r\n\tLogon Process:\t\tAdvapi  \r\n\tAuthentication Package:\tNegotiate\r\n\tTransited Services:\t-\r\n\tPackage Name (NTLM only):\t-\r\n\tKey Length:\t\t0\r\n\r\nThis event is generated when a logon session is created. It is generated on the computer that was accessed.\r\n\r\nThe subject fields indicate the account on the local system which requested the logon. This is most commonly a service such as the Server service, or a local process such as Winlogon.exe or Services.exe.\r\n\r\nThe logon type field indicates the kind of logon that occurred. The most common types are 2 (interactive) and 3 (network).\r\n\r\nThe New Logon fields indicate the account for whom the new logon was created, i.e. the account that was logged on.\r\n\r\nThe network fields indicate where a remote logon request originated. Workstation name is not always available and may be left blank in some cases.\r\n\r\nThe impersonation level field indicates the extent to which a process in the logon session can impersonate.\r\n\r\nThe authentication information fields provide detailed information about this specific logon request.\r\n\t- Logon GUID is a unique identifier that can be used to correlate this event with a KDC event.\r\n\t- Transited services indicate which intermediate services have participated in this logon request.\r\n\t- Package name indicates which sub-protocol was used among the NTLM protocols.\r\n\t- Key length indicates the length of the generated session key. This will be 0 if no session key was requested.",
			}, model.LabelSet{}, "not important", now),
			expected: newEntry(map[string]any{
				"Description":                                             "An account was successfully logged on.",
				"Subject_SecurityID":                                      "S-1-1-1",
				"Subject_AccountName":                                     "DESKTOP-AAAAA$",
				"Subject_AccountDomain":                                   "WORKGROUP",
				"Subject_LogonID":                                         "0xAAA",
				"LogonInformation_LogonType":                              "5",
				"LogonInformation_RestrictedAdminMode":                    "-",
				"LogonInformation_RemoteCredentialGuard":                  "-",
				"LogonInformation_VirtualAccount":                         "No",
				"LogonInformation_ElevatedToken":                          "Yes",
				"ImpersonationLevel":                                      "Impersonation",
				"NewLogon_SecurityID":                                     "S-1-1-1",
				"NewLogon_AccountName":                                    "SYSTEM",
				"NewLogon_AccountDomain":                                  "NT AUTHORITY",
				"NewLogon_LogonID":                                        "0xAAA",
				"NewLogon_LinkedLogonID":                                  "0x0",
				"NewLogon_NetworkAccountName":                             "-",
				"NewLogon_NetworkAccountDomain":                           "-",
				"NewLogon_LogonGUID":                                      "{00000000-0000-0000-0000-000000000000}",
				"ProcessInformation_ProcessID":                            "0x4c0",
				"ProcessInformation_ProcessName":                          "C:\\Windows\\System32\\services.exe",
				"NetworkInformation_WorkstationName":                      "-",
				"NetworkInformation_SourceNetworkAddress":                 "-",
				"NetworkInformation_SourcePort":                           "-",
				"DetailedAuthenticationInformation_LogonProcess":          "Advapi",
				"DetailedAuthenticationInformation_AuthenticationPackage": "Negotiate",
				"DetailedAuthenticationInformation_TransitedServices":     "-",
				"DetailedAuthenticationInformation_PackageName(NTLMonly)": "-",
				"DetailedAuthenticationInformation_KeyLength":             "0",
			}, model.LabelSet{}, "not important", now),
		},
		{
			name: "Security3",
			config: `
			stage.windowsevent {}
			`,
			entry: newEntry(map[string]any{
				"message": "Special privileges assigned to new logon.\r\n\r\nSubject:\r\n\tSecurity ID:\t\tS-1-1-1\r\n\tAccount Name:\t\tSYSTEM\r\n\tAccount Domain:\t\tNT AUTHORITY\r\n\tLogon ID:\t\t0xAAA\r\n\r\nPrivileges:\t\tSeAssignPrimaryTokenPrivilege\r\n\t\t\tSeTcbPrivilege\r\n\t\t\tSeSecurityPrivilege\r\n\t\t\tSeTakeOwnershipPrivilege\r\n\t\t\tSeLoadDriverPrivilege\r\n\t\t\tSeBackupPrivilege\r\n\t\t\tSeRestorePrivilege\r\n\t\t\tSeDebugPrivilege\r\n\t\t\tSeAuditPrivilege\r\n\t\t\tSeSystemEnvironmentPrivilege\r\n\t\t\tSeImpersonatePrivilege\r\n\t\t\tSeDelegateSessionUserImpersonatePrivilege",
			}, model.LabelSet{}, "not important", now),
			expected: newEntry(map[string]any{
				"Description":           "Special privileges assigned to new logon.",
				"Subject_SecurityID":    "S-1-1-1",
				"Subject_AccountName":   "SYSTEM",
				"Subject_AccountDomain": "NT AUTHORITY",
				"Subject_LogonID":       "0xAAA",
				"Privileges":            "SeAssignPrimaryTokenPrivilege,SeTcbPrivilege,SeSecurityPrivilege,SeTakeOwnershipPrivilege,SeLoadDriverPrivilege,SeBackupPrivilege,SeRestorePrivilege,SeDebugPrivilege,SeAuditPrivilege,SeSystemEnvironmentPrivilege,SeImpersonatePrivilege,SeDelegateSessionUserImpersonatePrivilege",
			}, model.LabelSet{}, "not important", now),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.source != "" {
				tt.expected.Extracted[tt.source] = tt.entry.Extracted[tt.source]
			} else {
				tt.expected.Extracted[defaultWindowsEventSource] = tt.entry.Extracted[defaultWindowsEventSource]
			}

			runPipelineTest(t, loadConfig(tt.config), []Entry{tt.entry}, []Entry{tt.expected})
		})
	}
}

func TestValidateWindowsEvent(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name    string
		cfg     string
		wantErr bool
	}

	tests := []testCase{
		{
			name: "with source",
			cfg:  `stage.windowsevent { source = "msg"}`,
		},
		{
			name:    "emptu source",
			cfg:     `stage.windowsevent { source = ""}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config Configs
			err := syntax.Unmarshal([]byte(tt.cfg), &config)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
