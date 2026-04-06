package rbac

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) HasPermission(userID uuid.UUID, resource, action string) (bool, error) {
	var count int64
	err := s.db.Model(&Permission{}).
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Joins("JOIN user_roles ON user_roles.role_id = role_permissions.role_id").
		Where("user_roles.user_id = ? AND permissions.resource = ? AND permissions.action = ?",
			userID, resource, action).
		Count(&count).Error
	return count > 0, err
}

func (s *Service) GetUserPermissions(userID uuid.UUID) ([]Permission, error) {
	var perms []Permission
	err := s.db.Distinct("permissions.*").
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Joins("JOIN user_roles ON user_roles.role_id = role_permissions.role_id").
		Where("user_roles.user_id = ?", userID).
		Find(&perms).Error
	return perms, err
}

func (s *Service) FindUserByKeycloakID(kcID string) (*User, error) {
	var u User
	if err := s.db.Preload("Roles").Where("keycloak_id = ?", kcID).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Service) EnsureUser(kcID, email, name string) (*User, error) {
	var u User
	result := s.db.Where("keycloak_id = ?", kcID).First(&u)
	if result.Error == nil {
		return &u, nil
	}
	u = User{KeycloakID: kcID, Email: email, DisplayName: name, Enabled: true}
	if err := s.db.Create(&u).Error; err != nil {
		return nil, err
	}
	var viewer Role
	if s.db.Where("name = ?", "viewer").First(&viewer).Error == nil {
		s.db.Model(&u).Association("Roles").Append(&viewer)
	}
	return &u, nil
}

func (s *Service) CreateRole(name, description string) (*Role, error) {
	role := &Role{Name: name, Description: description, IsSystem: false}
	if err := s.db.Create(role).Error; err != nil {
		return nil, err
	}
	return role, nil
}

func (s *Service) DeleteRole(id uuid.UUID) error {
	var role Role
	if err := s.db.First(&role, id).Error; err != nil {
		return err
	}
	if role.IsSystem {
		return fmt.Errorf("cannot delete system role %q", role.Name)
	}
	s.db.Model(&role).Association("Permissions").Clear()
	return s.db.Delete(&role).Error
}

func (s *Service) SetRolePermissions(roleID uuid.UUID, permIDs []uuid.UUID) error {
	var perms []Permission
	s.db.Where("id IN ?", permIDs).Find(&perms)
	var role Role
	if err := s.db.First(&role, roleID).Error; err != nil {
		return err
	}
	return s.db.Model(&role).Association("Permissions").Replace(perms)
}

func (s *Service) CreatePermission(resource, action, desc string) (*Permission, error) {
	p := &Permission{Resource: resource, Action: action, Description: desc, IsSystem: false}
	if err := s.db.Create(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) DeletePermission(id uuid.UUID) error {
	var p Permission
	if err := s.db.First(&p, id).Error; err != nil {
		return err
	}
	if p.IsSystem {
		return fmt.Errorf("cannot delete system permission %s:%s", p.Resource, p.Action)
	}
	return s.db.Delete(&p).Error
}
