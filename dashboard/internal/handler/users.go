package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/kube-llmops/dashboard/internal/rbac"
)

func ListUsers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var users []rbac.User
		if err := db.Preload("Roles").Find(&users).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, users)
	}
}

func GetUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid id"})
			return
		}
		var user rbac.User
		if err := db.Preload("Roles.Permissions").First(&user, id).Error; err != nil {
			c.JSON(404, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusOK, user)
	}
}

func CreateUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Email       string   `json:"email" binding:"required"`
			DisplayName string   `json:"displayName" binding:"required"`
			RoleIDs     []string `json:"roleIds"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		svc := rbac.NewService(db)
		user, err := svc.EnsureUser("manual-"+body.Email, body.Email, body.DisplayName)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		// Assign roles if provided
		if len(body.RoleIDs) > 0 {
			var roles []rbac.Role
			for _, rid := range body.RoleIDs {
				id, err := uuid.Parse(rid)
				if err != nil {
					continue
				}
				var role rbac.Role
				if db.First(&role, id).Error == nil {
					roles = append(roles, role)
				}
			}
			db.Model(user).Association("Roles").Replace(roles)
		}
		db.Preload("Roles").First(user, user.ID)
		c.JSON(http.StatusCreated, user)
	}
}

func UpdateUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid id"})
			return
		}
		var user rbac.User
		if err := db.First(&user, id).Error; err != nil {
			c.JSON(404, gin.H{"error": "user not found"})
			return
		}
		var body struct {
			DisplayName *string `json:"displayName"`
			Enabled     *bool   `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if body.DisplayName != nil {
			user.DisplayName = *body.DisplayName
		}
		if body.Enabled != nil {
			user.Enabled = *body.Enabled
		}
		db.Save(&user)
		c.JSON(http.StatusOK, user)
	}
}

func DeleteUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid id"})
			return
		}
		var user rbac.User
		if err := db.First(&user, id).Error; err != nil {
			c.JSON(404, gin.H{"error": "user not found"})
			return
		}
		db.Model(&user).Association("Roles").Clear()
		db.Delete(&user)
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	}
}

func AssignRoles(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid id"})
			return
		}
		var user rbac.User
		if err := db.First(&user, id).Error; err != nil {
			c.JSON(404, gin.H{"error": "user not found"})
			return
		}
		var body struct {
			RoleIDs []string `json:"roleIds" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		var roles []rbac.Role
		for _, rid := range body.RoleIDs {
			uid, err := uuid.Parse(rid)
			if err != nil {
				continue
			}
			var role rbac.Role
			if db.First(&role, uid).Error == nil {
				roles = append(roles, role)
			}
		}
		db.Model(&user).Association("Roles").Replace(roles)
		db.Preload("Roles").First(&user, user.ID)
		c.JSON(http.StatusOK, user)
	}
}
