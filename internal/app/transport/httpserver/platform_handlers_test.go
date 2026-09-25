package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestSeedPasswordHashes(t *testing.T) {
	cases := []struct {
		hash, password string
	}{
		{"$2y$12$O4/DGs2ejwLcjRhuL4xMzu2WZMFaTWI.x9iNS9/oURXr97mx7SODi", "1414"},
		{"$2y$12$uqu8ZjbKT0P0eD7zzhpubehoTYuI9tzVQ6Ku8orIiqTyxTUGSf3AO", "1111"},
	}
	for _, item := range cases {
		if err := bcrypt.CompareHashAndPassword([]byte(item.hash), []byte(item.password)); err != nil {
			t.Fatalf("seed password hash is invalid: %v", err)
		}
	}
}

func TestNewSessionTokenIsRandomAndHasHash(t *testing.T) {
	first, firstHash, err := newSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	second, secondHash, err := newSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if first == second || len(first) < 40 || len(firstHash) != 32 || len(secondHash) != 32 {
		t.Fatalf("unexpected session tokens: lengths %d/%d", len(first), len(second))
	}
}

func TestRequireAuthRejectsMissingToken(t *testing.T) {
	handler := HttpServer{}
	request := httptest.NewRequest(http.MethodGet, "/drive", nil)
	recorder := httptest.NewRecorder()
	handler.RequireAuth(func(http.ResponseWriter, *http.Request) {
		t.Fatal("protected handler must not be called")
	})(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestLoginValidation(t *testing.T) {
	for _, valid := range []string{"marat", "doctor.2", "user_name"} {
		if !loginPattern.MatchString(valid) {
			t.Fatalf("expected valid login %q", valid)
		}
	}
	for _, invalid := range []string{"я", "a", "name with spaces"} {
		if loginPattern.MatchString(invalid) {
			t.Fatalf("expected invalid login %q", invalid)
		}
	}
}
