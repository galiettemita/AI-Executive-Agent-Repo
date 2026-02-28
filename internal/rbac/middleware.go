package rbac

import (
	"net/http"

	"github.com/google/uuid"
)

const (
	HeaderWorkspaceID = "X-Workspace-ID"
	HeaderUserID      = "X-User-ID"
)

func (s *Service) RequireRole(minimumRole string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		workspaceIDRaw := r.Header.Get(HeaderWorkspaceID)
		userIDRaw := r.Header.Get(HeaderUserID)
		if workspaceIDRaw == "" || userIDRaw == "" {
			http.Error(w, "missing role context headers", http.StatusBadRequest)
			return
		}

		workspaceID, err := uuid.Parse(workspaceIDRaw)
		if err != nil {
			http.Error(w, "invalid workspace id", http.StatusBadRequest)
			return
		}
		userID, err := uuid.Parse(userIDRaw)
		if err != nil {
			http.Error(w, "invalid user id", http.StatusBadRequest)
			return
		}

		if err := s.CheckRoleAtLeast(workspaceID, userID, minimumRole); err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
