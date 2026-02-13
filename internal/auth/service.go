package auth

import "database/sql"

type Service struct {
	repo *Repository
}

func NewService(r *Repository) *Service {
	return &Service{repo: r}
}

func (s *Service) IsAdmin(tgID int64) (bool, error) {
	_, err := s.repo.GetByTelegramID(tgID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}
