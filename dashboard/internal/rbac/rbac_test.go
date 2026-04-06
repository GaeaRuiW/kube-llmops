package rbac

import (
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &Role{}, &Permission{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// Task 4 tests
func TestUserModel_Fields(t *testing.T) {
	u := User{ID: uuid.New(), KeycloakID: "kc-123", Email: "test@example.com", DisplayName: "Test User", Enabled: true}
	if u.Email != "test@example.com" {
		t.Errorf("expected test@example.com, got %s", u.Email)
	}
}

func TestPermission_Fields(t *testing.T) {
	p := Permission{ID: uuid.New(), Resource: "models", Action: "create", IsSystem: true}
	if p.Resource != "models" || p.Action != "create" {
		t.Error("permission fields wrong")
	}
}

// Task 5 tests
func TestSeed_CreatesDefaultPermissions(t *testing.T) {
	db := setupTestDB(t)
	if err := Seed(db); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var count int64
	db.Model(&Permission{}).Count(&count)
	if count < 24 {
		t.Errorf("expected >= 24 permissions, got %d", count)
	}
}

func TestSeed_Creates3SystemRoles(t *testing.T) {
	db := setupTestDB(t)
	if err := Seed(db); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var roles []Role
	db.Where("is_system = ?", true).Find(&roles)
	if len(roles) != 3 {
		t.Errorf("expected 3 system roles, got %d", len(roles))
	}
	names := map[string]bool{}
	for _, r := range roles {
		names[r.Name] = true
	}
	for _, n := range []string{"admin", "editor", "viewer"} {
		if !names[n] {
			t.Errorf("missing system role: %s", n)
		}
	}
}

func TestSeed_AdminHasAllPermissions(t *testing.T) {
	db := setupTestDB(t)
	Seed(db)
	var admin Role
	db.Preload("Permissions").Where("name = ?", "admin").First(&admin)
	var totalPerms int64
	db.Model(&Permission{}).Count(&totalPerms)
	if len(admin.Permissions) != int(totalPerms) {
		t.Errorf("admin has %d perms, total is %d", len(admin.Permissions), totalPerms)
	}
}

func TestSeed_Idempotent(t *testing.T) {
	db := setupTestDB(t)
	Seed(db)
	Seed(db)
	var count int64
	db.Model(&Role{}).Count(&count)
	if count != 3 {
		t.Errorf("expected 3 roles after double seed, got %d", count)
	}
}

// Task 6 tests
func TestService_HasPermission(t *testing.T) {
	db := setupTestDB(t)
	Seed(db)
	svc := NewService(db)
	user := &User{ID: uuid.New(), KeycloakID: "kc-admin", Email: "admin@test.com", DisplayName: "Admin", Enabled: true}
	db.Create(user)
	var adminRole Role
	db.Where("name = ?", "admin").First(&adminRole)
	db.Model(user).Association("Roles").Append(&adminRole)
	ok, err := svc.HasPermission(user.ID, "models", "create")
	if err != nil {
		t.Fatalf("has permission: %v", err)
	}
	if !ok {
		t.Error("admin should have models:create")
	}
}

func TestService_ViewerCannotCreate(t *testing.T) {
	db := setupTestDB(t)
	Seed(db)
	svc := NewService(db)
	user := &User{ID: uuid.New(), KeycloakID: "kc-viewer", Email: "viewer@test.com", DisplayName: "Viewer", Enabled: true}
	db.Create(user)
	var viewerRole Role
	db.Where("name = ?", "viewer").First(&viewerRole)
	db.Model(user).Association("Roles").Append(&viewerRole)
	ok, _ := svc.HasPermission(user.ID, "models", "create")
	if ok {
		t.Error("viewer should NOT have models:create")
	}
	ok, _ = svc.HasPermission(user.ID, "models", "view")
	if !ok {
		t.Error("viewer should have models:view")
	}
}

func TestService_DeleteSystemRole_Rejected(t *testing.T) {
	db := setupTestDB(t)
	Seed(db)
	svc := NewService(db)
	var admin Role
	db.Where("name = ?", "admin").First(&admin)
	err := svc.DeleteRole(admin.ID)
	if err == nil {
		t.Error("deleting system role should fail")
	}
}

func TestService_CreateAndDeleteCustomRole(t *testing.T) {
	db := setupTestDB(t)
	Seed(db)
	svc := NewService(db)
	role, err := svc.CreateRole("ml-engineer", "ML team role")
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	if role.IsSystem {
		t.Error("custom role should not be system")
	}
	err = svc.DeleteRole(role.ID)
	if err != nil {
		t.Errorf("delete custom role: %v", err)
	}
}

func TestService_EnsureUser_CreatesNew(t *testing.T) {
	db := setupTestDB(t)
	Seed(db)
	svc := NewService(db)
	user, err := svc.EnsureUser("kc-new", "new@test.com", "New User")
	if err != nil {
		t.Fatalf("ensure user: %v", err)
	}
	if user.Email != "new@test.com" {
		t.Error("wrong email")
	}
	// Check viewer role assigned
	db.Preload("Roles").First(user, user.ID)
	hasViewer := false
	for _, r := range user.Roles {
		if r.Name == "viewer" {
			hasViewer = true
		}
	}
	if !hasViewer {
		t.Error("new user should get viewer role")
	}
}

func TestService_EnsureUser_ExistingNotDuplicated(t *testing.T) {
	db := setupTestDB(t)
	Seed(db)
	svc := NewService(db)
	u1, _ := svc.EnsureUser("kc-dup", "dup@test.com", "Dup User")
	u2, _ := svc.EnsureUser("kc-dup", "dup@test.com", "Dup User")
	if u1.ID != u2.ID {
		t.Error("EnsureUser should return same user")
	}
}
