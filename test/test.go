package main

/*
	Orborus exists to listen for new workflow executions whcih are deployed as workers.
*/

// FIXME:
// 2022/01/12 17:13:36 [WARNING] Swarm init: Error response from daemon: manager stopped: failed to listen on remote API address: listen tcp: address tcp/2377%!(EXTRA string=172.23.0.2): unknown port

// frikky@debian:~/git/shuffle/functions/onprem/worker$ docker service create --replicas 5 --name shuffle-workers --env SHUFFLE_SWARM_CONFIG=run --publish published=33333,target=33333 ghcr.io/frikky/shuffle-worker:nightly

//  Potential issues:
// Default network could be same as on the host
// Ingress network may not exist (default)

import (
	"github.com/shuffle/shuffle-shared"

	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"

	//"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
	uuid "github.com/satori/go.uuid"

	//network "github.com/docker/docker/api/types/network"
	//natting "github.com/docker/go-connections/nat"
	"github.com/mackerelio/go-osstat/cpu"
	"github.com/mackerelio/go-osstat/memory"
)

// Starts jobs in bulk, so this could be increased
var sleepTime = 3
var maxConcurrency = 50

// Timeout if something rashes
var workerTimeoutEnv = os.Getenv("SHUFFLE_ORBORUS_EXECUTION_TIMEOUT")
var concurrencyEnv = os.Getenv("SHUFFLE_ORBORUS_EXECUTION_CONCURRENCY")
var appSdkVersion = os.Getenv("SHUFFLE_APP_SDK_VERSION")
var workerVersion = os.Getenv("SHUFFLE_WORKER_VERSION")
var newWorkerImage = os.Getenv("SHUFFLE_WORKER_IMAGE")

//var baseimagename = "docker.pkg.github.com/frikky/shuffle"
//var baseimagename = "ghcr.io/frikky"
// var baseimagename = "frikky/shuffle"
var baseimagename = os.Getenv("SHUFFLE_BASE_IMAGE_NAME")
var baseimageregistry = os.Getenv("SHUFFLE_BASE_IMAGE_REGISTRY")
var baseimagetagsuffix = os.Getenv("SHUFFLE_BASE_IMAGE_TAG_SUFFIX")

// Used for cloud with auth
var auth = os.Getenv("AUTH")
var org = os.Getenv("ORG")

//var orgId = os.Getenv("ORG_ID")
var baseUrl = os.Getenv("BASE_URL")
var environment = os.Getenv("ENVIRONMENT_NAME")
var dockerApiVersion = os.Getenv("DOCKER_API_VERSION")
var runningMode = strings.ToLower(os.Getenv("RUNNING_MODE"))
var cleanupEnv = strings.ToLower(os.Getenv("CLEANUP"))
var timezone = os.Getenv("TZ")
var containerName = os.Getenv("ORBORUS_CONTAINER_NAME")
var swarmConfig = os.Getenv("SHUFFLE_SWARM_CONFIG")
var swarmNetworkName = os.Getenv("SHUFFLE_SWARM_NETWORK_NAME")
var executionIds = []string{}

var dockercli *dockerclient.Client
var containerId string

func init() {
	var err error

	dockercli, err = dockerclient.NewEnvClient()
	if err != nil {
		log.Printf("Unable to create docker client: %s", err)
	}

	getThisContainerId()
}

// form id of current running container
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
		//log.Printf("[INFO] Service: %#v", service.Spec.Annotations.Name)

		//portFound := false
		//for _, endpoint := range service.Spec.EndpointSpec.Ports {
		//	if strings.Contains(endpoint.Name, "port") {
		//		//portFound = true
		//	}
		//}

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
