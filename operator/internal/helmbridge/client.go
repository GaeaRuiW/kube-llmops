package helmbridge

import (
	"fmt"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
)

// HelmClient defines the interface for Helm operations.
type HelmClient interface {
	Install(name, namespace, chartPath string, values map[string]interface{}) (*release.Release, error)
	Upgrade(name, namespace, chartPath string, values map[string]interface{}) (*release.Release, error)
	GetRelease(name, namespace string) (*release.Release, error)
	Uninstall(name, namespace string) error
	Rollback(name, namespace string, version int) error
}

// SDKClient implements HelmClient using the Helm SDK.
type SDKClient struct{}

// actionConfig creates a new Helm action configuration for the given namespace.
func (c *SDKClient) actionConfig(namespace string) (*action.Configuration, error) {
	settings := cli.New()
	cfg := new(action.Configuration)
	if err := cfg.Init(settings.RESTClientGetter(), namespace, "secret", func(format string, v ...interface{}) {}); err != nil {
		return nil, fmt.Errorf("init helm config: %w", err)
	}
	return cfg, nil
}

// Install installs a Helm chart as a new release.
func (c *SDKClient) Install(name, namespace, chartPath string, values map[string]interface{}) (*release.Release, error) {
	cfg, err := c.actionConfig(namespace)
	if err != nil {
		return nil, err
	}

	chart, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}

	install := action.NewInstall(cfg)
	install.ReleaseName = name
	install.Namespace = namespace
	install.CreateNamespace = false
	install.Wait = false
	install.DisableOpenAPIValidation = true
	install.SkipCRDs = true   // CRDs managed separately
	install.DisableHooks = true // Hooks block on long-running jobs

	return install.Run(chart, values)
}

// Upgrade upgrades an existing Helm release.
func (c *SDKClient) Upgrade(name, namespace, chartPath string, values map[string]interface{}) (*release.Release, error) {
	cfg, err := c.actionConfig(namespace)
	if err != nil {
		return nil, err
	}

	chart, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}

	upgrade := action.NewUpgrade(cfg)
	upgrade.Namespace = namespace
	upgrade.Wait = false
	upgrade.DisableHooks = true // Hooks block on long-running jobs

	return upgrade.Run(name, chart, values)
}

// GetRelease retrieves an existing Helm release by name.
func (c *SDKClient) GetRelease(name, namespace string) (*release.Release, error) {
	cfg, err := c.actionConfig(namespace)
	if err != nil {
		return nil, err
	}

	get := action.NewGet(cfg)
	return get.Run(name)
}

// Uninstall removes a Helm release by name.
func (c *SDKClient) Uninstall(name, namespace string) error {
	cfg, err := c.actionConfig(namespace)
	if err != nil {
		return err
	}

	uninstall := action.NewUninstall(cfg)
	_, err = uninstall.Run(name)
	return err
}

// Rollback rolls back a Helm release to the specified version (0 = previous).
func (c *SDKClient) Rollback(name, namespace string, version int) error {
	cfg, err := c.actionConfig(namespace)
	if err != nil {
		return err
	}

	rollback := action.NewRollback(cfg)
	rollback.Version = version
	rollback.CleanupOnFail = true
	return rollback.Run(name)
}

// FixStuckRelease detects and recovers from pending-install / pending-upgrade states.
// Returns true if the release was fixed and the caller should retry.
func (c *SDKClient) FixStuckRelease(name, namespace string) (bool, error) {
	rel, err := c.GetRelease(name, namespace)
	if err != nil || rel == nil {
		return false, nil
	}

	switch rel.Info.Status {
	case release.StatusPendingInstall:
		// Stuck in pending-install — uninstall and let caller re-install
		_ = c.Uninstall(name, namespace)
		return true, nil
	case release.StatusPendingUpgrade, release.StatusPendingRollback:
		// Stuck in pending-upgrade — rollback to last good version
		if err := c.Rollback(name, namespace, 0); err != nil {
			// Rollback failed — uninstall as last resort
			_ = c.Uninstall(name, namespace)
		}
		return true, nil
	case release.StatusFailed:
		// Last operation failed — try upgrade (Helm allows upgrade from failed state)
		return false, nil
	default:
		return false, nil
	}
}

// MockHelmClient is a test double for HelmClient.
type MockHelmClient struct {
	LastValues map[string]interface{}
	InstallErr error
	UpgradeErr error
}

// Install records the values and returns a fake release or a configured error.
func (m *MockHelmClient) Install(name, namespace, chartPath string, values map[string]interface{}) (*release.Release, error) {
	m.LastValues = values
	if m.InstallErr != nil {
		return nil, m.InstallErr
	}
	return &release.Release{Name: name, Version: 1}, nil
}

// Upgrade records the values and returns a fake release or a configured error.
func (m *MockHelmClient) Upgrade(name, namespace, chartPath string, values map[string]interface{}) (*release.Release, error) {
	m.LastValues = values
	if m.UpgradeErr != nil {
		return nil, m.UpgradeErr
	}
	return &release.Release{Name: name, Version: 2}, nil
}

// GetRelease returns a fake release with the given name.
func (m *MockHelmClient) GetRelease(name, namespace string) (*release.Release, error) {
	return &release.Release{Name: name, Version: 1}, nil
}

// Uninstall is a no-op for the mock.
func (m *MockHelmClient) Uninstall(name, namespace string) error {
	return nil
}

// Rollback is a no-op for the mock.
func (m *MockHelmClient) Rollback(name, namespace string, version int) error {
	return nil
}
