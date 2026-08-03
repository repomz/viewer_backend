package httpmodels

import "testing"

func TestOnlyCanonicalAgentCommandsAreAccepted(t *testing.T) {
	for _, command := range []string{
		"get_report",
		"send_study_to_yandex",
		"send_dicom_to_mapdr",
		"generate_operations_report",
	} {
		request := UserRequestCreateRequest{
			UserID:  "operator",
			AgentID: 2,
			Command: command,
		}
		if err := request.Validate(); err == nil {
			t.Fatalf("legacy command %q unexpectedly accepted", command)
		}
	}
}

func TestInteractiveWorkflowCommandsAreAccepted(t *testing.T) {
	cases := []UserRequestCreateRequest{
		{UserID: "operator", AgentID: 2, Command: "sync_studies"},
		{
			UserID: "operator", AgentID: 2, Command: "import_study",
			Payload: map[string]any{"protocol_ref": "opaque-reference"},
		},
		{
			UserID: "operator", AgentID: 2, Command: "send_xa_to_pacs",
			Payload: map[string]any{"study_uid": "1.2.3"},
		},
		{UserID: "operator", AgentID: 2, Command: "xa_polling_on"},
		{UserID: "operator", AgentID: 2, Command: "ct_polling_off"},
	}
	for _, request := range cases {
		if err := request.Validate(); err != nil {
			t.Fatalf("command %q rejected: %v", request.Command, err)
		}
	}
}
