package main

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Use clientset to create pods
	podClient := clientset.CoreV1().Pods("default")

	// Create a new pod object
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-1",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "container-1",
					Image: "nginx:1.14.2",
				},
			},
		},
	}

	// Create the new pod
	_, err = podClient.Create(context.Background(), pod1, metav1.CreateOptions{})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Create another new pod object
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-2",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "container-2",
					Image: "nginx:1.14.2",
				},
			},
		},
	}

	// Create the new pod
	_, err = podClient.Create(context.Background(), pod2, metav1.CreateOptions{})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Use clientset to create services
	serviceClient := clientset.CoreV1().Services("default")

	// Create a new service object
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-service",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "my-app",
			},
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Protocol: corev1.ProtocolTCP,
					Port:     80,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 80,
					},
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	// Create the new service
	_, err = serviceClient.Create(context.Background(), service, metav1.CreateOptions{})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
