package main

import (
	"fmt"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"github.com/mackerelio/go-osstat/cpu"
	"github.com/mackerelio/go-osstat/memory"
)

var sleepTime = 3
var maxConcurrency = 50

// Timeout if something rashes
var workerTimeoutEnv = os.Getenv("SHUFFLE_ORBORUS_EXECUTION_TIMEOUT")
var concurrencyEnv = os.Getenv("SHUFFLE_ORBORUS_EXECUTION_CONCURRENCY")
var appSdkVersion = os.Getenv("SHUFFLE_APP_SDK_VERSION")
var workerVersion = os.Getenv("SHUFFLE_WORKER_VERSION")
var newWorkerImage = os.Getenv("SHUFFLE_WORKER_IMAGE")

var baseimagename = os.Getenv("SHUFFLE_BASE_IMAGE_NAME")
var baseimageregistry = os.Getenv("SHUFFLE_BASE_IMAGE_REGISTRY")

func getThisContainerId() {
	fCol := ""

	// some adjusting based on current running mode
	switch runningMode {
	case "kubernetes":
		// cgroup will be like:
		// 11:net_cls,net_prio:/kubepods/besteffort/podf132b44d-cfcf-43f7-9906-79f58e268333/851466f8b5ed5aa0f265b1c95c6d2bafbc51a38dd5c5a1621b6e586572150009
		fCol = "5"
		log.Printf("[INFO] Running containerized in Kubernetes!")

	case "docker":
		// cgroup will be like:
		// 12:perf_event:/docker/0f06810364f52a2cd6e80bfba27419cb8a29758a204cd676388f4913bb366f2b
		fCol = "3"
		log.Printf("[INFO] Running containerized in Docker!")

	default:
		fCol = "3" // for backward-compatibility with production
		log.Printf("[WARNING] RUNNING_MODE not set - defaulting to Docker (NOT Kubernetes).")
	}
	if fCol != "" {
			cmd := fmt.Sprintf("cat /proc/self/cgroup | grep memory | tail -1 | cut -d/ -f%s | grep -o -E '[0-9A-z]{64}'", fCol)
			out, err := exec.Command("bash", "-c", cmd).Output()
			if err == nil {
				containerId = strings.TrimSpace(string(out))
				log.Printf("[DEBUG] Set containerId network to %s", containerId)

				// cgroup error. Use fallback strategy below.
				// https://github.com/moby/moby/issues/7015
				//log.Printf("Checking if %s is in %s", ".scope", string(out))
				if strings.Contains(string(out), ".scope") {
					log.Printf("[DEBUG] ContainerId contains scope. setting to empty.")
					containerId = ""
					//docker-76c537e9a4b7c7233011f5d70e6b7f2d600b6413ac58a96519b8dca7a3f7117a.scope
				}
			} else {
				log.Printf("[WARNING] Failed getting container ID: %s", err)
			}
		}

		if containerId == "" {
		if containerName != "" {
			containerId = containerName
			log.Printf("[INFO] Falling back to CONTAINER_NAME as container ID")
		} else {
			containerId = "shuffle-orborus"
			log.Printf(`[WARNING] CONTAINER_NAME is not set. Falling back to default name "%s" as container ID`, containerId)
		}
	}

	log.Printf(`[INFO] Started with containerId "%s"`, containerId)
}

func cleanupExistingNodes(ctx context.Context) error {
	serviceListOptions := types.ServiceListOptions{}
	services, err := dockercli.ServiceList(
		context.Background(),
		serviceListOptions,
	)

	if err != nil {
		log.Printf("[DEBUG] Failed finding containers: %s", err)
		return err
	}

	//log.Printf("\n\nFound %d contaienrs", len(services))

	for _, service := range services {
	
		if strings.Contains(service.Spec.Annotations.Name, "opensearch") {
			continue
		}

		if strings.Contains(service.Spec.TaskTemplate.ContainerSpec.Image, "shuffle") {

			if !strings.Contains(service.Spec.TaskTemplate.ContainerSpec.Image, "shuffle-frontend") &&
				!strings.Contains(service.Spec.TaskTemplate.ContainerSpec.Image, "shuffle-backend") &&
				!strings.Contains(service.Spec.TaskTemplate.ContainerSpec.Image, "shuffle-orborus") {

				err = dockercli.ServiceRemove(ctx, service.ID)
				if err != nil {
					log.Printf("[DEBUG] Failed to remove service %s", service.Spec.Annotations.Name)
				} else {
					log.Printf("[DEBUG] Removed service %#v", service.Spec.TaskTemplate.ContainerSpec.Image)
				}
			}
		}
	}

	return nil
}


func main() {
// Load the Kubernetes configuration from file    
	config, err := clientcmd.BuildConfigFromFlags("", "/path/to/kubeconfig")
    if err != nil {
		panic(err.Error())
	}

// Create a client to communicate with the Kubernetes API
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}


// Define the deployment object
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "myapp",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "myapp",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
					"app": "myapp",
				},
					},
			Spec: corev1.PodSpec{
			Containers: []corev1.Container{
						{
							Name: "myapp",
							Image: "myapp:latest",
							Command: []string{
								"/bin/myapp",
						},
							Args: []string{
								"--config=/etc/myapp/config.yaml",
						},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8080,
								},
							},
						},
					},
				},
			},
		},
	};
