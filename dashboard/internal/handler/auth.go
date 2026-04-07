package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/kube-llmops/dashboard/internal/auth"
	"github.com/kube-llmops/dashboard/internal/rbac"
)

func GetCurrentUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawUser, ok := c.Get("user")
		if !ok {
			c.JSON(http.StatusOK, gin.H{
				"user": nil,
			})
			return
		}
		user := rawUser.(*rbac.User)
		perms, _ := c.Get("permissions")
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

// AuthLogin redirects the browser to Keycloak's authorization endpoint.
func AuthLogin(oidcProv *auth.OIDCProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		if oidcProv == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SSO not configured"})
			return
		}
		b := make([]byte, 16)
		rand.Read(b)
		state := hex.EncodeToString(b)
		c.Redirect(http.StatusFound, oidcProv.AuthCodeURL(state))
	}
}

// AuthCallback exchanges the authorization code for tokens and redirects
// the browser to the frontend with the id_token as a URL fragment.
func AuthCallback(oidcProv *auth.OIDCProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		if oidcProv == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SSO not configured"})
			return
		}
		code := c.Query("code")
		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing code"})
			return
		}
		token, err := oidcProv.Exchange(c.Request.Context(), code)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "exchange failed: " + err.Error()})
			return
		}
		rawIDToken, ok := token.Extra("id_token").(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no id_token"})
			return
		}
		// Redirect to frontend with token in query (frontend picks it up and cleans URL)
		redirectURL := "/login/callback?" + url.Values{
			"id_token": {rawIDToken},
		}.Encode()
		c.Redirect(http.StatusFound, redirectURL)
	}
}

// AuthConfig returns the SSO configuration for the frontend.
func AuthConfig(oidcProv *auth.OIDCProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"sso_enabled": oidcProv != nil,
		})
	}
}

func AuthLogout() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "logged out"})
	}
}
