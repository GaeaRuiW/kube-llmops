package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/kube-llmops/dashboard/internal/rbac"
)

func ListRoles(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var roles []rbac.Role
		if err := db.Preload("Permissions").Find(&roles).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, roles)
	}
}

func GetRole(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid id"})
			return
		}
		var role rbac.Role
		if err := db.Preload("Permissions").First(&role, id).Error; err != nil {
			c.JSON(404, gin.H{"error": "role not found"})
			return
		}
		c.JSON(http.StatusOK, role)
	}
}

func CreateRole(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Name        string `json:"name" binding:"required"`
			Description string `json:"description"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		svc := rbac.NewService(db)
		role, err := svc.CreateRole(body.Name, body.Description)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, role)
	}
}

func UpdateRole(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid id"})
			return
		}
		var role rbac.Role
		if err := db.First(&role, id).Error; err != nil {
			c.JSON(404, gin.H{"error": "role not found"})
			return
		}
		if role.IsSystem {
			c.JSON(403, gin.H{"error": "cannot modify system role"})
			return
		}
		var body struct {
			Name        *string `json:"name"`
			Description *string `json:"description"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if body.Name != nil {
			role.Name = *body.Name
		}
		if body.Description != nil {
			role.Description = *body.Description
		}
		db.Save(&role)
		c.JSON(http.StatusOK, role)
	}
}

func DeleteRole(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid id"})
			return
		}
		svc := rbac.NewService(db)
		if err := svc.DeleteRole(id); err != nil {
			c.JSON(403, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	}
}

func SetRolePermissions(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid id"})
			return
		}
		var body struct {
			PermissionIDs []string `json:"permissionIds" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		var permIDs []uuid.UUID
		for _, pid := range body.PermissionIDs {
			uid, err := uuid.Parse(pid)
			if err != nil {
				continue
			}
			permIDs = append(permIDs, uid)
		}
		svc := rbac.NewService(db)
		if err := svc.SetRolePermissions(id, permIDs); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "permissions updated"})
	}
}
