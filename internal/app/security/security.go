package security

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

const (
	RoleCreator   = "creator"
	RoleModerator = "moderator"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrSessionGone  = errors.New("session not found")
)

type Config struct {
	JWTSecret   string
	TokenTTL    time.Duration
	RedisAddr   string
	RedisPass   string
	RedisDB     int
	KeyPrefix   string
	RedisEnable bool
}

type Principal struct {
	UserID    uint
	Login     string
	Role      string
	SessionID string
}

type Session struct {
	ID        string    `json:"id"`
	UserID    uint      `json:"user_id"`
	Login     string    `json:"login"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type Service struct {
	secret    []byte
	tokenTTL  time.Duration
	keyPrefix string
	redis     *redis.Client
}

type Claims struct {
	SessionID string `json:"sid"`
	Login     string `json:"login"`
	Role      string `json:"role"`
	UserID    uint   `json:"uid"`
	jwt.RegisteredClaims
}

func NewService(cfg Config) (*Service, error) {
	secret := strings.TrimSpace(cfg.JWTSecret)
	if secret == "" {
		return nil, errors.New("jwt secret is empty")
	}

	ttl := cfg.TokenTTL
	if ttl <= 0 {
		ttl = 2 * time.Hour
	}

	keyPrefix := strings.TrimSpace(cfg.KeyPrefix)
	if keyPrefix == "" {
		keyPrefix = "rip:session:"
	}

	service := &Service{
		secret:    []byte(secret),
		tokenTTL:  ttl,
		keyPrefix: keyPrefix,
	}

	if cfg.RedisEnable {
		client := redis.NewClient(&redis.Options{
			Addr:         strings.TrimSpace(cfg.RedisAddr),
			Password:     cfg.RedisPass,
			DB:           cfg.RedisDB,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
			DialTimeout:  3 * time.Second,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := client.Ping(ctx).Err(); err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("redis unavailable: %w", err)
		}
		service.redis = client
	}

	return service, nil
}

func (s *Service) Close() error {
	if s.redis == nil {
		return nil
	}
	return s.redis.Close()
}

func (s *Service) Login(ctx context.Context, userID uint, login, role string) (token string, expiresAt time.Time, principal Principal, err error) {
	if userID == 0 || strings.TrimSpace(login) == "" || strings.TrimSpace(role) == "" {
		return "", time.Time{}, Principal{}, ErrInvalidToken
	}
	sessionID, err := generateSessionID()
	if err != nil {
		return "", time.Time{}, Principal{}, err
	}

	now := time.Now().UTC()
	expiresAt = now.Add(s.tokenTTL)
	principal = Principal{
		UserID:    userID,
		Login:     login,
		Role:      strings.TrimSpace(role),
		SessionID: sessionID,
	}

	if s.redis != nil {
		payload, err := json.Marshal(Session{
			ID:        sessionID,
			UserID:    userID,
			Login:     login,
			Role:      principal.Role,
			CreatedAt: now,
		})
		if err != nil {
			return "", time.Time{}, Principal{}, err
		}
		key := s.keyPrefix + sessionID
		if err := s.redis.Set(ctx, key, string(payload), s.tokenTTL).Err(); err != nil {
			return "", time.Time{}, Principal{}, err
		}
	}

	claims := Claims{
		SessionID: sessionID,
		Login:     login,
		Role:      principal.Role,
		UserID:    userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(uint64(userID), 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err = jwtToken.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, Principal{}, err
	}

	return token, expiresAt, principal, nil
}

func (s *Service) AuthenticateBearer(ctx context.Context, headerValue string) (Principal, error) {
	parts := strings.SplitN(strings.TrimSpace(headerValue), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return Principal{}, ErrInvalidToken
	}

	rawToken := strings.TrimSpace(parts[1])
	if rawToken == "" {
		return Principal{}, ErrInvalidToken
	}

	parsed, err := jwt.ParseWithClaims(rawToken, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.secret, nil
	})
	if err != nil {
		return Principal{}, ErrInvalidToken
	}

	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return Principal{}, ErrInvalidToken
	}

	principal := Principal{
		UserID:    claims.UserID,
		Login:     claims.Login,
		Role:      claims.Role,
		SessionID: claims.SessionID,
	}
	if principal.UserID == 0 || principal.SessionID == "" || principal.Role == "" {
		return Principal{}, ErrInvalidToken
	}

	if s.redis != nil {
		key := s.keyPrefix + principal.SessionID
		raw, err := s.redis.Get(ctx, key).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				return Principal{}, ErrSessionGone
			}
			return Principal{}, err
		}

		var session Session
		if err := json.Unmarshal([]byte(raw), &session); err != nil {
			return Principal{}, err
		}
		if session.UserID != principal.UserID || !strings.EqualFold(session.Role, principal.Role) {
			return Principal{}, ErrInvalidToken
		}
	}

	return principal, nil
}

func (s *Service) Logout(ctx context.Context, principal Principal) error {
	if s.redis == nil || strings.TrimSpace(principal.SessionID) == "" {
		return nil
	}
	key := s.keyPrefix + strings.TrimSpace(principal.SessionID)
	return s.redis.Del(ctx, key).Err()
}

func generateSessionID() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}
