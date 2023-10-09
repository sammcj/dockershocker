package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"golang.org/x/time/rate"
)

const (
	defaultIdleTimeout = time.Minute * 15
	autoShutdownLabel  = "dockershocker.enabled"
	idleTimeoutLabel   = "dockershocker.timeout_minutes"
	rateLimit          = 5  // Five requests per second
	burstLimit         = 10 // Allow burst of 10 requests
)

var (
	dockerClient      *client.Client
	dockerSocket			string
	lastAccessed      = make(map[string]time.Time)
	lastAccessedMutex = &sync.Mutex{} // Mutex for lastAccessed
	limiter           = rate.NewLimiter(rateLimit, burstLimit)
	tmpl              *template.Template
	logLevel          string
	port              string
)

type ContainerInfo struct {
	Name       string
	Status     string
	LastAccess time.Time
	Timeout    time.Duration
}

func init() {
	flag.StringVar(&logLevel, "logLevel", "info", "Set log level: debug, info, warn, error")
	flag.StringVar(&port, "port", "8080", "Set the port to listen on")
	flag.StringVar(&dockerSocket, "dockerSocket", "tcp://dockerproxy:2375", "Set the path to the docker socket (unix://path or tcp://host:port)")
	flag.Parse()

	var err error
	dockerClient, err = client.NewClientWithOpts(client.WithHost(dockerSocket), client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Error initializing Docker client: %s", err)
	}

	tmpl = template.Must(template.New("").Parse(`
		<h1>Managed Containers</h1>
		<table border="1">
			<tr>
				<th>Name</th>
				<th>Status</th>
				<th>Last Access</th>
				<th>Timeout</th>
			</tr>
			{{ range . }}
				<tr>
					<td>{{ .Name }}</td>
					<td>{{ .Status }}</td>
					<td>{{ .LastAccess }}</td>
					<td>{{ .Timeout }}</td>
				</tr>
			{{ end }}
		</table>
	`))
}

func main() {
	if logLevel == "debug" {
		log.Println("Debug logging enabled")
	}

	go monitorContainers()

	http.HandleFunc("/", rateLimitMiddleware(traefikMiddleware))
	http.HandleFunc("/health", rateLimitMiddleware(healthCheckHandler))
	http.HandleFunc("/containers", rateLimitMiddleware(showContainersHandler))

	log.Println("Server started at: " + port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %s", err)
	}
}

func rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	_, err := dockerClient.Ping(context.Background())
	if err != nil {
		http.Error(w, "Failed to connect to Docker API", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Healthy"))
}

func showContainersHandler(w http.ResponseWriter, r *http.Request) {
	containers, err := dockerClient.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		http.Error(w, "Failed to fetch containers", http.StatusInternalServerError)
		return
	}

	var managedContainers []ContainerInfo
	for _, container := range containers {
		if _, ok := container.Labels[autoShutdownLabel]; ok && container.Labels[autoShutdownLabel] == "true" {
			timeout := getContainerTimeout(container)
			info := ContainerInfo{
				Name:       container.Names[0],
				Status:     container.Status,
				LastAccess: lastAccessed[container.ID],
				Timeout:    timeout,
			}
			managedContainers = append(managedContainers, info)
		}
	}

	tmpl.Execute(w, managedContainers)
}

func monitorContainers() {
	for {
		containers, err := dockerClient.ContainerList(context.Background(), types.ContainerListOptions{})
		if logLevel == "debug" {
			log.Printf("Monitoring containers...")
		}
		if err != nil {
			log.Printf("Error fetching containers: %s", err)
			// wait before trying again
			time.Sleep(time.Second * 2)
			continue
		}

		for _, container := range containers {
			if _, ok := container.Labels[autoShutdownLabel]; ok && container.Labels[autoShutdownLabel] == "true" {
				timeout := getContainerTimeout(container)
				lastAccessedMutex.Lock()
				if time.Since(lastAccessed[container.ID]) > timeout {
					noWaitTimeout := 0
					if err := dockerClient.ContainerStop(context.Background(), container.ID, containertypes.StopOptions{Timeout: &noWaitTimeout}); err != nil {
						log.Printf("Error stopping container %s: %s", container.ID, err)
					} else {
						log.Printf("Stopped container %s due to inactivity", container.ID)
						delete(lastAccessed, container.ID)
					}
				}
				lastAccessedMutex.Unlock()
			}
		}
		time.Sleep(time.Minute)
	}
}

func traefikMiddleware(w http.ResponseWriter, r *http.Request) {
	host := r.Host

	if logLevel == "debug" {
		log.Printf("Received request for host: %s", r.Host)
	}

	containers, err := dockerClient.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		http.Error(w, "Error fetching containers", http.StatusInternalServerError)
		return
	}

	var targetContainer *types.Container
	for _, container := range containers {
		for _, network := range container.NetworkSettings.Networks {
			if network.Aliases != nil {
				for _, alias := range network.Aliases {
					if alias == host {
						targetContainer = &container
						break
					}
				}
			}
			if targetContainer != nil {
				break
			}
		}
	}

	if targetContainer == nil {
		http.Error(w, "No container matched the requested host", http.StatusNotFound)
		return
	}

	if targetContainer.State != "running" {
		if _, ok := targetContainer.Labels[autoShutdownLabel]; ok && targetContainer.Labels[autoShutdownLabel] == "true" {
			err := dockerClient.ContainerStart(context.Background(), targetContainer.ID, types.ContainerStartOptions{})
			if err != nil {
				http.Error(w, fmt.Sprintf("Error starting container for host %s: %s", host, err), http.StatusInternalServerError)
				return
			}
			log.Printf("Started container %s due to incoming request for host %s", targetContainer.ID, host)
		}
	}

	lastAccessedMutex.Lock()
	lastAccessed[targetContainer.ID] = time.Now()
	lastAccessedMutex.Unlock()
	http.Redirect(w, r, r.URL.String(), http.StatusSeeOther)
}

func getContainerTimeout(container types.Container) time.Duration {
	timeout := defaultIdleTimeout
	if timeoutVal, ok := container.Labels[idleTimeoutLabel]; ok {
		minutes, err := strconv.Atoi(timeoutVal)
		if err == nil {
			timeout = time.Minute * time.Duration(minutes)
		}
	}
	return timeout
}
