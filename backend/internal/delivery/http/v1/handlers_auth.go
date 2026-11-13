package v1

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ganfay/split-core/internal/domain"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// CreateSession
//
// @Summary      Create session
// @Description  Returns UUID.
// @Tags         sessions
// @Accept       json
// @Produce      json
// @Success      200 {string} domain.Session true
// @Failure      500
// @Router       /api/v1/auth/telegram/init [post]
func (s *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	u := uuid.New()
	ctx, cancel := requestContext(r)
	defer cancel()

	session := domain.Session{
		Uuid:   u.String(),
		Status: "pending",
	}
	err := s.stateUC.Session(ctx, session)
	if err != nil {
		slog.Error("Failed to create session ", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	slog.Debug("Success create session ", "uuid", session.Uuid)
	writeJSON(w, http.StatusOK, session)
}

// CheckSession
//
// @Summary      Check session status
// @Description  Returns the current status of a session by UUID.
// @Tags         sessions
// @Accept       json
// @Produce      json
// @Param        request body domain.SessionCheckRequest true "Session UUID"
// @Success      200 {string} string "pending"
// @Failure      400
// @Failure      500
// @Router       /api/v1/auth/telegram/status [get]
// @Router       /api/v1/auth/telegram/status [post]
func (s *Handler) CheckSession(w http.ResponseWriter, r *http.Request) {
	var req domain.SessionCheckRequest
	if !decodeJSON(w, r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req.Uuid = strings.TrimSpace(req.Uuid)
	if req.Uuid == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()

	session, err := s.stateUC.GetSessionCtx(ctx, req.Uuid)
	if err != nil {
		slog.Error("[HANDLER CHECKSESSION] Failed to get session ", "err", err)
		if errors.Is(err, redis.Nil) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	slog.Debug("Success check session ", "uuid", session.Uuid)
	writeJSON(w, http.StatusOK, session.Status)
}

// GenerateAllJWTTokens
//
// @Summary      generate refresh, access jwt tokens
// @Tags         sessions
// @Accept       json
// @Produce      json
// @Param        request body domain.SessionCheckRequest true "Session UUID"
// @Success      200 {string} domain.Tokens
// @Failure      400
// @Failure      500
// @Router       /api/v1/auth/telegram/tokens [post]
func (s *Handler) GenerateAllJWTTokens(w http.ResponseWriter, r *http.Request) {
	var req domain.SessionCheckRequest
	if !decodeJSON(w, r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req.Uuid = strings.TrimSpace(req.Uuid)
	if req.Uuid == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()

	session, err := s.stateUC.GetSessionCtx(ctx, req.Uuid)
	if err != nil {
		slog.Error("Error getting session ", "err", err)
		if errors.Is(err, redis.Nil) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if session.Status != "authenticated" {
		slog.Error("[HANDLER JWT] Session is not authenticated")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	refreshJWT, err := GenerateRefreshJWT(session.IID)
	if err != nil {
		slog.Error("[HANDLER JWT] Failed to generate refresh JWT", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	accessJWT, err := GenerateAccessJWT(session.IID)
	if err != nil {
		slog.Error("[HANDLER JWT] Failed to generate access JWT", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp := domain.Tokens{
		AccessToken: accessJWT,
	}
	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	cookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshJWT,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
	writeJSON(w, http.StatusOK, resp)
}

// GetAccessToken
//
// @Summary      generate and get access token if refresh valid
// @Tags         sessions
// @Accept       json
// @Produce      json
// @Success      200 {string} domain.Tokens
// @Failure      400
// @Failure      500
// @Router       /api/v1/auth/telegram/access [post]
func (s *Handler) GetAccessToken(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		slog.Error("GetAccessToken: Failed to get cookie ", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	slog.Debug("[HANDLER] GetAccessToken")
	iid, err := ParseJWTRefresh(cookie.Value)
	if err != nil {
		slog.Error("GetAccessToken: Failed to parse jwt refresh ", "err", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	jwt, err := GenerateAccessJWT(iid)
	if err != nil {
		slog.Error("GetAccessToken: Failed to generate access JWT", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	resp := domain.Tokens{
		AccessToken: jwt,
	}
	writeJSON(w, http.StatusOK, resp)
}
