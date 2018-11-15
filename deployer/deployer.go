package deployer

import (
	"fmt"
	"github.com/google/uuid"
	"os"
	"strings"

	"github.com/Rakanixu/k8-cid/utils"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	yaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
)

type Deployer struct {
	Client      *kubernetes.Clientset
	tags        []string
	uuid        uuid.UUID
	namespace   string
	deployments []*deployment
}

func NewDeployer(c *kubernetes.Clientset, t []string) (*Deployer, error) {
	return &Deployer{
		Client: c,
		tags:   t,
		uuid:   uuid.New(),
	}, nil
}

func (d *Deployer) Init() error {
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
	d.SetNamespace(namespace[0 : len(namespace)-1])

	if err := d.generateServiceAccounts(); err != nil {
		return err
	}

	if err := d.generateClusterRoles(); err != nil {
		return err
	}

	if err := d.generateClusterRoleBindings(); err != nil {
		return err
	}

	if err := d.generateDeployment(); err != nil {
		return err
	}

	if err := d.generateServices(); err != nil {
		return err
	}

	return nil
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

		// Creates service accounts
		if deployment.k8sServiceAccount.GetObjectMeta().GetName() != "" {
			fmt.Println("Creating service account ", deployment.k8sServiceAccount.GetObjectMeta().GetName())
			svcAccountClient := d.Client.CoreV1().ServiceAccounts(ns)
			result, err := svcAccountClient.Create(deployment.k8sServiceAccount)
			if err != nil {
				return err
			}
			fmt.Printf("Created service account %s on namespace %s \n", result.GetObjectMeta().GetName(), result.GetObjectMeta().GetNamespace())
		}

		// Creates cluster roles
		if deployment.k8sClusterRole.GetObjectMeta().GetName() != "" {
			fmt.Println("Creating cluster role ", deployment.k8sClusterRole.GetObjectMeta().GetName())
			clusterRoleClient := d.Client.RbacV1().ClusterRoles()
			result, err := clusterRoleClient.Create(deployment.k8sClusterRole)
			if err != nil {
				return err
			}
			fmt.Printf("Created cluster role %s on namespace %s \n", result.GetObjectMeta().GetName(), result.GetObjectMeta().GetNamespace())
		}

		// Creates cluster role bindings
		if deployment.k8sClusterRoleBinding.GetObjectMeta().GetName() != "" {
			fmt.Println("Creating cluster role binding ", deployment.k8sClusterRole.GetObjectMeta().GetName())
			clusterRoleClient := d.Client.RbacV1().ClusterRoleBindings()
			result, err := clusterRoleClient.Create(deployment.k8sClusterRoleBinding)
			if err != nil {
				return err
			}
			fmt.Printf("Created cluster role binding %s on namespace %s \n", result.GetObjectMeta().GetName(), result.GetObjectMeta().GetNamespace())
		}

		// Creates deployment
		if deployment.k8sDeployment.GetObjectMeta().GetName() != "" {
			fmt.Println("Creating deployment ", deployment.k8sDeployment.GetObjectMeta().GetName())
			deploymentsClient := d.Client.AppsV1().Deployments(ns)
			result, err := deploymentsClient.Create(deployment.k8sDeployment)
			if err != nil {
				return err
			}
			fmt.Printf("Created deployment %s on namespace %s \n", result.GetObjectMeta().GetName(), result.GetObjectMeta().GetNamespace())
		}

		// Creates service associated to deployment
		if deployment.k8sService.GetObjectMeta().GetName() != "" {
			fmt.Println("Creating service ", deployment.k8sService.GetObjectMeta().GetName())
			svcClient := d.Client.CoreV1().Services(ns)
			resultSvc, err := svcClient.Create(deployment.k8sService)
			if err != nil {
				return err
			}

			if resultSvc.Spec.Type == "NodePort" || resultSvc.Spec.Type == "LoadBalancer" {
				for _, v := range resultSvc.Spec.Ports {
					deployment.conn = append(deployment.conn, fmt.Sprintf("%s:%d", v.Name, v.NodePort))
				}
				// fmt.Println(resultSvc.Spec.LoadBalancerIP)
			}
			fmt.Printf("Created service %s on namespace %s \n", resultSvc.GetObjectMeta().GetName(), resultSvc.GetObjectMeta().GetNamespace())
		}
	}

	fmt.Println("\nExposed services")
	for _, deployment := range d.deployments {
		if len(deployment.conn) > 0 {
			for _, v := range deployment.conn {
				fmt.Println(v)
			}
		}
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

		// Deletes a deployment
		if deployment.k8sDeployment.GetObjectMeta().GetName() != "" {
			fmt.Println("Deleting deployment ", n)
			deploymentsClient := d.Client.AppsV1().Deployments(ns)
			if err := deploymentsClient.Delete(n, &metav1.DeleteOptions{
				PropagationPolicy: &deletePolicy,
			}); err != nil {
				if strings.Contains(err.Error(), "not found") {
					fmt.Println(err.Error())
				} else {
					return err
				}
			} else {
				fmt.Println("Deleted deployment ", n)
			}
		}

		nSvc := deployment.k8sService.GetObjectMeta().GetName()
		nsSvc := deployment.k8sService.GetObjectMeta().GetNamespace()

		// Deletes a service associated to a deployment
		if deployment.k8sService.GetObjectMeta().GetName() != "" {
			fmt.Println("Deleting service ", nSvc)
			svcClient := d.Client.CoreV1().Services(nsSvc)
			if err := svcClient.Delete(nSvc, &metav1.DeleteOptions{
				PropagationPolicy: &deletePolicy,
			}); err != nil {
				if strings.Contains(err.Error(), "not found") {
					fmt.Println(err.Error())
				} else {
					return err
				}
			} else {
				fmt.Println("Deleted service ", nSvc)
			}
		}

		nSvcAccount := deployment.k8sServiceAccount.GetObjectMeta().GetName()
		nsSvcAccount := deployment.k8sServiceAccount.GetObjectMeta().GetNamespace()

		// Deletes service accounts
		if deployment.k8sServiceAccount.GetObjectMeta().GetName() != "" {
			fmt.Println("Deleting service account ", nSvcAccount)
			svcAccountClient := d.Client.CoreV1().ServiceAccounts(nsSvcAccount)
			if err := svcAccountClient.Delete(nSvcAccount, &metav1.DeleteOptions{
				PropagationPolicy: &deletePolicy,
			}); err != nil {
				if strings.Contains(err.Error(), "not found") {
					fmt.Println(err.Error())
				} else {
					return err
				}
			} else {
				fmt.Println("Deleted service account ", nSvc)
			}
		}

		nClusterRoleBinding := deployment.k8sClusterRoleBinding.GetObjectMeta().GetName()

		// Deletes cluster role bindings
		if deployment.k8sClusterRoleBinding.GetObjectMeta().GetName() != "" {
			fmt.Println("Deleting cluster role binding ", nClusterRoleBinding)
			clusterRoleBindingClient := d.Client.RbacV1().ClusterRoleBindings()
			if err := clusterRoleBindingClient.Delete(nClusterRoleBinding, &metav1.DeleteOptions{
				PropagationPolicy: &deletePolicy,
			}); err != nil {
				if strings.Contains(err.Error(), "not found") {
					fmt.Println(err.Error())
				} else {
					return err
				}
			} else {
				fmt.Println("Deleted cluster role binding ", nClusterRoleBinding)
			}
		}

		nClusterRole := deployment.k8sClusterRole.GetObjectMeta().GetName()

		// Deletes cluster roles
		if deployment.k8sClusterRole.GetObjectMeta().GetName() != "" {
			fmt.Println("Deleting cluster role ", nClusterRole)
			clusterRoleClient := d.Client.RbacV1().ClusterRoles()
			if err := clusterRoleClient.Delete(nClusterRole, &metav1.DeleteOptions{
				PropagationPolicy: &deletePolicy,
			}); err != nil {
				if strings.Contains(err.Error(), "not found") {
					fmt.Println(err.Error())
				} else {
					return err
				}
			} else {
				fmt.Println("Deleted cluster role ", nClusterRole)
			}
		}
	}

	// Delete deployments's namespaces
	for _, dns := range deploymentNamespaces {
		fmt.Println("Deleting namespace ", dns)
		if err := d.Client.Core().Namespaces().Delete(dns, &metav1.DeleteOptions{}); err != nil {
			if strings.Contains(err.Error(), "not found") {
				fmt.Println(err.Error())
			} else {
				return err
			}
		} else {
			fmt.Println("Deleted namespace ", dns)
		}
	}

	return nil
}

