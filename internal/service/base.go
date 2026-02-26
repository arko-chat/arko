package service

import (
	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/models"
	"github.com/arko-chat/arko/internal/ws"
)

type BaseService struct {
	matrix *matrix.Manager
	hub    ws.WSHub
}

func NewBaseService(
	mgr *matrix.Manager,
	hub ws.WSHub,
) *BaseService {
	return &BaseService{
		matrix: mgr,
		hub:    hub,
	}
}

func (s *BaseService) Hub() ws.WSHub {
	return s.hub
}

func (s *BaseService) Matrix() *matrix.Manager {
	return s.matrix
}

func (s *BaseService) GetCurrentUser() (models.User, error) {
	return s.matrix.GetCurrentUser(s.matrix.GetCurrentUserID())
}

func (s *BaseService) GetCurrentUserID() string {
	return s.matrix.GetCurrentUserID()
}
