package httpmodels

import "testing"

func TestGetReportPeriodMustBeOneToFour(t *testing.T) {
	for _, period := range []any{float64(0), float64(5), 1.5, "2"} {
		request := UserRequestCreateRequest{
			UserID:  "operator",
			AgentID: 2,
			Command: "get_report",
			Payload: map[string]any{"period": period},
		}
		if err := request.Validate(); err == nil {
			t.Fatalf("period %#v unexpectedly accepted", period)
		}
	}

	for _, period := range []any{float64(1), float64(4)} {
		request := UserRequestCreateRequest{
			UserID:  "operator",
			AgentID: 2,
			Command: "get_report",
			Payload: map[string]any{"period": period},
		}
		if err := request.Validate(); err != nil {
			t.Fatalf("period %#v rejected: %v", period, err)
		}
	}
}

func TestOnlyCanonicalAgentCommandsAreAccepted(t *testing.T) {
	for _, command := range []string{
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
		{UserID: "operator", AgentID: 2, Command: "get_plan"},
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
