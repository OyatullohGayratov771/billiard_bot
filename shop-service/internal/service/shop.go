package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"shop-service/internal/models"
	"shop-service/internal/repository"
)

var allowedExt = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true}

type Service struct {
	repo       *repository.Repo
	uploadsDir string
}

func New(repo *repository.Repo, uploadsDir string) *Service {
	if uploadsDir == "" {
		uploadsDir = "uploads"
	}
	_ = os.MkdirAll(uploadsDir, 0o755)
	return &Service{repo: repo, uploadsDir: uploadsDir}
}

func (s *Service) ListInStock() ([]*models.Product, error) { return s.repo.ListInStock() }
func (s *Service) ListAll() ([]*models.Product, error)     { return s.repo.ListAll() }

func (s *Service) Create(p *models.Product) (int64, error) {
	if err := normalize(p); err != nil {
		return 0, err
	}
	return s.repo.Create(p)
}

func (s *Service) Update(id int64, p *models.Product) error {
	if err := normalize(p); err != nil {
		return err
	}
	old := s.repo.ImageURL(id)
	ok, err := s.repo.Update(id, p)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("mahsulot topilmadi")
	}
	if old != "" && old != p.ImageURL {
		s.deleteImage(old)
	}
	return nil
}

func (s *Service) Delete(id int64) error {
	img := s.repo.ImageURL(id)
	if err := s.repo.Delete(id); err != nil {
		return err
	}
	s.deleteImage(img)
	return nil
}

func normalize(p *models.Product) error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return errors.New("nomi bo'sh bo'lmasin")
	}
	if p.Emoji == "" {
		p.Emoji = "🎱"
	}
	if p.Price < 0 {
		p.Price = 0
	}
	return nil
}

// ── Image storage ──

// SaveImage stores raw bytes and returns the public URL (/uploads/<name>).
func (s *Service) SaveImage(src io.Reader, origName string, max int64) (string, error) {
	ext := strings.ToLower(filepath.Ext(origName))
	if !allowedExt[ext] {
		return "", errors.New("faqat rasm (jpg, png, webp, gif)")
	}
	rb := make([]byte, 16)
	if _, err := rand.Read(rb); err != nil {
		return "", err
	}
	name := hex.EncodeToString(rb) + ext
	dst, err := os.Create(filepath.Join(s.uploadsDir, name))
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, io.LimitReader(src, max)); err != nil {
		return "", err
	}
	return "/uploads/" + name, nil
}

// ImagePath resolves an upload filename to a safe absolute path (no traversal).
func (s *Service) ImagePath(file string) (string, bool) {
	name := filepath.Base(file)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return "", false
	}
	if !allowedExt[strings.ToLower(filepath.Ext(name))] {
		return "", false
	}
	return filepath.Join(s.uploadsDir, name), true
}

func (s *Service) deleteImage(imageURL string) {
	if !strings.HasPrefix(imageURL, "/uploads/") {
		return
	}
	name := filepath.Base(strings.TrimPrefix(imageURL, "/uploads/"))
	if name == "" || name == "." || name == ".." {
		return
	}
	_ = os.Remove(filepath.Join(s.uploadsDir, name))
}
