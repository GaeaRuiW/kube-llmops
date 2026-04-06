package printer

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sigsyaml "sigs.k8s.io/yaml"
)

const apiVersion = "llmops.kubellmops.io/v1alpha1"

// ensureMDTypeMeta restores TypeMeta that client-go strips on reads.
func ensureMDTypeMeta(items []v1alpha1.ModelDeployment) {
	for i := range items {
		items[i].TypeMeta = metav1.TypeMeta{APIVersion: apiVersion, Kind: "ModelDeployment"}
	}
}

// PrintModelDeployments prints a list of ModelDeployments in the specified format.
func PrintModelDeployments(w io.Writer, format string, items []v1alpha1.ModelDeployment) {
	switch format {
	case "json":
		ensureMDTypeMeta(items)
		printJSON(w, items)
	case "yaml":
		ensureMDTypeMeta(items)
		printYAML(w, items)
	case "wide":
		printMDTable(w, items, true)
	default:
		printMDTable(w, items, false)
	}
}

// PrintFineTuneRuns prints a list of FineTuneRuns in the specified format.
func PrintFineTuneRuns(w io.Writer, format string, items []v1alpha1.FineTuneRun) {
	switch format {
	case "json", "yaml":
		for i := range items {
			items[i].TypeMeta = metav1.TypeMeta{APIVersion: apiVersion, Kind: "FineTuneRun"}
		}
	}
	switch format {
	case "json":
		printJSON(w, items)
	case "yaml":
		printYAML(w, items)
	default:
		printFTRTable(w, items)
	}
}

// PrintLLMPlatform prints an LLMPlatform in the specified format.
func PrintLLMPlatform(w io.Writer, format string, item *v1alpha1.LLMPlatform) {
	if format == "json" || format == "yaml" {
		item.TypeMeta = metav1.TypeMeta{APIVersion: apiVersion, Kind: "LLMPlatform"}
	}
	switch format {
	case "json":
		printJSON(w, item)
	case "yaml":
		printYAML(w, item)
	default:
		printPlatformTable(w, item)
	}
}

func printMDTable(w io.Writer, items []v1alpha1.ModelDeployment, wide bool) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if wide {
		fmt.Fprintln(tw, "NAME\tENGINE\tREPLICAS\tPHASE\tENDPOINT\tSOURCE")
	} else {
		fmt.Fprintln(tw, "NAME\tENGINE\tREPLICAS\tPHASE")
	}
	for _, md := range items {
		replicas := fmt.Sprintf("%d/%d", md.Status.ReadyReplicas, md.Status.TotalReplicas)
		if wide {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", md.Name, md.Status.Engine, replicas, md.Status.Phase, md.Status.Endpoint, md.Spec.Source)
		} else {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", md.Name, md.Status.Engine, replicas, md.Status.Phase)
		}
	}
	tw.Flush()
}

func printFTRTable(w io.Writer, items []v1alpha1.FineTuneRun) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tBASE MODEL\tMETHOD\tPHASE\tLOSS\tDURATION")
	for _, ftr := range items {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", ftr.Name, ftr.Spec.BaseModel, ftr.Spec.Method, ftr.Status.Phase, ftr.Status.Metrics.TrainLoss, ftr.Status.Metrics.TrainingDuration)
	}
	tw.Flush()
}

func printPlatformTable(w io.Writer, lp *v1alpha1.LLMPlatform) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "COMPONENT\tPHASE\tENDPOINT")
	fmt.Fprintf(tw, "Platform\t%s\t\n", lp.Status.Phase)
	printComponent(tw, "Gateway", lp.Status.Components.Gateway)
	printComponent(tw, "Grafana", lp.Status.Components.Grafana)
	printComponent(tw, "Prometheus", lp.Status.Components.Prometheus)
	printComponent(tw, "Langfuse", lp.Status.Components.Langfuse)
	printComponent(tw, "MinIO", lp.Status.Components.MinIO)
	printComponent(tw, "PostgreSQL", lp.Status.Components.PostgreSQL)
	printComponent(tw, "Dify", lp.Status.Components.Dify)
	tw.Flush()
}

func printComponent(tw *tabwriter.Writer, name string, cs *v1alpha1.ComponentStatus) {
	if cs == nil {
		return
	}
	fmt.Fprintf(tw, "  %s\t%s\t%s\n", name, cs.Phase, cs.Endpoint)
}

func printJSON(w io.Writer, v interface{}) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func printYAML(w io.Writer, v interface{}) {
	b, err := sigsyaml.Marshal(v)
	if err != nil {
		fmt.Fprintf(w, "error: %v\n", err)
		return
	}
	w.Write(b)
}
