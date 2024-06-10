package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"

	azclient "github.com/gardener/remedy-controller/pkg/client/azure"
)

// NOTE: This simulator is a work in progress and does not (yet) reproduce the issue. More investigation is needed.

func main() {
	var vmName string
	if len(os.Args) > 1 {
		vmName = os.Args[1]
	}
	if len(vmName) == 0 {
		fmt.Printf("Error: VM name not specified\n")
		os.Exit(1)
	}
	err := simulate(vmName)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func simulate(vmName string) error {
	// Read Azure credentials
	credentials, err := azclient.ReadConfig("dev/credentials.yaml")
	if err != nil {
		return err
	}

	// Create OAuth config
	oauthConfig, err := adal.NewOAuthConfig(azure.PublicCloud.ActiveDirectoryEndpoint, credentials.TenantID)
	if err != nil {
		return err
	}

	// Create service principal token
	servicePrincipalToken, err := adal.NewServicePrincipalToken(*oauthConfig, credentials.ClientID, credentials.ClientSecret, azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		return err
	}
	authorizer := autorest.NewBearerAuthorizer(servicePrincipalToken)

	// Create clients
	vmClient := compute.NewVirtualMachinesClient(credentials.SubscriptionID)
	vmClient.Authorizer = authorizer
	diskClient := compute.NewDisksClient(credentials.SubscriptionID)
	diskClient.Authorizer = authorizer

	// Get Kubernetes config
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		return err
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}

	// Create context
	ctx := context.TODO()

	// Create PVC
	fmt.Printf("Creating PVC\n")
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dnfsim",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse("2Gi"),
				},
			},
			StorageClassName: pointer.String("managed-standard-ssd"),
		},
	}
	_, err = clientset.CoreV1().PersistentVolumeClaims("default").Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// Create pod
	fmt.Printf("Creating pod\n")
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dnfsim",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx",
					Ports: []corev1.ContainerPort{
						{
							Name:          "http",
							ContainerPort: 80,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "dnfsim",
							MountPath: "/usr/share/nginx/html",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "dnfsim",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "dnfsim",
						},
					},
				},
			},
		},
	}
	pod, err = clientset.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// Wait until pod is running
	err = retry(60, 6*time.Second, func() error {
		pod, err = clientset.CoreV1().Pods("default").Get(ctx, "dnfsim", metav1.GetOptions{})
		if err != nil {
			return err
		}
		if pod.Status.Phase == corev1.PodPending {
			return fmt.Errorf("pod is still pending")
		}
		return nil
	})
	if err != nil {
		return err
	}
	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("pod status: %v", pod.Status.Phase)
	}

	// Get VM
	fmt.Printf("Getting VM %s\n", vmName)
	vm, err := vmClient.Get(ctx, credentials.ResourceGroup, vmName, compute.InstanceView)
	if err != nil {
		return err
	}
	fmt.Printf("Disks: %+v\n", vm.StorageProfile.DataDisks)

	// Detach disk
	fmt.Printf("Detaching disks\n")
	disk := (*vm.StorageProfile.DataDisks)[0]
	vm.StorageProfile.DataDisks = &[]compute.DataDisk{}
	future, err := vmClient.CreateOrUpdate(ctx, credentials.ResourceGroup, vmName, vm)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, vmClient.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(vmClient)
	if err != nil {
		return err
	}

	// Get VM
	fmt.Printf("Getting VM %s\n", vmName)
	vm, err = vmClient.Get(ctx, credentials.ResourceGroup, vmName, compute.InstanceView)
	if err != nil {
		return err
	}
	fmt.Printf("Disks: %+v\n", vm.StorageProfile.DataDisks)

	errs, ctxx := errgroup.WithContext(ctx)

	errs.Go(func() error {
		// Reattach disk
		fmt.Printf("Reattaching disk\n")
		vm.StorageProfile.DataDisks = &[]compute.DataDisk{
			disk,
		}
		future1, err1 := vmClient.CreateOrUpdate(ctxx, credentials.ResourceGroup, vmName, vm)
		if err1 != nil {
			return err1
		}
		err1 = future1.WaitForCompletionRef(ctxx, vmClient.Client)
		if err1 != nil {
			return err1
		}
		_, err1 = future1.Result(vmClient)
		if err1 != nil {
			return err1
		}

		return nil
	})

	errs.Go(func() error {
		// Wait between 0.5s and 1.5s
		ms := rand.Intn(1001) + 500
		fmt.Printf("Waiting for %v milliseconds\n", ms)
		time.Sleep(time.Duration(ms) * time.Millisecond)

		// Delete disk
		fmt.Printf("Deleting disk\n")
		future2, err2 := diskClient.Delete(ctxx, credentials.ResourceGroup, *disk.Name)
		if err2 != nil {
			return err2
		}
		err2 = future2.WaitForCompletionRef(ctxx, diskClient.Client)
		if err2 != nil {
			return err2
		}
		_, err2 = future2.Result(diskClient)
		if err2 != nil {
			return err2
		}

		return nil
	})

	err = errs.Wait()
	if err != nil {
		return err
	}

	// Get VM
	fmt.Printf("Getting VM %s\n", vmName)
	vm, err = vmClient.Get(ctx, credentials.ResourceGroup, vmName, compute.InstanceView)
	if err != nil {
		return err
	}
	fmt.Printf("Disks: %+v\n", vm.StorageProfile.DataDisks)

	return nil
}

func retry(n int, t time.Duration, f func() error) error {
	var err error
	for i := 0; i < n; i++ {
		err = f()
		if err == nil {
			return nil
		}
		time.Sleep(t)
	}
	return fmt.Errorf("timed out: %w", err)
}
