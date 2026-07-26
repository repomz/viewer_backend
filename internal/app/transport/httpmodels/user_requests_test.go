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
