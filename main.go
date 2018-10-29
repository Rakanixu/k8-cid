package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	//"time"

	"github.com/Rakanixu/k8-cid/deployer"
	"github.com/Rakanixu/k8-cid/utils"
	// "k8s.io/apimachinery/pkg/api/errors"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
)

type arrayFlags []string

func (i *arrayFlags) String() string {
	return "my string representation"
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var repoComponents arrayFlags
var reposCommits arrayFlags

func main() {
	var kubeconfig *string
	if home := utils.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Var(&repoComponents, "config", "Set which component / microservice belongs to each repository")
	flag.Var(&reposCommits, "repos", "Repositories")
	flag.Parse()
	tailArgs := flag.Args()

	// create hidden folder to store k8s-cid configuration data
	utils.CreateDirIfNotExist(utils.HomeDir() + utils.K8sCidWorkingDir)

	// set configuration
	if len(repoComponents) > 0 {
		var m map[string][]string
		m = make(map[string][]string)
		for i := 0; i < len(repoComponents); i++ {
			splitted := strings.Split(repoComponents[i], "=")
			m[splitted[0]] = strings.Split(splitted[1], ",")
		}
		j, err := json.Marshal(m)
		if err != nil {
			fmt.Println(err)
			panic(err)
		}
		if err := ioutil.WriteFile(utils.HomeDir()+utils.K8sCidWorkingDir+"/repositories-components.json", j, 0777); err != nil {
			fmt.Println("Could not save configuration file.")
		}
		fmt.Println("Configuration file saved!")
		return
	}

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// create deployer
	d, err := deployer.NewDeployer(clientset, reposCommits)
	if err != nil {
		panic(err.Error())
	}

	if err := d.Init(); err != nil {
		panic(err.Error())
	}

	// Create deployment
	if len(tailArgs) == 1 && tailArgs[0] == utils.CREATE_RESOURCE {
		if err := d.Create(); err != nil {
			panic(err.Error())
		}
		// Delete deployment
	} else if len(tailArgs) == 1 && tailArgs[0] == utils.DELETE_RESOURCE {
		if err := d.Delete(); err != nil {
			panic(err.Error())
		}
		// Invalid arguments
	} else {
		panic(fmt.Sprintf("Invalid arguments %s", tailArgs))
	}

	/* 	time.Sleep(10 * time.Second)

	   	if err := d.Delete(); err != nil {
	   		panic(err.Error())
	   	} */

	/*
		for {
			pods, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{})
			if err != nil {
				panic(err.Error())
			}
			fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

			// Examples for error handling:
			// - Use helper functions like e.g. errors.IsNotFound()
			// - And/or cast to StatusError and use its properties like e.g. ErrStatus.Message
			namespace := "default"
			pod := "vodpackager-75b7dcd56-s42q5"
			_, err = clientset.CoreV1().Pods(namespace).Get(pod, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				fmt.Printf("Pod %s in namespace %s not found\n", pod, namespace)
			} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
				fmt.Printf("Error getting pod %s in namespace %s: %v\n",
					pod, namespace, statusError.ErrStatus.Message)
			} else if err != nil {
				panic(err.Error())
			} else {
				fmt.Printf("Found pod %s in namespace %s\n", pod, namespace)
			}

			time.Sleep(10 * time.Second)
		} */
}
