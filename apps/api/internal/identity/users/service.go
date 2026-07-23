package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/store"
)

var (
	ErrIdentityConflict = errors.New("verified email belongs to a different identity")
	ErrConsentRequired  = errors.New("current terms and privacy consent are required")
	ErrGuestConflict    = errors.New("guest session was migrated to a different user")
)

type GoogleProfile struct {
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
	AvatarURL     string
}

type ProvisionInput struct {
	Profile         GoogleProfile
	Locale          string
	TermsAccepted   bool
	PrivacyAccepted bool
	IPHash          []byte
	GuestID         *uuid.UUID
	RequestID       string
}

type Service struct {
	database        *database.Pool
	ids             ids.Source
	clock           clock.Clock
	termsVersion    string
	privacyVersion  string
	bootstrapAdmins map[string]struct{}
}

func New(
	pool *database.Pool,
	idSource ids.Source,
	timeSource clock.Clock,
	termsVersion, privacyVersion string,
	bootstrapAdmins map[string]struct{},
) *Service {
	return &Service{
		database:        pool,
		ids:             idSource,
		clock:           timeSource,
		termsVersion:    termsVersion,
		privacyVersion:  privacyVersion,
		bootstrapAdmins: bootstrapAdmins,
	}
}

func (service *Service) ProvisionGoogle(
	ctx context.Context,
	input ProvisionInput,
) (store.User, bool, error) {
	return service.provisionGoogle(ctx, input, 3)
}

func (service *Service) provisionGoogle(
	ctx context.Context,
	input ProvisionInput,
	attempts int,
) (store.User, bool, error) {
	if !input.Profile.EmailVerified || input.Profile.Subject == "" || input.Profile.Email == "" {
		return store.User{}, false, ErrIdentityConflict
	}
	if !input.TermsAccepted || !input.PrivacyAccepted {
		return store.User{}, false, ErrConsentRequired
	}
	if input.Locale != "es" {
		input.Locale = "en"
	}
	var result store.User
	var created bool
	err := service.database.WithinTransaction(ctx, pgx.TxOptions{
		IsoLevel: pgx.Serializable,
	}, func(queries *store.Queries) error {
		existing, err := queries.FindUserByGoogleSubject(ctx, input.Profile.Subject)
		if err == nil {
			result = existing.User
			return service.migrateGuest(ctx, queries, input.GuestID, result.ID)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("find Google identity: %w", err)
		}
		if _, err := queries.FindUserByEmail(ctx, input.Profile.Email); err == nil {
			return ErrIdentityConflict
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("find user by email: %w", err)
		}

		userID, err := service.ids.New()
		if err != nil {
			return err
		}
		identityID, err := service.ids.New()
		if err != nil {
			return err
		}
		displayName := strings.TrimSpace(input.Profile.DisplayName)
		if displayName == "" {
			displayName = strings.Split(input.Profile.Email, "@")[0]
		}
		result, err = queries.CreateUser(ctx, store.CreateUserParams{
			ID:          userID,
			Email:       strings.ToLower(input.Profile.Email),
			DisplayName: displayName,
			AvatarUrl:   nullableString(input.Profile.AvatarURL),
			Locale:      input.Locale,
			Role:        "user",
		})
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		if err := queries.CreateGoogleIdentity(ctx, store.CreateGoogleIdentityParams{
			ID:              identityID,
			UserID:          userID,
			ProviderSubject: input.Profile.Subject,
			VerifiedEmail:   strings.ToLower(input.Profile.Email),
		}); err != nil {
			return fmt.Errorf("create Google identity: %w", err)
		}
		if len(input.IPHash) != 0 && len(input.IPHash) != sha256Size {
			return errors.New("consent IP hash must be 32 bytes")
		}
		if err := queries.RecordTermsAcceptance(ctx, store.RecordTermsAcceptanceParams{
			UserID:         userID,
			TermsVersion:   service.termsVersion,
			PrivacyVersion: service.privacyVersion,
			IpHash:         input.IPHash,
		}); err != nil {
			return fmt.Errorf("record legal acceptance: %w", err)
		}
		created = true

		if _, ok := service.bootstrapAdmins[strings.ToLower(input.Profile.Email)]; ok {
			now := timestamp(service.clock.Now())
			updated, err := queries.PromoteBootstrapAdmin(ctx, store.PromoteBootstrapAdminParams{
				ID: userID, UpdatedAt: now,
			})
			if err != nil {
				return fmt.Errorf("bootstrap administrator: %w", err)
			}
			if updated == 1 {
				auditID, err := service.ids.New()
				if err != nil {
					return err
				}
				after, _ := json.Marshal(map[string]string{"role": "admin"})
				if err := queries.InsertAdminAudit(ctx, store.InsertAdminAuditParams{
					ID:          auditID,
					ActorUserID: &userID,
					Action:      "user.bootstrap_admin",
					TargetType:  "user",
					TargetID:    userID.String(),
					AfterValue:  after,
					RequestID:   nullableString(input.RequestID),
				}); err != nil {
					return fmt.Errorf("audit administrator bootstrap: %w", err)
				}
				result.Role = "admin"
			}
		}
		return service.migrateGuest(ctx, queries, input.GuestID, userID)
	})
	if err == nil {
		return result, created, nil
	}
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		return store.User{}, false, err
	}
	if (postgresError.Code == "40001" || postgresError.Code == "40P01") && attempts > 1 {
		timer := time.NewTimer(time.Duration(4-attempts) * 10 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return store.User{}, false, ctx.Err()
		case <-timer.C:
			return service.provisionGoogle(ctx, input, attempts-1)
		}
	}
	if postgresError.Code != "23505" {
		return store.User{}, false, err
	}
	existing, lookupErr := service.database.Queries().FindUserByGoogleSubject(
		ctx,
		input.Profile.Subject,
	)
	if lookupErr != nil {
		return store.User{}, false, ErrIdentityConflict
	}
	migrationErr := service.database.WithinTransaction(
		ctx,
		pgx.TxOptions{IsoLevel: pgx.Serializable},
		func(queries *store.Queries) error {
			return service.migrateGuest(ctx, queries, input.GuestID, existing.User.ID)
		},
	)
	return existing.User, false, migrationErr
}

func (service *Service) migrateGuest(
	ctx context.Context,
	queries *store.Queries,
	guestID *uuid.UUID,
	userID uuid.UUID,
) error {
	if guestID == nil {
		return nil
	}
	guest, err := queries.LockGuestSession(ctx, *guestID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lock guest migration: %w", err)
	}
	if guest.MigratedUserID != nil {
		if *guest.MigratedUserID == userID {
			return nil
		}
		return ErrGuestConflict
	}
	now := timestamp(service.clock.Now())
	if _, err := queries.MigrateGuestConversations(ctx, store.MigrateGuestConversationsParams{
		GuestSessionID: &guest.ID,
		UserID:         &userID,
		UpdatedAt:      now,
	}); err != nil {
		return fmt.Errorf("migrate guest conversations: %w", err)
	}
	updated, err := queries.MarkGuestMigrated(ctx, store.MarkGuestMigratedParams{
		ID:             guest.ID,
		MigratedUserID: &userID,
		MigratedAt:     now,
	})
	if err != nil {
		return fmt.Errorf("mark guest migrated: %w", err)
	}
	if updated != 1 {
		return ErrGuestConflict
	}
	return nil
}

const sha256Size = 32

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