func (d *Deployer) generateServiceAccounts() error {
	for _, deployment := range d.deployments {
		srcYML := fmt.Sprintf("config/%s-svc-account.yml", deployment.component)
		f, err := os.Open(srcYML)
		if err != nil {
			f, err = os.Open(srcYML)
			if err != nil {
				fmt.Println("Service Account not found for ", srcYML)
			}
		}

		if err == nil {
			if err = yaml.NewYAMLOrJSONDecoder(f, 1000).Decode(deployment.k8sServiceAccount); err != nil {
				return err
			}
			deployment.k8sServiceAccount.Name = deployment.k8sServiceAccount.Name + d.GetNamespace()
			deployment.k8sServiceAccount.Namespace = d.GetNamespace()
		}
	}

	return nil
}

func (d *Deployer) generateClusterRoles() error {
	for _, deployment := range d.deployments {
		srcYML := fmt.Sprintf("config/%s-cluster-role.yml", deployment.component)
		f, err := os.Open(srcYML)
		if err != nil {
			f, err = os.Open(srcYML)
			if err != nil {
				fmt.Println("Cluster role not found for ", srcYML)
			}
		}

		if err == nil {
			if err = yaml.NewYAMLOrJSONDecoder(f, 1000).Decode(deployment.k8sClusterRole); err != nil {
				return err
			}
			deployment.k8sClusterRole.Name = deployment.k8sClusterRole.Name + d.GetNamespace()
			deployment.k8sClusterRole.Namespace = d.GetNamespace()
		}
	}

	return nil
}

