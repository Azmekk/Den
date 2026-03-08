package handler

import (
	"net/http"

	"github.com/martinmckenna/den/internal/httputil"
	"github.com/martinmckenna/den/internal/service"
)

type UserHandler struct {
	svc *service.UserService
}

func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.List(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, users)
}
