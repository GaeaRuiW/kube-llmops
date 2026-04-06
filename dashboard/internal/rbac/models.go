package rbac

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	KeycloakID  string     `gorm:"uniqueIndex" json:"keycloakId"`
	Email       string     `gorm:"uniqueIndex" json:"email"`
	DisplayName string     `json:"displayName"`
	Avatar      string     `json:"avatar,omitempty"`
	Enabled     bool       `gorm:"default:true" json:"enabled"`
	LastLogin   *time.Time `json:"lastLogin,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	Roles       []Role     `gorm:"many2many:user_roles" json:"roles,omitempty"`
}

type Role struct {
	ID          uuid.UUID    `gorm:"type:uuid;primaryKey" json:"id"`
	Name        string       `gorm:"uniqueIndex" json:"name"`
	Description string       `json:"description"`
	IsSystem    bool         `gorm:"default:false" json:"isSystem"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
	Permissions []Permission `gorm:"many2many:role_permissions" json:"permissions,omitempty"`
}

type Permission struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Resource    string    `gorm:"uniqueIndex:idx_resource_action" json:"resource"`
	Action      string    `gorm:"uniqueIndex:idx_resource_action" json:"action"`
	Description string    `json:"description"`
	IsSystem    bool      `gorm:"default:false" json:"isSystem"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

func (r *Role) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

func (p *Permission) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}
