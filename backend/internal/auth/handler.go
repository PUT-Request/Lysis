package auth

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"lysis/internal/config"
	"lysis/internal/db"
)

type Handler struct {
	DB     *sql.DB
	Config *config.Config
}

func NewHandler(database *sql.DB, cfg *config.Config) *Handler {
	return &Handler{DB: database, Config: cfg}
}

type signupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string  `json:"token"`
	User  *db.User `json:"user"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		writeError(w, "valid email required", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		writeError(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	existing, err := db.GetUserByEmail(h.DB, req.Email)
	if err != nil {
		logError("signup lookup", err)
	}
	if existing != nil || err != nil {
		writeJSON(w, http.StatusConflict, errorResponse{Error: "email already registered"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), h.Config.Auth.PasswordBcryptCost)
	if err != nil {
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	user, err := db.InsertUser(h.DB, req.Email, string(hash))
	if err != nil {
		writeError(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	ttl, _ := time.ParseDuration(h.Config.Auth.AccessTTL)
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	token, err := GenerateToken(user.ID, user.Email, h.Config.Auth.JWTSecret, ttl)
	if err != nil {
		writeError(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, authResponse{Token: token, User: user})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	user, err := db.GetUserByEmail(h.DB, req.Email)
	if err != nil {
		writeError(w, "database error", http.StatusInternalServerError)
		return
	}
	if user == nil {
		writeError(w, "invalid email or password", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		writeError(w, "invalid email or password", http.StatusUnauthorized)
		return
	}

	ttl, _ := time.ParseDuration(h.Config.Auth.AccessTTL)
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	token, err := GenerateToken(user.ID, user.Email, h.Config.Auth.JWTSecret, ttl)
	if err != nil {
		writeError(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, authResponse{Token: token, User: user})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	uc := GetUser(r)
	if uc == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := db.GetUserByID(h.DB, uc.UserID)
	if err != nil || user == nil {
		writeError(w, "user not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"user": user})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorResponse{Error: msg})
}

func logError(context string, err error) {
	if err != nil {
		log.Printf("[auth] %s: %v", context, err)
	}
}
