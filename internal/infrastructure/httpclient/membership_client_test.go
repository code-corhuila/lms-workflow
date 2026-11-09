package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSuspendPostsToStudentEndpoint(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/students/student-1/suspend" {
			t.Errorf("request = %s %s, want POST /api/v1/students/student-1/suspend", r.Method, r.URL.Path)
		}
		requireServiceToken(t, r)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := NewMembershipClient(srv.URL, testSecret).Suspend(context.Background(), "student-1"); err != nil {
		t.Fatalf("Suspend() error = %v", err)
	}
	if !called {
		t.Error("server was never called")
	}
}

func TestSuspendNon200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	if err := NewMembershipClient(srv.URL, testSecret).Suspend(context.Background(), "student-1"); err == nil {
		t.Error("Suspend() error = nil, want an error for 403")
	}
}

func TestSuspendUnreachableIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	if err := NewMembershipClient(url, testSecret).Suspend(context.Background(), "student-1"); err == nil {
		t.Error("Suspend() error = nil, want a connection error")
	}
}
