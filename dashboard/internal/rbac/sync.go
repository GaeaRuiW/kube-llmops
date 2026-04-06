package rbac

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

type KeycloakClient struct {
	baseURL string
	realm   string
	client  *http.Client
}

func NewKeycloakClient() *KeycloakClient {
	return &KeycloakClient{
		baseURL: os.Getenv("KEYCLOAK_ADMIN_URL"),
		realm:   "kube-llmops",
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// SyncUsers fetches all users from Keycloak and syncs to local DB.
// TODO: implement when Keycloak Admin API is available
func (kc *KeycloakClient) SyncUsers() error {
	if kc.baseURL == "" {
		return fmt.Errorf("KEYCLOAK_ADMIN_URL not set")
	}
	return nil
}
