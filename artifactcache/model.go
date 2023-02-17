package artifactcache

import (
	"fmt"
	"net/http"
	"time"
)

type Cache struct {
	ID       int64     `xorm:"pk" json:"-"`
	Key      string    `xorm:"TEXT index unique(key_version)" json:"key"`
	Version  string    `xorm:"TEXT unique(key_version)" json:"version"`
	Complete bool      `xorm:"index(complete_used_at)" json:"-"`
	UsedAt   time.Time `xorm:"index(complete_used_at) updated" json:"-"`
}

// Bind implements render.Binder
func (c *Cache) Bind(_ *http.Request) error {
	if c.Key == "" {
		return fmt.Errorf("missing key")
	}
	if c.Version == "" {
		return fmt.Errorf("missing version")
	}
	return nil
}
