package v1

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/ganfay/split-core/internal/domain"
	"github.com/google/uuid"
)

func (s *Handler) CreateSession(w http.ResponseWriter, _ *http.Request) {
	u := uuid.New()
	ctx := context.Background()
	var session domain.Session
	session.Uuid = u.String()
	session.Status = "pending"
	err := s.stateUC.Session(ctx, session)
	if err != nil {
		slog.Error("Failed to create session ", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var resp domain.Session
	resp.Uuid = u.String()
	resp.Status = "pending"
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(resp)
	slog.Debug("Success create session ", "uuid", resp.Uuid)
	if err != nil {
		slog.Error("[HANDLER CREATESESSION] Failed to encode json ", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *Handler) CheckSession(w http.ResponseWriter, r *http.Request) {
	var req domain.SessionCheckRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		slog.Error("[HANDLER CHECKSESSION] Failed to decode json ", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ctx := context.Background()

	session, err := s.stateUC.GetSessionCtx(ctx, req.Uuid)
	if err != nil {
		slog.Error("[HANDLER CHECKSESSION] Failed to get session ", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	slog.Debug("Success check session ", "uuid", session.Uuid)
	err = json.NewEncoder(w).Encode(session.Status)
	if err != nil {
		slog.Error("[HANDLER CHECKSESSION] Failed to encode json ", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *Handler) GenerateAllJWTTokens(w http.ResponseWriter, r *http.Request) {
	var req domain.SessionCheckRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		slog.Error("Error decoding json ", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ctx := context.Background()
	session, err := s.stateUC.GetSessionCtx(ctx, req.Uuid)
	if err != nil {
		slog.Error("Error getting session ", "err", err)
		w.WriteHeader(http.StatusBadRequest)
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
	cookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshJWT,
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // HTTPS only
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, cookie)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		slog.Error("[HANDLER JWT] Failed to encode json ", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *Handler) GetAccessToken(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		slog.Error("GetAccessToken: Failed to get cookie ", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	slog.Debug("[HANDLER] GetAccessToken", "cookie", cookie, "ctx", ctx)
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
	w.Header().Set("Content-Type", "application/json")
	resp := domain.Tokens{
		AccessToken: jwt,
	}
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		slog.Error("GetAccessToken: Failed to encode json ", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
