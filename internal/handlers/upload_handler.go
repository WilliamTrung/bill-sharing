package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// handleUploadedFile serves one attachment. The route is authenticated and the
// caller must belong to the bill's project, so a URL alone reveals nothing: the
// attachment URL is returned by bill reads, which are themselves scoped.
func (s *Server) handleUploadedFile(w http.ResponseWriter, r *http.Request) {
	billID, ok := pathID(w, r, "billID")
	if !ok {
		return
	}

	filename := path.Base(strings.ReplaceAll(r.PathValue("filename"), "\\", "/"))
	if filename == "" || filename == "." || filename == "/" || strings.Contains(filename, "..") {
		writeError(w, http.StatusBadRequest, "Invalid filename")
		return
	}

	// Resolve the caller's project membership in the same statement, so one query
	// both authorizes and locates the file.
	var exists bool
	err := s.db.QueryRow(r.Context(), `
		SELECT EXISTS (
			SELECT 1
			FROM bill_attachments a
			JOIN bills b ON b.id = a.bill_id
			JOIN project_members pm ON pm.project_id = b.project_id AND pm.user_id = $3
			WHERE a.bill_id = $1 AND a.stored_filename = $2
		)`, billID, filename, s.authUserID(r),
	).Scan(&exists)
	if err != nil {
		s.logger.Error("lookup attachment", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load attachment")
		return
	}
	if !exists {
		// 404 rather than 403: an outsider should not learn the bill exists.
		http.NotFound(w, r)
		return
	}

	file, err := os.Open(filepath.Join(s.cfg.UploadDir, strconv.FormatInt(billID, 10), filename))
	if errors.Is(err, os.ErrNotExist) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.logger.Error("open attachment", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load attachment")
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load attachment")
		return
	}

	w.Header().Set("Content-Type", mimeForExtension(filepath.Ext(filename)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename))
	http.ServeContent(w, r, filename, info.ModTime(), file)
}

// removeUploadDir deletes the directory holding a bill's attachments. The path is
// keyed by the global bill id, which is unique across projects.
func (s *Server) removeUploadDir(billID int64) error {
	return os.RemoveAll(filepath.Join(s.cfg.UploadDir, strconv.FormatInt(billID, 10)))
}

func mimeForExtension(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}
