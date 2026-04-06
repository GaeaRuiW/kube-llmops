package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/kube-llmops/dashboard/internal/auth"
	"github.com/kube-llmops/dashboard/internal/rbac"
)

func GetCurrentUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := c.MustGet("user").(*rbac.User)
		perms := c.MustGet("permissions").([]rbac.Permission)
		svc := rbac.NewService(db)
		full, err := svc.FindUserByKeycloakID(user.KeycloakID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"user":        full,
			"permissions": perms,
		})
	}
}

func AuthCallback(oidcProv *auth.OIDCProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Query("code")
		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing code"})
			return
		}
		token, err := oidcProv.Exchange(c.Request.Context(), code)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "exchange failed"})
			return
		}
		rawIDToken, ok := token.Extra("id_token").(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no id_token"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"access_token": token.AccessToken,
			"id_token":     rawIDToken,
			"expires_in":   token.Expiry,
		})
	}
}

func AuthLogout() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "logged out"})
	}
}
