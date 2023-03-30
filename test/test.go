package main

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// Load the Kubernetes configuration from file
	config, err := clientcmd.BuildConfigFromFlags("", "C:/Shuffle-Git/test/test.yml")
	if err != nil {
		panic(err)
	}

	// Create a Kubernetes client
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	// Define the Pod spec
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "example-pod",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "container-1",
					Image: "nginx",
				},
			},
		},
	}

	// Create the Pod
	fmt.Println("Creating Pod...")
	createdPod, err := clientset.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Pod %s created\n", createdPod.GetName())

	// Wait for a few seconds
	time.Sleep(5 * time.Second)

	// Delete the Pod
	fmt.Println("Deleting Pod...")
	err = clientset.CoreV1().Pods("default").Delete(context.Background(), createdPod.GetName(), metav1.DeleteOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Pod %s deleted\n", createdPod.GetName())
}
