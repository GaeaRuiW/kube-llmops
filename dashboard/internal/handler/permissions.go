package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/kube-llmops/dashboard/internal/rbac"
)

func ListPermissions(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var perms []rbac.Permission
		if err := db.Find(&perms).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, perms)
	}
}

func CreatePermission(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Resource    string `json:"resource" binding:"required"`
			Action      string `json:"action" binding:"required"`
			Description string `json:"description"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		svc := rbac.NewService(db)
		perm, err := svc.CreatePermission(body.Resource, body.Action, body.Description)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, perm)
	}
}

func UpdatePermission(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid id"})
			return
		}
		var perm rbac.Permission
		if err := db.First(&perm, id).Error; err != nil {
			c.JSON(404, gin.H{"error": "permission not found"})
			return
		}
		if perm.IsSystem {
			c.JSON(403, gin.H{"error": "cannot modify system permission"})
			return
		}
		var body struct {
			Description *string `json:"description"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if body.Description != nil {
			perm.Description = *body.Description
		}
		db.Save(&perm)
		c.JSON(http.StatusOK, perm)
	}
}

func DeletePermission(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid id"})
			return
		}
		svc := rbac.NewService(db)
		if err := svc.DeletePermission(id); err != nil {
			c.JSON(403, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	}
}
