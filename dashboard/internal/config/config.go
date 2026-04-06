package config

import "os"

type Config struct {
	Port      string
	Namespace string
	DB        DBConfig
	OIDC      OIDCConfig
	Proxy     ProxyConfig
}

type DBConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
}

func (d DBConfig) DSN() string {
	return "host=" + d.Host + " port=" + d.Port + " user=" + d.User +
		" password=" + d.Password + " dbname=" + d.Name + " sslmode=disable"
}

type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type ProxyConfig struct {
	Grafana    string
	Langfuse   string
	Dify       string
	MLflow     string
	JupyterHub string
	MinIO      string
	Keycloak   string
	LiteLLM    string
	Prometheus string
}

func Load() *Config {
	return &Config{
		Port:      env("PORT", "3000"),
		Namespace: env("NAMESPACE", "default"),
		DB: DBConfig{
			Host:     env("DB_HOST", "localhost"),
			Port:     env("DB_PORT", "5432"),
			Name:     env("DB_NAME", "dashboard"),
			User:     env("DB_USER", "dashboard"),
			Password: env("DB_PASSWORD", "dashboard-default-pw"),
		},
		OIDC: OIDCConfig{
			IssuerURL:    env("OIDC_ISSUER_URL", ""),
			ClientID:     env("OIDC_CLIENT_ID", "dashboard"),
			ClientSecret: env("OIDC_CLIENT_SECRET", "dashboard-oidc-secret"),
			RedirectURL:  env("OIDC_REDIRECT_URL", ""),
		},
		Proxy: ProxyConfig{
			Grafana:    env("PROXY_GRAFANA", "http://kube-llmops-grafana:3000"),
			Langfuse:   env("PROXY_LANGFUSE", "http://kube-llmops-langfuse:3000"),
			Dify:       env("PROXY_DIFY", "http://kube-llmops-dify-web:3000"),
			MLflow:     env("PROXY_MLFLOW", "http://kube-llmops-mlflow:5000"),
			JupyterHub: env("PROXY_JUPYTERHUB", "http://kube-llmops-jupyterhub:8000"),
			MinIO:      env("PROXY_MINIO", "http://kube-llmops-minio:9001"),
			Keycloak:   env("PROXY_KEYCLOAK", "http://kube-llmops-keycloak:8080"),
			LiteLLM:    env("PROXY_LITELLM", "http://kube-llmops-litellm:4000"),
			Prometheus: env("PROXY_PROMETHEUS", "http://kube-llmops-prometheus:9090"),
		},
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
