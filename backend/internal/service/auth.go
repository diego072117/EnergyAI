package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"energyai/internal/domain"
)

type Auth struct {
	repo      Repository
	secret    []byte
	ttl       time.Duration
	now       func() time.Time
	dummyHash []byte // compared for unknown users so timing does not leak which emails exist
}

func NewAuth(repo Repository, secret string, ttl time.Duration) *Auth {
	dummy, _ := bcrypt.GenerateFromPassword([]byte("energyai-dummy-password"), bcrypt.DefaultCost)
	return &Auth{repo: repo, secret: []byte(secret), ttl: ttl, now: time.Now, dummyHash: dummy}
}

type Claims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	jwt.RegisteredClaims
}

type LoginResult struct {
	Token     string      `json:"token"`
	ExpiresAt time.Time   `json:"expires_at"`
	User      domain.User `json:"user"`
}

func (a *Auth) EnsureUser(ctx context.Context, email, name, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return a.repo.UpsertUser(ctx, strings.ToLower(strings.TrimSpace(email)), name, string(hash))
}

func (a *Auth) Login(ctx context.Context, email, password string) (LoginResult, error) {
	u, err := a.repo.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if errors.Is(err, ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(a.dummyHash, []byte(password))
		return LoginResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResult{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	now := a.now()
	exp := now.Add(a.ttl)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Email: u.Email,
		Name:  u.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.Email,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			Issuer:    "energyai",
		},
	}).SignedString(a.secret)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: token, ExpiresAt: exp, User: u}, nil
}

func (a *Auth) Verify(token string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		return a.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer("energyai"),
		jwt.WithTimeFunc(a.now))
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	return claims, nil
}