func (d *Deployer) generateClusterRoleBindings() error {
	for _, deployment := range d.deployments {
		srcYML := fmt.Sprintf("config/%s-cluster-role-binding.yml", deployment.component)
		f, err := os.Open(srcYML)
		if err != nil {
			f, err = os.Open(srcYML)
			if err != nil {
				fmt.Println("Cluster role binding not found for ", srcYML)
			}
		}

		if err == nil {
			if err = yaml.NewYAMLOrJSONDecoder(f, 1000).Decode(deployment.k8sClusterRoleBinding); err != nil {
				return err
			}
			deployment.k8sClusterRoleBinding.Name = deployment.k8sClusterRoleBinding.Name + d.GetNamespace()
			deployment.k8sClusterRoleBinding.Namespace = d.GetNamespace()
			for k, v := range deployment.k8sClusterRoleBinding.Subjects {
				if v.Kind == "ServiceAccount" {
					if deployment.k8sClusterRoleBinding.Subjects[k].Name != "default" {
						deployment.k8sClusterRoleBinding.Subjects[k].Name =
							deployment.k8sClusterRoleBinding.Subjects[k].Name + d.GetNamespace()
					}
					deployment.k8sClusterRoleBinding.Subjects[k].Namespace = d.GetNamespace()
				}
			}
		}
	}

	return nil
}

func (d *Deployer) generateDeployment() error {
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

		deployment.k8sDeployment.Namespace = d.GetNamespace()
		if deployment.k8sDeployment.Spec.Template.Spec.ServiceAccountName != "" {
			deployment.k8sDeployment.Spec.Template.Spec.ServiceAccountName = deployment.k8sServiceAccount.Name
		}
		for k, v := range deployment.k8sDeployment.Spec.Template.Spec.Containers {
			img := strings.Split(v.Image, ":")
			deployment.k8sDeployment.Spec.Template.Spec.Containers[k].Image = fmt.Sprintf("%s:%s", img[0], deployment.commitTag)
		}
	}

	return nil
}

func (d *Deployer) generateServices() error {
	for _, deployment := range d.deployments {
		srcYML := fmt.Sprintf("config/%s-svc.yml", deployment.component)
		f, err := os.Open(srcYML)
		if err != nil {
			f, err = os.Open(srcYML)
			if err != nil {
				fmt.Println("Service not found for ", srcYML)
			}
		}

		if err == nil {
			if err = yaml.NewYAMLOrJSONDecoder(f, 1000).Decode(deployment.k8sService); err != nil {
				return err
			}

			deployment.k8sService.Namespace = deployment.k8sDeployment.Namespace
		}
	}

	return nil
}

func (d *Deployer) SetNamespace(ns string) {
	d.namespace = strings.Replace(ns, ".", "-", -1)
}

func (d *Deployer) GetNamespace() string {
	return d.namespace
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
	component             string
	repo                  string
	commitTag             string
	conn                  []string
	k8sDeployment         *appsv1.Deployment
	k8sService            *apiv1.Service
	k8sServiceAccount     *apiv1.ServiceAccount
	k8sClusterRole        *rbacv1.ClusterRole
	k8sClusterRoleBinding *rbacv1.ClusterRoleBinding
}

func newDeployment(c string, r string, ct string) *deployment {
	return &deployment{
		component:             c,
		repo:                  r,
		commitTag:             ct,
		conn:                  []string{},
		k8sDeployment:         &appsv1.Deployment{},
		k8sService:            &apiv1.Service{},
		k8sServiceAccount:     &apiv1.ServiceAccount{},
		k8sClusterRole:        &rbacv1.ClusterRole{},
		k8sClusterRoleBinding: &rbacv1.ClusterRoleBinding{},
	}
}
