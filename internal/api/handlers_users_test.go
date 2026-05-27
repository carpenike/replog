package api

import (
	"fmt"
	"net/http"
	"testing"
)

// --- ListUsers ---

func TestListUsers_AdminSeesAll(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	env.createUser(t, "coach1", true, false)
	env.createUser(t, "coach2", true, false)
	env.createUser(t, "athleteuser", false, false)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "GET", "/api/users", nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got []*UserWithAthlete
	decodeJSON(t, rr, &got)
	if len(got) != 4 {
		t.Errorf("expected 4 users, got %d", len(got))
	}
}

func TestListUsers_NonAdminForbidden(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "GET", "/api/users", nil, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestListUsers_Unauthenticated(t *testing.T) {
	env := setupTest(t)
	rr := env.do(t, "GET", "/api/users", nil, nil)
	requireStatus(t, rr, http.StatusUnauthorized)
}

// --- CreateUser ---

func TestCreateUser_Admin(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "POST", "/api/users",
		`{"username":"newcoach","name":"Coach","password":"hunter22","email":"c@x.com","is_coach":true}`,
		cookies)
	requireStatus(t, rr, http.StatusCreated)

	var got User
	decodeJSON(t, rr, &got)
	if got.Username != "newcoach" {
		t.Errorf("got username %q, want newcoach", got.Username)
	}
	if !got.IsCoach || got.IsAdmin {
		t.Errorf("got is_coach=%v is_admin=%v, want coach-only", got.IsCoach, got.IsAdmin)
	}
}

func TestCreateUser_NonAdminForbidden(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST", "/api/users", `{"username":"x","password":"y"}`, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestCreateUser_DuplicateUsername(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	env.createUser(t, "alice", false, false)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "POST", "/api/users",
		`{"username":"alice","password":"hunter22"}`, cookies)
	requireStatus(t, rr, http.StatusConflict)
}

func TestCreateUser_RequiresUsername(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "POST", "/api/users",
		`{"username":"","password":"hunter22"}`, cookies)
	requireStatus(t, rr, http.StatusBadRequest)
}

// --- GetUser ---

func TestGetUser_Admin(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	target := env.createUser(t, "alice", true, false)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "GET", fmt.Sprintf("/api/users/%d", target.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got User
	decodeJSON(t, rr, &got)
	if got.ID != target.ID || got.Username != "alice" {
		t.Errorf("got %+v, want id=%d username=alice", got, target.ID)
	}
}

func TestGetUser_NonAdminForbidden(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	target := env.createUser(t, "alice", false, false)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "GET", fmt.Sprintf("/api/users/%d", target.ID), nil, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestGetUser_NotFound(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "GET", "/api/users/9999", nil, cookies)
	requireStatus(t, rr, http.StatusNotFound)
}

// --- UpdateUser ---

func TestUpdateUser_Admin(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	target := env.createUser(t, "alice", false, false)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "PUT", fmt.Sprintf("/api/users/%d", target.ID),
		`{"username":"alice","name":"Alice Smith","email":"alice@example.com","is_coach":true}`,
		cookies)
	requireStatus(t, rr, http.StatusOK)

	var got User
	decodeJSON(t, rr, &got)
	if !got.IsCoach {
		t.Error("expected is_coach=true after update")
	}
}

func TestUpdateUser_PasswordChange(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	target := env.createUser(t, "alice", false, false)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "PUT", fmt.Sprintf("/api/users/%d", target.ID),
		`{"username":"alice","password":"newpass99"}`, cookies)
	requireStatus(t, rr, http.StatusOK)

	// Old password should now fail.
	rr = env.do(t, "POST", "/api/login",
		`{"username":"alice","password":"password123"}`, nil)
	requireStatus(t, rr, http.StatusUnauthorized)

	// New password should work.
	rr = env.do(t, "POST", "/api/login",
		`{"username":"alice","password":"newpass99"}`, nil)
	requireStatus(t, rr, http.StatusOK)
}

func TestUpdateUser_NonAdminForbidden(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	target := env.createUser(t, "alice", false, false)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "PUT", fmt.Sprintf("/api/users/%d", target.ID),
		`{"username":"alice","is_admin":true}`, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestUpdateUser_DuplicateUsername(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	env.createUser(t, "alice", false, false)
	bob := env.createUser(t, "bob", false, false)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "PUT", fmt.Sprintf("/api/users/%d", bob.ID),
		`{"username":"alice"}`, cookies)
	requireStatus(t, rr, http.StatusConflict)
}

// --- DeleteUser ---

func TestDeleteUser_Admin(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	target := env.createUser(t, "alice", false, false)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "DELETE", fmt.Sprintf("/api/users/%d", target.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	rr = env.do(t, "GET", fmt.Sprintf("/api/users/%d", target.ID), nil, cookies)
	requireStatus(t, rr, http.StatusNotFound)
}

func TestDeleteUser_CannotDeleteSelf(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "DELETE", fmt.Sprintf("/api/users/%d", admin.ID), nil, cookies)
	requireStatus(t, rr, http.StatusBadRequest)
}

func TestDeleteUser_NonAdminForbidden(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	target := env.createUser(t, "alice", false, false)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "DELETE", fmt.Sprintf("/api/users/%d", target.ID), nil, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

// --- SetUserMCPAccess (HOF-004) ---

func TestSetUserMCPAccess_AdminCanToggle(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	target := env.createUser(t, "coach1", true, false)
	cookies := env.loginAs(t, admin)

	// Default is false (default-deny).
	rr := env.do(t, "GET", fmt.Sprintf("/api/users/%d", target.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var got User
	decodeJSON(t, rr, &got)
	if got.MCPEnabled {
		t.Fatalf("expected mcp_enabled=false at start, got true")
	}

	// Enable.
	rr = env.do(t, "PUT", fmt.Sprintf("/api/users/%d/mcp", target.ID),
		MCPAccessRequest{Enabled: true}, cookies)
	requireStatus(t, rr, http.StatusOK)
	decodeJSON(t, rr, &got)
	if !got.MCPEnabled {
		t.Errorf("expected mcp_enabled=true after PUT enable, got false")
	}

	// Disable.
	rr = env.do(t, "PUT", fmt.Sprintf("/api/users/%d/mcp", target.ID),
		MCPAccessRequest{Enabled: false}, cookies)
	requireStatus(t, rr, http.StatusOK)
	decodeJSON(t, rr, &got)
	if got.MCPEnabled {
		t.Errorf("expected mcp_enabled=false after PUT disable, got true")
	}
}

func TestSetUserMCPAccess_NonAdminForbidden(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach1", true, false)
	target := env.createUser(t, "coach2", true, false)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "PUT", fmt.Sprintf("/api/users/%d/mcp", target.ID),
		MCPAccessRequest{Enabled: true}, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestSetUserMCPAccess_NotFound(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "PUT", "/api/users/999999/mcp",
		MCPAccessRequest{Enabled: true}, cookies)
	requireStatus(t, rr, http.StatusNotFound)
}
