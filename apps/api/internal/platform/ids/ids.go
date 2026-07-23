package ids

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"sync"

	"github.com/google/uuid"
)

type Source interface {
	New() (uuid.UUID, error)
}

type UUIDv7 struct {
	random io.Reader
}

func NewUUIDv7() UUIDv7 {
	return UUIDv7{random: rand.Reader}
}

func (source UUIDv7) New() (uuid.UUID, error) {
	id, err := uuid.NewV7FromReader(source.random)
	if err != nil {
		return uuid.Nil, fmt.Errorf("generate UUIDv7: %w", err)
	}
	return id, nil
}

type Fake struct {
	mutex sync.Mutex
	ids   []uuid.UUID
}

func NewFake(values ...uuid.UUID) *Fake {
	return &Fake{ids: append([]uuid.UUID(nil), values...)}
}

func (source *Fake) New() (uuid.UUID, error) {
	source.mutex.Lock()
	defer source.mutex.Unlock()
	if len(source.ids) == 0 {
		return uuid.Nil, fmt.Errorf("fake ID source exhausted")
	}
	id := source.ids[0]
	source.ids = source.ids[1:]
	return id, nil
}

func SecureToken(bytes int) (string, error) {
	if bytes < 16 {
		return "", fmt.Errorf("secure token length must be at least 16 bytes")
	}
	buffer := make([]byte, bytes)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		return "", fmt.Errorf("generate secure token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
