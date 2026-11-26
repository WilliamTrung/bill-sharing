package handlers

import (
	"net/http"
	"net/url"

	"github.com/WilliamTrung/bill-sharing/internal/realtime"
	"github.com/coder/websocket"
)

// handleWebSocket upgrades an authenticated request into a long-lived event
// stream. The session cookie is the only credential, exactly as for the REST
// API, so there is no separate token scheme to secure. The route is project
// scoped, so the client also proves it belongs to the project it is listening
// for; events still carry their own project id because one user may have tabs
// open on different projects.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	pc, ok := projectFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Same-origin only. The request host is always allowed; adding the
		// configured public host covers access through the proxy, where the Host
		// header is the public domain but the app is reached over an alias.
		OriginPatterns: []string{publicHost(s.cfg.PublicBaseURL)},
	})
	if err != nil {
		// Accept has already written a response.
		s.logger.Warn("websocket accept failed", "error", err)
		return
	}

	s.logger.Info("websocket connected", "user_id", pc.UserID, "project_id", pc.ProjectID)
	s.hub.Serve(r.Context(), realtime.NewClient(pc.UserID, pc.ProjectID, conn))
	s.logger.Info("websocket disconnected", "user_id", pc.UserID, "project_id", pc.ProjectID)
}

// publicHost extracts the bare host from the configured public base URL so it
// can be matched as a WebSocket origin pattern.
func publicHost(base string) string {
	u, err := url.Parse(base)
	if err != nil {
		return ""
	}
	return u.Host
}
