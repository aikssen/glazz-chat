package clock

import (
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
}

type UTC struct{}

func (UTC) Now() time.Time {
	return time.Now().UTC()
}

type Fake struct {
	mutex sync.RWMutex
	now   time.Time
}

func NewFake(now time.Time) *Fake {
	return &Fake{now: now.UTC()}
}

func (f *Fake) Now() time.Time {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	return f.now
}

func (f *Fake) Advance(duration time.Duration) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.now = f.now.Add(duration)
}
