package service

import (
	"fmt"

	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/models"
	"github.com/arko-chat/arko/internal/ws"
)

type BaseService struct {
	matrix matrix.ManagerClient
	hub    *ws.Hub
}

func NewBaseService(
	mgr matrix.ManagerClient,
	hub *ws.Hub,
) *BaseService {
	return &BaseService{
		matrix: mgr,
		hub:    hub,
	}
}

func (s *BaseService) GetCurrentSession() (matrix.SessionClient, error) {
	session := s.matrix.GetCurrentMatrixSession()
	if session == nil {
		return nil, fmt.Errorf("missing matrix session")
	}
	return session, nil
}

func (s *BaseService) GetCurrentUser() (models.User, error) {
	session, err := s.GetCurrentSession()
	if err != nil {
		return models.User{}, err
	}
	return session.GetCurrentUser()
}

func (s *BaseService) GetCurrentUserID() string {
	return s.matrix.GetCurrentUserID()
}
