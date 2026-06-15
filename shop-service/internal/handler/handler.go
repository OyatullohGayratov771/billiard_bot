package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"shop-service/internal/models"
	"shop-service/internal/service"
)

const maxImage = 12 << 20 // 12 MB

type Handler struct {
	svc           *service.Service
	internalToken string
}

func New(svc *service.Service, internalToken string) *Handler {
	return &Handler{svc: svc, internalToken: internalToken}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.health)

	// Public (mijoz)
	mux.HandleFunc("GET /products", h.listInStock)
	mux.HandleFunc("GET /uploads/{file}", h.serveImage)

	// Internal (web-service / bot) — X-Internal-Token
	mux.HandleFunc("GET /products/all", h.internal(h.listAll))
	mux.HandleFunc("POST /products", h.internal(h.create))
	mux.HandleFunc("PUT /products/{id}", h.internal(h.update))
	mux.HandleFunc("DELETE /products/{id}", h.internal(h.delete))
	mux.HandleFunc("POST /products/image", h.internal(h.uploadImage))
}

// internal enforces the shared token only when it is configured.
// Endpoints are reachable only inside the Docker network (expose-only),
// so an unset token (dev) means "no enforcement", matching other services.
func (h *Handler) internal(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.internalToken != "" && r.Header.Get("X-Internal-Token") != h.internalToken {
			writeError(w, 401, "unauthorized")
			return
		}
		next(w, r)
	}
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) listInStock(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListInStock()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, list)
}

func (h *Handler) listAll(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListAll()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, list)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var p models.Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	id, err := h.svc.Create(&p)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"id": id, "status": "ok"})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	var p models.Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	if err := h.svc.Update(id, &p); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	if err := h.svc.Delete(id); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) uploadImage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxImage); err != nil {
		writeError(w, 400, "fayl juda katta yoki noto'g'ri")
		return
	}
	file, hdr, err := r.FormFile("image")
	if err != nil {
		writeError(w, 400, "rasm fayli topilmadi")
		return
	}
	defer file.Close()
	if hdr.Size > maxImage {
		writeError(w, 400, "rasm 12MB dan kichik bo'lsin")
		return
	}
	url, err := h.svc.SaveImage(file, hdr.Filename, maxImage)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"url": url})
}

func (h *Handler) serveImage(w http.ResponseWriter, r *http.Request) {
	path, ok := h.svc.ImagePath(r.PathValue("file"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, path)
}

// ── helpers ──

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
