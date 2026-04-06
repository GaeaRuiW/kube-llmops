package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/operator/internal/cli/printer"
	"github.com/kube-llmops/operator/internal/cli/util"
)

func newPlatformCmd() *cobra.Command {
	p := &cobra.Command{
		Use:   "platform",
		Short: "Manage the LLM platform",
	}
	p.AddCommand(newPlatformInitCmd())
	p.AddCommand(newPlatformStatusCmd())
	p.AddCommand(newPlatformUpdateCmd())
	return p
}

func newPlatformInitCmd() *cobra.Command {
	var (
		gateway       bool
		observability bool
		logging       bool
		rag           bool
		finetune      bool
		security      bool
		nodeportHost  string
		dryRun        bool
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the LLM platform",
		RunE: func(cmd *cobra.Command, args []string) error {
			lp := buildLLMPlatform(gateway, observability, logging, rag, finetune, security, nodeportHost)

			if dryRun {
				printer.PrintLLMPlatform(os.Stdout, "yaml", lp)
				return nil
			}

			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			lp.Namespace = kc.Namespace
			if err := kc.CRClient.Create(context.Background(), lp); err != nil {
				return fmt.Errorf("failed to create LLMPlatform: %w", err)
			}
			fmt.Fprintf(os.Stdout, "LLMPlatform %q created\n", lp.Name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&gateway, "gateway", true, "Enable gateway")
	cmd.Flags().BoolVar(&observability, "observability", true, "Enable observability")
	cmd.Flags().BoolVar(&logging, "logging", false, "Enable logging")
	cmd.Flags().BoolVar(&rag, "rag", false, "Enable RAG module")
	cmd.Flags().BoolVar(&finetune, "finetune", false, "Enable finetune module")
	cmd.Flags().BoolVar(&security, "security", false, "Enable security module")
	cmd.Flags().StringVar(&nodeportHost, "nodeport-host", "", "NodePort host IP")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print YAML without applying")
	return cmd
}

func newPlatformStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show platform status",
		RunE: func(cmd *cobra.Command, args []string) error {
			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			var lp v1alpha1.LLMPlatform
			if err := kc.CRClient.Get(context.Background(), types.NamespacedName{Name: "kube-llmops", Namespace: kc.Namespace}, &lp); err != nil {
				return fmt.Errorf("LLMPlatform not found. Run 'kubectl llmops platform init' first")
			}
			printer.PrintLLMPlatform(os.Stdout, output, &lp)
			return nil
		},
	}
}

func newPlatformUpdateCmd() *cobra.Command {
	var (
		enable  []string
		disable []string
		dryRun  bool
	)
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update platform configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			kc, err := util.NewKubeClients(kubeconfig, kubeCtx, namespace)
			if err != nil {
				return err
			}
			ctx := context.Background()
			var lp v1alpha1.LLMPlatform
			if err := kc.CRClient.Get(ctx, types.NamespacedName{Name: "kube-llmops", Namespace: kc.Namespace}, &lp); err != nil {
				return fmt.Errorf("LLMPlatform not found")
			}

			for _, m := range enable {
				setModule(&lp, m, true)
			}
			for _, m := range disable {
				setModule(&lp, m, false)
			}

			if dryRun {
				printer.PrintLLMPlatform(os.Stdout, "yaml", &lp)
				return nil
			}

			if err := kc.CRClient.Update(ctx, &lp); err != nil {
				return fmt.Errorf("failed to update: %w", err)
			}
			fmt.Fprintln(os.Stdout, "LLMPlatform updated.")
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&enable, "enable", nil, "Enable module (repeatable): rag|finetune|security|logging")
	cmd.Flags().StringArrayVar(&disable, "disable", nil, "Disable module (repeatable)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print YAML without applying")
	return cmd
}

func buildLLMPlatform(gateway, observability, logging, rag, finetune, security bool, nodeportHost string) *v1alpha1.LLMPlatform {
	lp := &v1alpha1.LLMPlatform{
		TypeMeta:   metav1.TypeMeta{APIVersion: "llmops.kubellmops.io/v1alpha1", Kind: "LLMPlatform"},
		ObjectMeta: metav1.ObjectMeta{Name: "kube-llmops"},
		Spec: v1alpha1.LLMPlatformSpec{
			Gateway:       v1alpha1.GatewaySpec{Enabled: gateway},
			Observability: v1alpha1.ObservabilitySpec{Enabled: observability},
			Logging:       v1alpha1.LoggingSpec{Enabled: logging},
			Modules: v1alpha1.ModulesSpec{
				RAG:      v1alpha1.EnabledToggle{Enabled: rag},
				Finetune: v1alpha1.EnabledToggle{Enabled: finetune},
				Security: v1alpha1.EnabledToggle{Enabled: security},
			},
			ModelStore: v1alpha1.ModelStoreSpec{
				Enabled:   true,
				Endpoint:  "kube-llmops-minio:9000",
				Bucket:    "models",
				AccessKey: "minioadmin",
				SecretKey: "minioadmin",
			},
			PostgreSQL: v1alpha1.PostgreSQLSpec{Enabled: true},
		},
	}
	if nodeportHost != "" {
		lp.Spec.NodePort = v1alpha1.NodePortSpec{Enabled: true, Host: nodeportHost}
	}
	return lp
}

func setModule(lp *v1alpha1.LLMPlatform, module string, enabled bool) {
	switch module {
	case "rag":
		lp.Spec.Modules.RAG.Enabled = enabled
	case "finetune":
		lp.Spec.Modules.Finetune.Enabled = enabled
	case "security":
		lp.Spec.Modules.Security.Enabled = enabled
	case "logging":
		lp.Spec.Logging.Enabled = enabled
	}
}
