package media

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/roma007/OpenQuizGo/internal/admin"
)

type Service struct {
	dir      string
	maxBytes int64
}

func New(dir string, maxMB int) *Service {
	if dir == "" {
		dir = "./data/uploads"
	}
	if maxMB <= 0 {
		maxMB = 5
	}
	return &Service{dir: dir, maxBytes: int64(maxMB) << 20}
}

func (s *Service) Routes(r chi.Router, guard *admin.Service) {
	r.Get("/media/{name}", s.handleGet)
	r.Group(func(r chi.Router) {
		r.Use(guard.Auth)
		r.With(guard.Require("question.write")).Post("/admin/media", s.handleUpload)
	})
}

func (s *Service) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBytes+(1<<20))
	if err := r.ParseMultipartForm(s.maxBytes); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "文件过大或表单不合法")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "缺少文件")
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, s.maxBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "读取文件失败")
		return
	}
	if int64(len(data)) > s.maxBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "图片不能超过限制")
		return
	}
	name, contentType, err := s.save(data)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url":          "/v1/media/" + name,
		"bytes":        len(data),
		"content_type": contentType,
	})
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !safeName(name) {
		writeError(w, http.StatusNotFound, "文件不存在")
		return
	}
	path := filepath.Join(s.dir, name)
	if _, err := os.Stat(path); err != nil {
		writeError(w, http.StatusNotFound, "文件不存在")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, path)
}

func (s *Service) save(data []byte) (string, string, error) {
	ext, contentType, err := imageExt(data)
	if err != nil {
		ext, contentType, err = audioExt(data)
	}
	if err != nil {
		return "", "", errors.New("仅支持 JPEG/PNG/WebP/GIF/MP3/WAV/M4A/OGG")
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return "", "", errors.New("存储不可用")
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", "", errors.New("生成文件名失败")
	}
	name := hex.EncodeToString(raw) + ext
	if err := os.WriteFile(filepath.Join(s.dir, name), data, 0o644); err != nil {
		return "", "", errors.New("写入文件失败")
	}
	return name, contentType, nil
}

func imageExt(data []byte) (string, string, error) {
	if len(data) < 8 {
		return "", "", errors.New("不是有效图片")
	}
	contentType := http.DetectContentType(data)
	switch contentType {
	case "image/jpeg":
		return ".jpg", contentType, nil
	case "image/png":
		return ".png", contentType, nil
	case "image/gif":
		return ".gif", contentType, nil
	case "image/webp":
		return ".webp", contentType, nil
	default:
		return "", "", errors.New("仅支持 JPEG/PNG/WebP/GIF")
	}
}

// audioExt 依据文件头识别常见音频格式，避免仅凭扩展名放行。
func audioExt(data []byte) (string, string, error) {
	if len(data) >= 3 && string(data[:3]) == "ID3" {
		return ".mp3", "audio/mpeg", nil
	}
	// MP3 帧同步：11 位全 1。
	if len(data) >= 2 && data[0] == 0xFF && data[1]&0xE0 == 0xE0 {
		return ".mp3", "audio/mpeg", nil
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WAVE" {
		return ".wav", "audio/wav", nil
	}
	if len(data) >= 4 && string(data[:4]) == "OggS" {
		return ".ogg", "audio/ogg", nil
	}
	if len(data) >= 12 && string(data[4:8]) == "ftyp" {
		switch string(data[8:12]) {
		case "M4A ", "M4B ", "mp42", "isom", "iso2":
			return ".m4a", "audio/mp4", nil
		}
	}
	return "", "", errors.New("不是有效音频")
}

func safeName(name string) bool {
	return name != "" && filepath.Base(name) == name && !strings.Contains(name, "..")
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"code": "MEDIA_ERROR", "message": message})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
