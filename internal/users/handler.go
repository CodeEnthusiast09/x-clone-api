package users

import (
	"errors"
	"log"
	"net/http"

	"github.com/CodeEnthusiast09/x-clone-api/internal/common"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) GetByUsername(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		common.Error(c, http.StatusBadRequest, "username is required")
		return
	}

	user, err := h.svc.GetByUsername(username)
	if errors.Is(err, ErrUserNotFound) {
		common.Error(c, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		log.Printf("users.GetByUsername: %v", err)
		common.Error(c, http.StatusInternalServerError, "failed to fetch user")
		return
	}

	common.Success(c, http.StatusOK, "user fetched", user)
}
