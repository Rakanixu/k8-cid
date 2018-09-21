package deployer

import (
	//"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"os"
	"strings"
	"time"

	"github.com/Rakanixu/k8-cid/utils"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	yaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
)

type Deployer struct {
	Client      *kubernetes.Clientset
	tags        []string
	uuid        uuid.UUID
	deployments []*deployment
}

func NewDeployer(c *kubernetes.Clientset, t []string) (*Deployer, error) {
	return &Deployer{
		Client: c,
		tags:   t,
		uuid:   uuid.New(),
	}, nil
}

func (d *Deployer) Create() error {
	liveNamespaces, err := d.namespaces()
	if err != nil {
		return err
	}

	for _, deployment := range d.deployments {
		ns := deployment.k8sDeployment.GetObjectMeta().GetNamespace()

		// Namespace does not exits
		if utils.Find(liveNamespaces, ns) == -1 {
			nsSpec := &apiv1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
				},
			}

			fmt.Println("Creating namespace ", ns)
			_, err := d.Client.Core().Namespaces().Create(nsSpec)
			if err != nil {
				return err
			}

			// Update namespaces
			if liveNamespaces, err = d.namespaces(); err != nil {
				return err
			}
		}

		fmt.Println("Creating deployment ", deployment.k8sDeployment.GetObjectMeta().GetName())
		deploymentsClient := d.Client.AppsV1().Deployments(ns)
		result, err := deploymentsClient.Create(deployment.k8sDeployment)
		if err != nil {
			return err
		}

		fmt.Printf("Created deployment %s on namespace %s \n", result.GetObjectMeta().GetName(), result.GetObjectMeta().GetNamespace())
	}

	return nil
}

func (d *Deployer) Delete() error {
	var deploymentNamespaces []string
	deletePolicy := metav1.DeletePropagationForeground

	// Delete all deployments
	for _, deployment := range d.deployments {
		n := deployment.k8sDeployment.GetObjectMeta().GetName()
		ns := deployment.k8sDeployment.GetObjectMeta().GetNamespace()

		if utils.Find(deploymentNamespaces, ns) == -1 {
			deploymentNamespaces = append(deploymentNamespaces, ns)
		}

		fmt.Println("Deleting deployment ", n)

		deploymentsClient := d.Client.AppsV1().Deployments(ns)
		if err := deploymentsClient.Delete(n, &metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		}); err != nil {
			return err
		}
		fmt.Println("Deleted deployment ", n)
	}

	// Delete deployments's namespaces
	for _, dns := range deploymentNamespaces {
		fmt.Println("Deleting namespace ", dns)
		if err := d.Client.Core().Namespaces().Delete(dns, &metav1.DeleteOptions{}); err != nil {
			return err
		}
		fmt.Println("Deleted namespace ", dns)
	}

	return nil
}

func (d *Deployer) GenerateDeployment() error {
	reposMap := utils.ReadRepos()
	namespace := ""

	for _, v := range d.tags {
		s := strings.Split(v, "=")
		repo := s[0]
		commitTag := s[1]
		namespace += repo + "-" + commitTag + "-"

		for _, component := range reposMap[repo] {
			d.deployments = append(d.deployments, newDeployment(component, repo, commitTag))
		}
	}
	namespace += time.Now().Format("2006-01-02-3-4")

	for _, deployment := range d.deployments {
		f, err := os.Open(fmt.Sprintf("config/%s.yaml", deployment.component))
		if err != nil {
			f, err = os.Open(fmt.Sprintf("config/%s.yml", deployment.component))
			if err != nil {
				return err
			}
		}

		if err = yaml.NewYAMLOrJSONDecoder(f, 1000).Decode(deployment.k8sDeployment); err != nil {
			return err
		}

		deployment.k8sDeployment.Namespace = namespace
	}

	return nil
}

func (d *Deployer) namespaces() ([]string, error) {
	var liveNamespaces []string
	nss, err := d.Client.Core().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, v := range nss.Items {
		liveNamespaces = append(liveNamespaces, v.GetObjectMeta().GetName())
	}

	return liveNamespaces, nil
}

type deployment struct {
	component     string
	repo          string
	commitTag     string
	k8sDeployment *appsv1.Deployment
}

func newDeployment(c string, r string, ct string) *deployment {
	return &deployment{
		component:     c,
		repo:          r,
		commitTag:     ct,
		k8sDeployment: &appsv1.Deployment{},
	}
}
