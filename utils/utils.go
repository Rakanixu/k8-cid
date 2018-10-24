package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

const CREATE_RESOURCE = "create"
const DELETE_RESOURCE = "delete"
const K8sCidWorkingDir = "/.k8s-cid"

func HomeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func K8sCidDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h + K8sCidWorkingDir
	}
	return os.Getenv("USERPROFILE") + K8sCidWorkingDir // windows
}

func CreateDirIfNotExist(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0777)
		if err != nil {
			fmt.Println(err)
			panic(err)
		}
	}
}

func RepositoriesComponentConfigPath() string {
	return HomeDir() + K8sCidWorkingDir + "/repositories-components.json"
}

func ReadRepos() map[string][]string {
	var m map[string][]string

	a, err := ioutil.ReadFile(RepositoriesComponentConfigPath())
	if err != nil {
		panic(err)
	}

	if err := json.Unmarshal(a, &m); err != nil {
		panic(err)
	}

	return m
}

func Int32Ptr(i int32) *int32 {
	return &i
}

func Find(a []string, x string) int {
	for i, n := range a {
		if x == n {
			return i
		}
	}
	return -1
}
