package tokens

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
)

type Claims struct {
	SessionID    string `json:"sid"`
	TokenVersion int32  `json:"ver"`
	jwt.RegisteredClaims
}

type KeyRing struct {
	mutex       sync.RWMutex
	activeKeyID string
	privateKey  ed25519.PrivateKey
	publicKeys  map[string]ed25519.PublicKey
	issuer      string
	audience    string
	lifetime    time.Duration
	clock       clock.Clock
}

func Load(cfg config.Auth, timeSource clock.Clock) (*KeyRing, error) {
	if cfg.PrivateKeyPath == "" {
		return NewEphemeral(cfg, timeSource)
	}
	content, err := os.ReadFile(cfg.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read JWT private key: %w", err)
	}
	block, _ := pem.Decode(content)
	if block == nil {
		return nil, errors.New("parse JWT private key: PEM block is required")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("parse JWT private key: PKCS8 Ed25519 key is required")
	}
	privateKey, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("parse JWT private key: Ed25519 key is required")
	}
	return newKeyRing(cfg, timeSource, privateKey), nil
}

func NewEphemeral(cfg config.Auth, timeSource clock.Clock) (*KeyRing, error) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ephemeral JWT key: %w", err)
	}
	return newKeyRing(cfg, timeSource, privateKey), nil
}

func newKeyRing(cfg config.Auth, timeSource clock.Clock, privateKey ed25519.PrivateKey) *KeyRing {
	publicKey := privateKey.Public().(ed25519.PublicKey)
	return &KeyRing{
		activeKeyID: cfg.ActiveKeyID,
		privateKey:  privateKey,
		publicKeys:  map[string]ed25519.PublicKey{cfg.ActiveKeyID: publicKey},
		issuer:      cfg.Issuer,
		audience:    cfg.Audience,
		lifetime:    cfg.AccessTokenTTL,
		clock:       timeSource,
	}
}

func (ring *KeyRing) Sign(userID, sessionID uuid.UUID, tokenVersion int32) (string, error) {
	now := ring.clock.Now()
	claims := Claims{
		SessionID:    sessionID.String(),
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    ring.issuer,
			Subject:   userID.String(),
			Audience:  jwt.ClaimStrings{ring.audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(ring.lifetime)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = ring.activeKeyID
	signed, err := token.SignedString(ring.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return signed, nil
}

func (ring *KeyRing) Verify(raw string) (Claims, error) {
	claims := Claims{}
	token, err := jwt.ParseWithClaims(
		raw,
		&claims,
		func(token *jwt.Token) (any, error) {
			keyID, ok := token.Header["kid"].(string)
			if !ok || keyID == "" {
				return nil, errors.New("JWT kid is required")
			}
			ring.mutex.RLock()
			defer ring.mutex.RUnlock()
			key, ok := ring.publicKeys[keyID]
			if !ok {
				return nil, errors.New("JWT kid is unknown or disabled")
			}
			return key, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(ring.issuer),
		jwt.WithAudience(ring.audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(30*time.Second),
		jwt.WithStrictDecoding(),
		jwt.WithTimeFunc(ring.clock.Now),
	)
	if err != nil || !token.Valid {
		return Claims{}, fmt.Errorf("verify access token: %w", err)
	}
	if claims.Subject == "" || claims.SessionID == "" || claims.TokenVersion <= 0 ||
		claims.NotBefore == nil || claims.IssuedAt == nil {
		return Claims{}, errors.New("verify access token: required claims are missing")
	}
	if _, err := uuid.Parse(claims.Subject); err != nil {
		return Claims{}, errors.New("verify access token: subject is invalid")
	}
	if _, err := uuid.Parse(claims.SessionID); err != nil {
		return Claims{}, errors.New("verify access token: session is invalid")
	}
	return claims, nil
}

func (ring *KeyRing) AddVerificationKey(keyID string, publicKey ed25519.PublicKey) {
	ring.mutex.Lock()
	defer ring.mutex.Unlock()
	ring.publicKeys[keyID] = publicKey
}

func (ring *KeyRing) RemoveVerificationKey(keyID string) {
	ring.mutex.Lock()
	defer ring.mutex.Unlock()
	if keyID != ring.activeKeyID {
		delete(ring.publicKeys, keyID)
	}
}
