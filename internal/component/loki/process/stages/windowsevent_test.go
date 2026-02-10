package stages

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
)

var testWindowsEventMsgDefaults = `
stage.windowsevent {}
`

var testWindowsEventMsgCustomSource = `
stage.windowsevent { source = "CustomSource" }
`

var testWindowsEventMsgDropInvalidLabels = `
stage.windowsevent { drop_invalid_labels = true }
`

var testWindowsEventMsgOverwriteExisting = `
stage.windowsevent { overwrite_existing = true }
`

func TestWindowsEvent(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		msgdata         string
		extractedValues map[string]any
	}{
		"System": {
			msgdata: "Windows Update started downloading an update.",
			extractedValues: map[string]any{
				"Description": "Windows Update started downloading an update.",
			},
		},
		"Setup": {
			msgdata: "Initiating changes for package KB5044285. Current state is Superseded. Target state is Absent. Client id: Arbiter.",
			extractedValues: map[string]any{
				"Description": "Initiating changes for package KB5044285. Current state is Superseded. Target state is Absent. Client id: Arbiter.",
			},
		},
		"Security1": {
			msgdata: "Credential Manager credentials were read.\r\n\r\nSubject:\r\n\tSecurity ID:\t\tS-1-5-21-1111111-1111111-1111111-1111\r\n\tAccount Name:\t\tBob\r\n\tAccount Domain:\t\tDESKTOP-AAAAAA\r\n\tLogon ID:\t\t0x11111111\r\n\tRead Operation:\t\tEnumerate Credentials\r\n\r\nThis event occurs when a user performs a read operation on stored credentials in Credential Manager.",
			extractedValues: map[string]any{
				"Description":           "Credential Manager credentials were read.",
				"Subject_SecurityID":    "S-1-5-21-1111111-1111111-1111111-1111",
				"Subject_AccountName":   "Bob",
				"Subject_AccountDomain": "DESKTOP-AAAAAA",
				"Subject_LogonID":       "0x11111111",
				"Subject_ReadOperation": "Enumerate Credentials",
			},
		},
		"Security2": {
			msgdata: "An account was successfully logged on.\r\n\r\nSubject:\r\n\tSecurity ID:\t\tS-1-1-1\r\n\tAccount Name:\t\tDESKTOP-AAAAA$\r\n\tAccount Domain:\t\tWORKGROUP\r\n\tLogon ID:\t\t0xAAA\r\n\r\nLogon Information:\r\n\tLogon Type:\t\t5\r\n\tRestricted Admin Mode:\t-\r\n\tRemote Credential Guard:\t-\r\n\tVirtual Account:\t\tNo\r\n\tElevated Token:\t\tYes\r\n\r\nImpersonation Level:\t\tImpersonation\r\n\r\nNew Logon:\r\n\tSecurity ID:\t\tS-1-1-1\r\n\tAccount Name:\t\tSYSTEM\r\n\tAccount Domain:\t\tNT AUTHORITY\r\n\tLogon ID:\t\t0xAAA\r\n\tLinked Logon ID:\t\t0x0\r\n\tNetwork Account Name:\t-\r\n\tNetwork Account Domain:\t-\r\n\tLogon GUID:\t\t{00000000-0000-0000-0000-000000000000}\r\n\r\nProcess Information:\r\n\tProcess ID:\t\t0x4c0\r\n\tProcess Name:\t\tC:\\Windows\\System32\\services.exe\r\n\r\nNetwork Information:\r\n\tWorkstation Name:\t-\r\n\tSource Network Address:\t-\r\n\tSource Port:\t\t-\r\n\r\nDetailed Authentication Information:\r\n\tLogon Process:\t\tAdvapi  \r\n\tAuthentication Package:\tNegotiate\r\n\tTransited Services:\t-\r\n\tPackage Name (NTLM only):\t-\r\n\tKey Length:\t\t0\r\n\r\nThis event is generated when a logon session is created. It is generated on the computer that was accessed.\r\n\r\nThe subject fields indicate the account on the local system which requested the logon. This is most commonly a service such as the Server service, or a local process such as Winlogon.exe or Services.exe.\r\n\r\nThe logon type field indicates the kind of logon that occurred. The most common types are 2 (interactive) and 3 (network).\r\n\r\nThe New Logon fields indicate the account for whom the new logon was created, i.e. the account that was logged on.\r\n\r\nThe network fields indicate where a remote logon request originated. Workstation name is not always available and may be left blank in some cases.\r\n\r\nThe impersonation level field indicates the extent to which a process in the logon session can impersonate.\r\n\r\nThe authentication information fields provide detailed information about this specific logon request.\r\n\t- Logon GUID is a unique identifier that can be used to correlate this event with a KDC event.\r\n\t- Transited services indicate which intermediate services have participated in this logon request.\r\n\t- Package name indicates which sub-protocol was used among the NTLM protocols.\r\n\t- Key length indicates the length of the generated session key. This will be 0 if no session key was requested.",
			extractedValues: map[string]any{
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
			},
		},
		"Security3": {
			msgdata: "Special privileges assigned to new logon.\r\n\r\nSubject:\r\n\tSecurity ID:\t\tS-1-1-1\r\n\tAccount Name:\t\tSYSTEM\r\n\tAccount Domain:\t\tNT AUTHORITY\r\n\tLogon ID:\t\t0xAAA\r\n\r\nPrivileges:\t\tSeAssignPrimaryTokenPrivilege\r\n\t\t\tSeTcbPrivilege\r\n\t\t\tSeSecurityPrivilege\r\n\t\t\tSeTakeOwnershipPrivilege\r\n\t\t\tSeLoadDriverPrivilege\r\n\t\t\tSeBackupPrivilege\r\n\t\t\tSeRestorePrivilege\r\n\t\t\tSeDebugPrivilege\r\n\t\t\tSeAuditPrivilege\r\n\t\t\tSeSystemEnvironmentPrivilege\r\n\t\t\tSeImpersonatePrivilege\r\n\t\t\tSeDelegateSessionUserImpersonatePrivilege",
			extractedValues: map[string]any{
				"Description":           "Special privileges assigned to new logon.",
				"Subject_SecurityID":    "S-1-1-1",
				"Subject_AccountName":   "SYSTEM",
				"Subject_AccountDomain": "NT AUTHORITY",
				"Subject_LogonID":       "0xAAA",
				"Privileges":            "SeAssignPrimaryTokenPrivilege,SeTcbPrivilege,SeSecurityPrivilege,SeTakeOwnershipPrivilege,SeLoadDriverPrivilege,SeBackupPrivilege,SeRestorePrivilege,SeDebugPrivilege,SeAuditPrivilege,SeSystemEnvironmentPrivilege,SeImpersonatePrivilege,SeDelegateSessionUserImpersonatePrivilege",
			},
		},
	}

	for testName, testData := range tests {
		testData := testData
		testData.extractedValues["message"] = testData.msgdata

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			pl, err := NewPipeline(log.NewNopLogger(), loadConfig(testWindowsEventMsgDefaults), nil, prometheus.DefaultRegisterer, featuregate.StabilityExperimental)
			require.NoError(t, err, "Expected pipeline creation to not result in error")
			out := processEntries(pl,
				newEntry(map[string]any{
					"message": testData.msgdata,
				}, nil, testData.msgdata, time.Now()))[0]
			require.Equal(t, testData.extractedValues, out.Extracted)
		})
	}
}

