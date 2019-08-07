package bot

import (
	"github.com/jinzhu/gorm"
)

type RSSLink struct {
	gorm.Model
	Url     string `gorm:"unique;not null"`
	AddedBy int    `gorm:"not null"`
}

func (r *RSSLink) String() string {
	return "RSSLink"
}

type HNRegistration struct {
	gorm.Model
	UserID   int `gorm:"unique;not null"`
	MinScore int `gorm:"default:0"`
}

func (h *HNRegistration) String() string {
	return "HNRegistration"
}

type RSSRegistration struct {
	RSSLinkID uint `gorm:"primary_key"`
	UserID    int  `gorm:"primary_key"`
	Alias     string
}

func (r *RSSRegistration) String() string {
	return "RSSRegistration"
}
