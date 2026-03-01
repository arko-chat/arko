package service

import (
	"fmt"

	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/models"
	"github.com/arko-chat/arko/internal/ws"
)

type BaseService struct {
	matrix *matrix.Manager
	hub    *ws.Hub
}

func NewBaseService(
	mgr *matrix.Manager,
	hub *ws.Hub,
) *BaseService {
	return &BaseService{
		matrix: mgr,
		hub:    hub,
	}
}

func (s *BaseService) GetCurrentUser() (models.User, error) {
	currentSession := s.matrix.GetCurrentMatrixSession()
	if currentSession == nil {
		return models.User{}, fmt.Errorf("missing matrix session")
	}
	return currentSession.GetCurrentUser()
}

func (s *BaseService) GetCurrentUserID() string {
	return s.matrix.GetCurrentUserID()
}