func TestWindowsEventArgs(t *testing.T) {
	tests := map[string]struct {
		config          string
		sourcekey       string
		msgdata         string
		extractedValues map[string]any
	}{
		"CustomSource": {
			config:    testWindowsEventMsgCustomSource,
			sourcekey: "CustomSource",
			msgdata:   "This is a test message.\r\n\r\nKey1: Value 1\r\nKey2: Value 2",
			extractedValues: map[string]any{
				"Description":  "This is a test message.",
				"Key1":         "Value 1",
				"Key2":         "Value 2",
				"testOverride": "initial",
			},
		},
		"DropInvalid": {
			config:    testWindowsEventMsgDropInvalidLabels,
			sourcekey: "message",
			msgdata:   "This is a test message.\r\n\r\n\xff\xfe\xfd: Value 1\r\nKey2: Value 2",
			extractedValues: map[string]any{
				"Description":  "This is a test message.",
				"Key2":         "Value 2",
				"testOverride": "initial",
			},
		},
		"OverrideExisting": {
			config:    testWindowsEventMsgOverwriteExisting,
			sourcekey: "message",
			msgdata:   "This is a test message.\r\n\r\ntestOverride: newValue\r\nKey2: Value 2",
			extractedValues: map[string]any{
				"Description":  "This is a test message.",
				"Key2":         "Value 2",
				"testOverride": "newValue",
			},
		},
		"DontOverride": {
			config:    testWindowsEventMsgDefaults,
			sourcekey: "message",
			msgdata:   "This is a test message.\r\n\r\ntestOverride: newValue\r\nKey2: Value 2",
			extractedValues: map[string]any{
				"Description":            "This is a test message.",
				"Key2":                   "Value 2",
				"testOverride":           "initial",
				"testOverride_extracted": "newValue",
			},
		},
	}
	for testName, testData := range tests {
		testData := testData
		testData.extractedValues[testData.sourcekey] = testData.msgdata

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			pl, err := NewPipeline(log.NewNopLogger(), loadConfig(testData.config), nil, prometheus.DefaultRegisterer, featuregate.StabilityExperimental)
			require.NoError(t, err, "Expected pipeline creation to not result in error")
			out := processEntries(pl,
				newEntry(map[string]any{
					testData.sourcekey: testData.msgdata,
					"testOverride":     "initial",
				}, nil, testData.msgdata, time.Now()))[0]
			require.Equal(t, testData.extractedValues, out.Extracted)
		})
	}
}

func TestWindowsEventValidate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		config string
		err    error
	}{
		"valid config": {
			`stage.windowsevent { source = "msg"}`,
			nil,
		},
		"empty source": {
			`stage.windowsevent { source = ""}`,
			fmt.Errorf(ErrInvalidLabelName, ""),
		},
	}
	for tName, tt := range tests {
		tt := tt
		t.Run(tName, func(t *testing.T) {
			var config Configs
			err := syntax.Unmarshal([]byte(tt.config), &config)
			if err == nil {
				require.Len(t, config.Stages, 1)
				err = config.Stages[0].WindowsEventConfig.Validate()
			}

			if err == nil && tt.err != nil {
				require.NotNil(t, err, "windowsevent.validate() expected error = %v, but got nil", tt.err)
			}
			if err != nil {
				require.Equal(t, tt.err.Error(), err.Error(), "windowsevent.validate() expected error = %v, actual error = %v", tt.err, err)
			}
		})
	}
}
