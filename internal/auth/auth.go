package auth

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// User represents a registered user.
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Password string `json:"-"` // stored as-is for now; hash in production
}

// Store is an in-memory user store (replaced with DB in Phase 3).
type Store struct {
	mu    sync.RWMutex
	users map[string]*User // username -> user
	byID  map[string]*User // id -> user
}

// NewStore creates an empty user store.
func NewStore() *Store {
	return &Store{
		users: make(map[string]*User),
		byID:  make(map[string]*User),
	}
}

// Register creates a new user.
func (s *Store) Register(username, password string) (*User, error) {
	if username == "" || password == "" {
		return nil, errors.New("username and password required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[username]; exists {
		return nil, fmt.Errorf("username %q already taken", username)
	}
	u := &User{
		ID:       uuid.New().String(),
		Username: username,
		Password: password,
	}
	s.users[username] = u
	s.byID[u.ID] = u
	return u, nil
}

// Authenticate checks credentials and returns the user.
func (s *Store) Authenticate(username, password string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[username]
	if !ok || u.Password != password {
		return nil, errors.New("invalid credentials")
	}
	return u, nil
}

// GetByID returns a user by ID.
func (s *Store) GetByID(id string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.byID[id]
	if !ok {
		return nil, fmt.Errorf("user %s not found", id)
	}
	return u, nil
}

// JWTService handles token creation and validation.
type JWTService struct {
	secret []byte
	expiry time.Duration
}

// NewJWTService creates a new JWT service.
func NewJWTService(secret string, expiry time.Duration) *JWTService {
	return &JWTService{
		secret: []byte(secret),
		expiry: expiry,
	}
}

// Claims are the JWT claims.
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// GenerateToken creates a JWT for the given user.
func (j *JWTService) GenerateToken(user *User) (string, error) {
	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secret)
}

// ValidateToken parses and validates a JWT, returning the claims.
func (j *JWTService) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
