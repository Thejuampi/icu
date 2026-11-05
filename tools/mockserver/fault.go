package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Fault injection (test-only, off until set; the mock is loopback-only):
//
//	POST   /_mock/fault?match=/sport-settings/&mode=401   every request whose
//	       path contains match answers that status (401, 403, 500, 503, any
//	       4xx/5xx) instead of its fixture; mode=close drops the connection
//	       without a response (a network failure).
//	DELETE /_mock/fault                                    clear it
//
// /_mock/reset also clears it. Injected faults are logged, never recorded as
// writes. One fault at a time; a new POST replaces it.
type fault struct {
	match string
	mode  string
}

const faultModeClose = "close"

func (s *server) handleFaultControl(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		s.setFault(nil)
		writeJSON(w, http.StatusOK, []byte(`{"fault":null}`))

		return
	}
	match, mode := r.URL.Query().Get("match"), r.URL.Query().Get("mode")
	if match == "" || strings.HasPrefix(match, "/_mock") {
		writeError(w, http.StatusBadRequest, "match must be a non-empty path fragment outside /_mock")

		return
	}
	if mode != faultModeClose {
		code, err := strconv.Atoi(mode)
		if err != nil || code < http.StatusBadRequest || code > 599 {
			writeError(w, http.StatusBadRequest, "mode must be close or a 4xx/5xx status")

			return
		}
	}
	s.setFault(&fault{match: match, mode: mode})
	writeJSON(w, http.StatusOK, []byte(fmt.Sprintf(`{"fault":{"match":%q,"mode":%q}}`, match, mode)))
}

func (s *server) setFault(f *fault) {
	s.mu.Lock()
	s.fault = f
	s.mu.Unlock()
}

// serveFault answers r with the injected fault when its path matches.
func (s *server) serveFault(w http.ResponseWriter, r *http.Request) bool {
	s.mu.Lock()
	f := s.fault
	s.mu.Unlock()
	if f == nil || strings.HasPrefix(r.URL.Path, "/_mock") || !strings.Contains(r.URL.Path, f.match) {
		return false
	}
	fmt.Fprintf(s.log, "fault %s %s -> %s\n", r.Method, r.URL.Path, f.mode)
	if f.mode == faultModeClose {
		if hijacker, ok := w.(http.Hijacker); ok {
			if conn, _, err := hijacker.Hijack(); err == nil {
				_ = conn.Close()

				return true
			}
		}
		panic(http.ErrAbortHandler) // no hijack support: abort the response
	}
	code, _ := strconv.Atoi(f.mode)
	writeError(w, code, "injected fault")

	return true
}
