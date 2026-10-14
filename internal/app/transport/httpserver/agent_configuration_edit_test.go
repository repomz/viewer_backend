package httpserver

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRemoteConfigurationCannotUsePublicQueue(t *testing.T) {
	for _, command := range []string{"update_agent_config", " UPDATE_AGENT_CONFIG "} {
		service := &userRequestServiceStub{request: queuedUserRequest()}
		handler := NewHttpServer(nil, nil, service)
		recorder := httptest.NewRecorder()
		handler.CreateUserRequest(recorder, httptest.NewRequest("POST", "/user_requests", strings.NewReader(`{"user_id":"admin","agent_id":2,"command":"`+command+`","payload":{}}`)))
		if recorder.Code != 403 {
			t.Fatalf("command %q: status %d", command, recorder.Code)
		}
		if service.created.Command != "" {
			t.Fatal("unauthorized command queued")
		}
	}
}

func TestEditableAgentConfigurationValidation(t *testing.T) {
	valid := editableAgentConfiguration{Description: "Опер 2", Heartbeat: 1, Study: pollingSnapshot{State: true, Interval: 1, Directories: []string{`C:\protocols`}}, XA: pollingSnapshot{Interval: 5}, CT: pollingSnapshot{Interval: 5}, Requests: pollingSnapshot{State: true, Interval: 1}}
	if err := valid.validate(); err != nil {
		t.Fatal(err)
	}
	for _, modify := range []func(*editableAgentConfiguration){
		func(c *editableAgentConfiguration) { c.Requests.State = false },
		func(c *editableAgentConfiguration) { c.XA.Interval = 0 },
		func(c *editableAgentConfiguration) { c.Study.Directories = nil },
		func(c *editableAgentConfiguration) { c.Heartbeat = 61 },
		func(c *editableAgentConfiguration) { c.CT.Directories = []string{"secret"} },
	} {
		c := valid
		modify(&c)
		if c.validate() == nil {
			t.Fatal("invalid settings accepted")
		}
	}
}

func TestAgentConfigurationEditRequiresAdmin(t *testing.T) {
	recorder := httptest.NewRecorder()
	HttpServer{}.UpdateAgentConfiguration(recorder, httptest.NewRequest("PUT", "/agent_configurations/2", nil))
	if recorder.Code != 403 {
		t.Fatalf("status %d", recorder.Code)
	}
}
