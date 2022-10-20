// Package cluster implements helpers for creating test clusters using
// [kind] as a library.
// The helpers facilitate some common customizations of clusters, such as
// creating multiple worker nodes or exposing a node port to facilitate the
// access to services deployed in the cluster using NodePort services.
// Other customizations can be achieved by providing a valid [kind configuration].
//
// [kind]: https://github.com/kubernetes-sigs/kind
// [kind configuration]: https://kind.sigs.k8s.io/docs/user/configuration
package cluster

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	kind "sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
)

const baseKindConfig = `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane`

const kindPortMapping = `
  - containerPort: %d
    hostPort: %d
    listenAddress: "0.0.0.0"
    protocol: tcp`

// NodePort defines the mapping of a node port to host port
type NodePort struct {
	// NodePort to access in the cluster
	NodePort int32
	// Port used in the host to access the node port
	HostPort int32
}

// Options defines options for customizing the cluster
type Options struct {
	// cluster configuration to use. Overrides other options (NodePorts, Workers)
	Config string
	// List of images to pre-load on each node.
	// The images must be available locally (e.g. with docker pull <image>)
	Images []string
	// maximum time to wait for cluster creation.
	Wait time.Duration
	// node ports to expose
	NodePorts []NodePort
	// number of worker nodes
	Workers int
}

// Config contains the configuration for creating a cluster
type Config struct {
	// name of the cluster
	name string
	// options for creating cluster
	options Options
}

// DefaultConfig creates a ClusterConfig with default options and
// default name "test-cluster"
func DefaultConfig() (*Config, error) {
	return NewConfig("test-cluster", Options{})
}

// NewConfig creates a ClusterConfig with the ClusterOptions
func NewConfig(name string, options Options) (*Config, error) {
	if name == "" {
		return nil, fmt.Errorf("cluster name is mandatory")
	}

	for _, np := range options.NodePorts {
		if np.HostPort == 0 || np.NodePort == 0 {
			return nil, fmt.Errorf("node port and host port are required in a NodePort")
		}
	}

	return &Config{
		name:    name,
		options: options,
	}, nil
}

// Render returns the Kind configuration for creating a cluster
// with this ClusterConfig
func (c *Config) Render() (string, error) {
	if c.options.Config != "" {
		return c.options.Config, nil
	}

	var config strings.Builder
	config.WriteString(baseKindConfig)
	if len(c.options.NodePorts) > 0 {
		// create section for port mappings in master node and add each mapped port
		fmt.Fprint(&config, "\n  extraPortMappings:")
		for _, np := range c.options.NodePorts {
			fmt.Fprintf(&config, kindPortMapping, np.NodePort, np.HostPort)
		}
	}

	for i := 0; i < c.options.Workers; i++ {
		fmt.Fprintf(&config, "\n- role: worker")
	}

	return config.String(), nil
}

// Cluster an active test cluster
type Cluster struct {
	// configuration used for creating the cluster
	config *Config
	//  path to the Kubeconfig
	kubeconfig string
	// kind cluster provider
	provider kind.Provider
	// mutex for concurrent modifications to cluster
	mtx sync.Mutex
	// name of the cluster
	name string
	// allocated node ports
	allocatedPorts map[NodePort]bool
	// available node ports exposed by the cluster
	availablePorts []NodePort
}

// try to bind to host port to check availability
func checkHostPort(port int32) error {
	l, err := net.Listen("tcp", ":"+strconv.Itoa(int(port)))
	if err != nil {
		return fmt.Errorf("host port is not available %d", port)
	}
	// ignore error
	_ = l.Close()

	return nil
}

// loadImages loads the images in the list to all cluster nodes' local repositories
// TODO: check all images are available locally before creating the cluster
// TODO: add option for attempting to pull images before loading them
func loadImages(images []string, nodes []nodes.Node) error {
	imagesTar, err := ioutil.TempFile(os.TempDir(), "image*.tar")
	if err != nil {
		return err
	}

	defer func() {
		// ignore error. Nothing to do if cannot remove image
		_ = os.Remove(imagesTar.Name())
	}()

	// save the images to a tar
	saveCmd := append([]string{"save", "-o", imagesTar.Name()}, images...)
	output, err := exec.Command("docker", saveCmd...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error saving images: %s", string(output))
	}

	// load the image on each node of the cluster
	for _, n := range nodes {
		image, err := os.Open(imagesTar.Name())
		if err != nil {
			return err
		}
		err = nodeutils.LoadImageArchive(n, image)
		// ignore error. Nothing to do if cannot close file
		_ = image.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

// Create creates a test cluster with the given name
func (c *Config) Create() (*Cluster, error) {
	// before creating cluster check host ports are available
	// to avoid weird kind error creating cluster
	ports := []NodePort{}
	for _, np := range c.options.NodePorts {
		err := checkHostPort(np.HostPort)
		if err != nil {
			return nil, err
		}
		ports = append(ports, np)
	}

	provider := kind.NewProvider()

	config, err := c.Render()
	if err != nil {
		return nil, err
	}

	kindOptions := []kind.CreateOption{
		kind.CreateWithNodeImage("kindest/node:v1.24.0"),
		kind.CreateWithRawConfig([]byte(config)),
	}

	if c.options.Wait > 0 {
		kindOptions = append(kindOptions, kind.CreateWithWaitForReady(c.options.Wait))
	}

	err = provider.Create(
		c.name,
		kindOptions...,
	)

	if err != nil {
		return nil, err
	}

	// pre-load images
	if len(c.options.Images) > 0 {
		nodes, err := provider.ListInternalNodes(c.name)
		if err != nil {
			return nil, err
		}
		err = loadImages(c.options.Images, nodes)
		if err != nil {
			return nil, err
		}
	}

	configPath := filepath.Join(os.TempDir(), c.name)
	err = provider.ExportKubeConfig(c.name, configPath, false)
	if err != nil {
		return nil, err
	}

	return &Cluster{
		name:           c.name,
		config:         c,
		kubeconfig:     configPath,
		provider:       *provider,
		allocatedPorts: map[NodePort]bool{},
		availablePorts: ports,
	}, nil
}

// Delete deletes a test cluster
func (c *Cluster) Delete() error {
	return c.provider.Delete(
		c.name,
		c.kubeconfig,
	)
}

// Kubeconfig returns the path to the kubeconfig for the test cluster
func (c *Cluster) Kubeconfig() string {
	return c.kubeconfig
}

// AllocatePort reserves a port from the pool of ports exposed by the cluster
// to ensure it is not been used by other service
func (c *Cluster) AllocatePort() NodePort {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if len(c.availablePorts) == 0 {
		return NodePort{}
	}

	port := c.availablePorts[0]
	c.availablePorts = c.availablePorts[1:]
	c.allocatedPorts[port] = true
	return port
}

// ReleasePort makes available a Port previously allocated by AllocatePort
func (c *Cluster) ReleasePort(p NodePort) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	_, assigned := c.allocatedPorts[p]
	if assigned {
		delete(c.allocatedPorts, p)
		c.availablePorts = append(c.availablePorts, p)
	}
}
