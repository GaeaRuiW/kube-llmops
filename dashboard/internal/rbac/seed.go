package rbac

import (
	"fmt"

	"gorm.io/gorm"
)

// permDef defines a default permission to seed.
type permDef struct {
	Resource    string
	Action      string
	Description string
}

// defaultPermissions returns the 29 default permissions.
func defaultPermissions() []permDef {
	return []permDef{
		// models (6)
		{"models", "view", "View models"},
		{"models", "create", "Create models"},
		{"models", "edit", "Edit models"},
		{"models", "delete", "Delete models"},
		{"models", "scale", "Scale models"},
		{"models", "canary", "Canary deploy models"},
		// finetune (3)
		{"finetune", "view", "View fine-tune jobs"},
		{"finetune", "create", "Create fine-tune jobs"},
		{"finetune", "delete", "Delete fine-tune jobs"},
		// rag (5)
		{"rag", "view", "View RAG pipelines"},
		{"rag", "create", "Create RAG pipelines"},
		{"rag", "upload", "Upload RAG documents"},
		{"rag", "delete", "Delete RAG pipelines"},
		{"rag", "query", "Query RAG pipelines"},
		// platform (2)
		{"platform", "view", "View platform settings"},
		{"platform", "edit", "Edit platform settings"},
		// monitoring (1)
		{"monitoring", "view", "View monitoring dashboards"},
		// users (4)
		{"users", "view", "View users"},
		{"users", "create", "Create users"},
		{"users", "edit", "Edit users"},
		{"users", "delete", "Delete users"},
		// roles (4)
		{"roles", "view", "View roles"},
		{"roles", "create", "Create roles"},
		{"roles", "edit", "Edit roles"},
		{"roles", "delete", "Delete roles"},
		// permissions (4)
		{"permissions", "view", "View permissions"},
		{"permissions", "create", "Create permissions"},
		{"permissions", "edit", "Edit permissions"},
		{"permissions", "delete", "Delete permissions"},
	}
}

// roleDef defines a system role and a filter function that selects its permissions.
type roleDef struct {
	Name        string
	Description string
	Filter      func(p permDef) bool
}

// defaultRoles returns the 3 system roles with their permission filters.
func defaultRoles() []roleDef {
	return []roleDef{
		{
			Name:        "admin",
			Description: "Full access to all resources",
			Filter:      func(p permDef) bool { return true },
		},
		{
			Name:        "editor",
			Description: "Can view and modify models, fine-tune, and RAG resources",
			Filter: func(p permDef) bool {
				switch p.Resource {
				case "models", "finetune", "rag":
					return true
				case "monitoring", "platform":
					return p.Action == "view"
				}
				return false
			},
		},
		{
			Name:        "viewer",
			Description: "Read-only access to all resources",
			Filter:      func(p permDef) bool { return p.Action == "view" },
		},
	}
}

// Seed creates default permissions and system roles. It is idempotent.
func Seed(db *gorm.DB) error {
	perms := defaultPermissions()
	permModels := make([]Permission, 0, len(perms))

	for _, pd := range perms {
		var p Permission
		result := db.Where("resource = ? AND action = ?", pd.Resource, pd.Action).FirstOrCreate(&p, Permission{
			Resource:    pd.Resource,
			Action:      pd.Action,
			Description: pd.Description,
			IsSystem:    true,
		})
		if result.Error != nil {
			return fmt.Errorf("seed permission %s:%s: %w", pd.Resource, pd.Action, result.Error)
		}
		permModels = append(permModels, p)
	}

	roles := defaultRoles()
	for _, rd := range roles {
		var role Role
		result := db.Where("name = ?", rd.Name).FirstOrCreate(&role, Role{
			Name:        rd.Name,
			Description: rd.Description,
			IsSystem:    true,
		})
		if result.Error != nil {
			return fmt.Errorf("seed role %s: %w", rd.Name, result.Error)
		}

		// Build the list of permissions for this role.
		var rolePerms []Permission
		for i, pd := range perms {
			if rd.Filter(pd) {
				rolePerms = append(rolePerms, permModels[i])
			}
		}

		if err := db.Model(&role).Association("Permissions").Replace(rolePerms); err != nil {
			return fmt.Errorf("seed role %s permissions: %w", rd.Name, err)
		}
	}

	return nil
}
