package api

import (
	"net/http"

	"github.com/carpenike/replog/internal/middleware"
)

// serviceCoachingRefusal is the 403 a coach service token receives on a
// coaching-decision write. The same user's browser session is not refused:
// approving still happens in the browser (ADR 007).
const serviceCoachingRefusal = "service token cannot make this coaching change"

// refuseServiceCoaching writes 403 and reports true when this request
// authenticated with a coach service token. Call it at the start of a
// handler that assigns a program, approves a program draft, creates an
// approved template, executes a generation, writes a training max, or
// promotes an athlete. Writing a draft is not refused. Object-level
// checks on the handlers this does not refuse are unchanged.
func refuseServiceCoaching(w http.ResponseWriter, r *http.Request) bool {
	if !middleware.IsServiceCredential(r.Context()) {
		return false
	}
	WriteError(w, http.StatusForbidden, serviceCoachingRefusal)
	return true
}
