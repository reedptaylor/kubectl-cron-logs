package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strings"
	"sync"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1typed "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	var wg sync.WaitGroup
	flags := setFlags(os.Args)

	clientset := configureClusterClient(&flags.Namespace)
	coreapi := clientset.CoreV1()
	batchapi := clientset.BatchV1()

	cronjob, err := batchapi.CronJobs(flags.Namespace).Get(context.TODO(), flags.Name, metav1.GetOptions{})
	if err != nil {
		log.Fatal(err)
	}

	jobs, err := batchapi.Jobs(flags.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Fatal(err)
	}
	for _, job := range jobs.Items {
		if !isOwnedByCronJob(job, *cronjob) {
			continue
		}

		wg.Add(1)
		go processJob(coreapi, job, &wg, flags)
	}

	wg.Wait()
}

func setFlags(args []string) CmdFlags {
	var flags CmdFlags

	index := 1
	for index < len(args) {
		arg := args[index]
		switch arg {
		case "-h", "--help":
			fmt.Println("Usage: coming soon")
			os.Exit(0)

		case "-v", "--version":
			fmt.Println("Version: 0.0.1")
			os.Exit(0)

		case "-n", "--namespace":
			index++
			if index >= len(args) || args[index][0] == '-' {
				log.Fatal("namespace is required with -n or --namespace")
			}
			flags.Namespace = args[index]

		case "-c", "--container":
			index++
			if index >= len(args) || args[index][0] == '-' {
				log.Fatal("container name is required with -c or --container")
			}
			flags.Container = args[index]

		case "-f", "--follow":
			flags.Follow = true

		case "--timestamps":
			flags.Showtimestamps = true

		default:
			flags.Name = arg
		}
		index++
	}

	if flags.Name == "" {
		log.Fatal("cronjob name is required")
	}

	return flags
}

func configureClusterClient(namespace *string) *kubernetes.Clientset {
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)
	config, err := kubeconfig.ClientConfig()
	if err != nil {
		log.Fatal(err)
	}

	if *namespace == "" {
		confignamespace, _, err := kubeconfig.Namespace()
		if err != nil {
			confignamespace = "default"
		}
		*namespace = confignamespace
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	return clientset
}

func isOwnedByCronJob(job batchv1.Job, cronjob batchv1.CronJob) bool {
	isowned := false
	for _, owner := range job.OwnerReferences {
		if owner.UID == cronjob.UID && *owner.Controller {
			isowned = true
			break
		}
	}
	return isowned
}

func processJob(coreapi corev1typed.CoreV1Interface, job batchv1.Job, wg *sync.WaitGroup, flags CmdFlags) {
	defer wg.Done()

	pods, err := coreapi.Pods(flags.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "batch.kubernetes.io/controller-uid=" + string(job.UID),
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, pod := range pods.Items {
		wg.Add(1)
		go processPod(coreapi, pod, wg, flags)
	}
}

func processPod(coreapi corev1typed.CoreV1Interface, pod corev1.Pod, wg *sync.WaitGroup, flags CmdFlags) {
	defer wg.Done()

	logs, err := coreapi.Pods(flags.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
		Container:  flags.Container,
		Timestamps: flags.Showtimestamps,
		Follow: flags.Follow,
	}).Stream(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
	defer logs.Close()

	printLogs(pod.Name, logs)
}

func printLogs(podname string, logs io.ReadCloser) {
	for {
		buf := make([]byte, 1024)
		bytesread, err := logs.Read(buf)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			log.Fatal(err)
		}

		if bytesread > 0 {
			lines := strings.Split(string(buf), "\n")

			for _, line := range lines {
				line = strings.Trim(line, "\x00")
				if line != "" {
					printPodName(podname)
					fmt.Printf("\033[0;0m%s\n", line)
				}
			}
		}
	}
}

func printPodName(podname string) {
	namesum := 0.0
	for _, char := range podname {
		namesum += (math.Pow(float64(char), 2) / 2) + 1
	}
	color := (int(namesum) % 6) + 31
	fmt.Printf("\033[1;%dm%s\033[0;1m ", color, podname)
}

type CmdFlags struct {
	Name           string
	Namespace      string
	Container      string
	Follow         bool
	Showtimestamps bool
}
