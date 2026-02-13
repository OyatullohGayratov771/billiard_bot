package player

import "database/sql"

type Service struct {
	repo *Repository
}

func NewService(r *Repository) *Service {
	return &Service{repo: r}
}

func (s *Service) GetOrCreate(tgID int64, name string) (*Player, error) {
	p, err := s.repo.GetByTelegramID(tgID)
	if err == nil {
		return p, nil
	}

	if err != sql.ErrNoRows {
		return nil, err
	}

	if err := s.repo.Create(tgID, name); err != nil {
		return nil, err
	}

	return s.repo.GetByTelegramID(tgID)
}
