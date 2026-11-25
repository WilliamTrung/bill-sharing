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

	var exists bool
	err := s.db.QueryRow(r.Context(),
		`SELECT EXISTS (SELECT 1 FROM bill_attachments WHERE bill_id = $1 AND stored_filename = $2)`,
		billID, filename,
	).Scan(&exists)
	if err != nil {
		s.logger.Error("lookup attachment", "error", err)
		writeError(w, http.StatusInternalServerError, "Could not load attachment")
		return
	}
	if !exists {
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
