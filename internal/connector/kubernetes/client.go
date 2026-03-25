package kubernetes

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ahlert/telvar/internal/catalog"
	"github.com/ahlert/telvar/internal/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	clientset k8s.Interface
}

func NewClient(cfg *config.KubernetesConfig) (*Client, error) {
	var restCfg *rest.Config
	var err error

	if cfg.InCluster {
		restCfg, err = rest.InClusterConfig()
	} else {
		restCfg, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
	}
	if err != nil {
		return nil, fmt.Errorf("building k8s config: %w", err)
	}

	cs, err := k8s.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("creating k8s clientset: %w", err)
	}

	return &Client{clientset: cs}, nil
}

type DeploymentInfo struct {
	Name            string
	Namespace       string
	Image           string
	ReplicasDesired int32
	ReplicasReady   int32
	Labels          map[string]string
}

func (c *Client) ListDeployments(ctx context.Context) ([]DeploymentInfo, error) {
	deployments, err := c.clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing deployments: %w", err)
	}

	var result []DeploymentInfo
	for _, d := range deployments.Items {
		info := DeploymentInfo{
			Name:          d.Name,
			Namespace:     d.Namespace,
			ReplicasReady: d.Status.ReadyReplicas,
			Labels:        d.Labels,
		}

		if d.Spec.Replicas != nil {
			info.ReplicasDesired = *d.Spec.Replicas
		}

		if len(d.Spec.Template.Spec.Containers) > 0 {
			info.Image = d.Spec.Template.Spec.Containers[0].Image
		}

		result = append(result, info)
	}

	return result, nil
}

func DeploymentToEntity(d DeploymentInfo) catalog.Entity {
	e := catalog.Entity{
		Name:     d.Name,
		Kind:     catalog.KindComponent,
		Tags:     make(map[string]string),
		Metadata: make(map[string]string),
	}

	e.Metadata["k8s.namespace"] = d.Namespace
	e.Metadata["k8s.image"] = d.Image
	e.Metadata["k8s.replicas_desired"] = strconv.Itoa(int(d.ReplicasDesired))
	e.Metadata["k8s.replicas_ready"] = strconv.Itoa(int(d.ReplicasReady))

	if env := inferEnvironment(d.Namespace); env != "" {
		e.Tags["environment"] = env
	}

	if team, ok := d.Labels["team"]; ok {
		e.Owner = team
	} else if managedBy, ok := d.Labels["app.kubernetes.io/managed-by"]; ok {
		e.Owner = managedBy
	}

	return e
}

func inferEnvironment(namespace string) string {
	ns := strings.ToLower(namespace)
	switch {
	case strings.Contains(ns, "prod"):
		return "production"
	case strings.Contains(ns, "staging"), strings.Contains(ns, "stg"):
		return "staging"
	case strings.Contains(ns, "dev"):
		return "development"
	default:
		return ""
	}
}
