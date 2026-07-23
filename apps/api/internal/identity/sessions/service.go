package sessions

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/aikssen/glazz-chat/apps/api/internal/identity/tokens"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/store"
)

var (
	ErrInvalidRefresh = errors.New("refresh token is invalid or expired")
	ErrRefreshReuse   = errors.New("refresh token reuse detected")
	ErrSessionRevoked = errors.New("session is revoked")
)

type Service struct {
	database   *database.Pool
	ids        ids.Source
	clock      clock.Clock
	signer     *tokens.KeyRing
	refreshTTL time.Duration
}

type Credentials struct {
	AccessToken  string
	RefreshToken string
	SessionID    uuid.UUID
	UserID       uuid.UUID
}

func New(
	pool *database.Pool,
	idSource ids.Source,
	timeSource clock.Clock,
	signer *tokens.KeyRing,
	refreshTTL time.Duration,
) *Service {
	return &Service{
		database:   pool,
		ids:        idSource,
		clock:      timeSource,
		signer:     signer,
		refreshTTL: refreshTTL,
	}
}

func (service *Service) Create(
	ctx context.Context,
	userID uuid.UUID,
	deviceLabel string,
) (Credentials, error) {
	sessionID, err := service.ids.New()
	if err != nil {
		return Credentials{}, err
	}
	familyID, err := service.ids.New()
	if err != nil {
		return Credentials{}, err
	}
	refreshToken, err := ids.SecureToken(32)
	if err != nil {
		return Credentials{}, err
	}
	now := service.clock.Now()
	expiresAt := now.Add(service.refreshTTL)
	tokenHash := hashToken(refreshToken)
	var session store.AuthSession

	err = service.database.WithinTransaction(ctx, pgx.TxOptions{}, func(queries *store.Queries) error {
		created, err := queries.CreateAuthSession(ctx, store.CreateAuthSessionParams{
			ID:               sessionID,
			UserID:           userID,
			FamilyID:         familyID,
			DeviceLabel:      nullableString(deviceLabel),
			RefreshExpiresAt: timestamp(expiresAt),
			RecentAuthAt:     timestamp(now),
		})
		if err != nil {
			return fmt.Errorf("create auth session: %w", err)
		}
		session = created
		if err := queries.InsertRefreshToken(ctx, store.InsertRefreshTokenParams{
			TokenHash: tokenHash[:],
			SessionID: sessionID,
			ExpiresAt: timestamp(expiresAt),
		}); err != nil {
			return fmt.Errorf("insert refresh token: %w", err)
		}
		return nil
	})
	if err != nil {
		return Credentials{}, err
	}
	accessToken, err := service.signer.Sign(userID, sessionID, session.TokenVersion)
	if err != nil {
		return Credentials{}, err
	}
	return Credentials{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		SessionID:    sessionID,
		UserID:       userID,
	}, nil
}

func (service *Service) Rotate(ctx context.Context, rawToken string) (Credentials, error) {
	if rawToken == "" {
		return Credentials{}, ErrInvalidRefresh
	}
	nextToken, err := ids.SecureToken(32)
	if err != nil {
		return Credentials{}, err
	}
	currentHash := hashToken(rawToken)
	nextHash := hashToken(nextToken)
	now := service.clock.Now()
	var row store.LockRefreshTokenRow
	var semanticError error

	err = service.database.WithinTransaction(ctx, pgx.TxOptions{}, func(queries *store.Queries) error {
		locked, err := queries.LockRefreshToken(ctx, currentHash[:])
		if errors.Is(err, pgx.ErrNoRows) {
			semanticError = ErrInvalidRefresh
			return nil
		}
		if err != nil {
			return fmt.Errorf("lock refresh token: %w", err)
		}
		row = locked
		if locked.UsedAt.Valid {
			if err := queries.RevokeSessionFamily(ctx, store.RevokeSessionFamilyParams{
				FamilyID:  locked.FamilyID,
				RevokedAt: timestamp(now),
			}); err != nil {
				return fmt.Errorf("revoke reused refresh family: %w", err)
			}
			semanticError = ErrRefreshReuse
			return nil
		}
		if locked.RevokedAt.Valid || !locked.ExpiresAt.Valid || !locked.ExpiresAt.Time.After(now) ||
			!locked.RefreshExpiresAt.Valid || !locked.RefreshExpiresAt.Time.After(now) {
			semanticError = ErrInvalidRefresh
			return nil
		}
		updated, err := queries.MarkRefreshTokenUsed(ctx, store.MarkRefreshTokenUsedParams{
			TokenHash: currentHash[:],
			UsedAt:    timestamp(now),
		})
		if err != nil {
			return fmt.Errorf("mark refresh token used: %w", err)
		}
		if updated != 1 {
			return errors.New("refresh rotation lost token lock")
		}
		if err := queries.InsertRefreshToken(ctx, store.InsertRefreshTokenParams{
			TokenHash: nextHash[:],
			SessionID: locked.SessionID,
			ExpiresAt: locked.RefreshExpiresAt,
		}); err != nil {
			return fmt.Errorf("insert rotated refresh token: %w", err)
		}
		return queries.TouchAuthSession(ctx, store.TouchAuthSessionParams{
			ID:         locked.SessionID,
			LastSeenAt: timestamp(now),
		})
	})
	if err != nil {
		return Credentials{}, err
	}
	if semanticError != nil {
		return Credentials{}, semanticError
	}
	accessToken, err := service.signer.Sign(row.UserID, row.SessionID, row.TokenVersion)
	if err != nil {
		return Credentials{}, err
	}
	return Credentials{
		AccessToken:  accessToken,
		RefreshToken: nextToken,
		SessionID:    row.SessionID,
		UserID:       row.UserID,
	}, nil
}

func (service *Service) ValidateAccessSession(
	ctx context.Context,
	claims tokens.Claims,
) (uuid.UUID, uuid.UUID, error) {
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, uuid.Nil, ErrSessionRevoked
	}
	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil {
		return uuid.Nil, uuid.Nil, ErrSessionRevoked
	}
	session, err := service.database.Queries().GetAuthSession(ctx, sessionID)
	if err != nil || session.UserID != userID || session.RevokedAt.Valid ||
		!session.RefreshExpiresAt.Valid || !session.RefreshExpiresAt.Time.After(service.clock.Now()) ||
		session.TokenVersion != claims.TokenVersion {
		return uuid.Nil, uuid.Nil, ErrSessionRevoked
	}
	return userID, sessionID, nil
}

func (service *Service) List(ctx context.Context, userID uuid.UUID) ([]store.AuthSession, error) {
	return service.database.Queries().ListAuthSessions(ctx, store.ListAuthSessionsParams{
		UserID:           userID,
		RefreshExpiresAt: timestamp(service.clock.Now()),
	})
}

func (service *Service) Revoke(ctx context.Context, userID, sessionID uuid.UUID) (bool, error) {
	rows, err := service.database.Queries().RevokeAuthSession(ctx, store.RevokeAuthSessionParams{
		ID:        sessionID,
		UserID:    userID,
		RevokedAt: timestamp(service.clock.Now()),
	})
	return rows == 1, err
}

func hashToken(raw string) [32]byte {
	return sha256.Sum256([]byte(raw))
}

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
