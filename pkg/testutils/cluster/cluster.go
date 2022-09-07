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
	"time"

	kind "sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
)

// NodePort defines the mapping of a node port to host port
type NodePort struct {
	// NodePort to access in the cluster
	NodePort int
	// Port used in the host to access the node port
	HostPort int
}

// ClusterOptions defines options for customizing the cluster
type ClusterOptions struct {
	// cluster configuration to use. Overrides other options (NodePorts, Workers)
	Config string
	// List of images to pre-load on each node.
	// The images must be available locally (e.g. with docker pull <image>)
	Images []string
	// maximum time to wait for cluster creation
	Wait time.Duration
	// node ports to expose
	NodePorts []NodePort
	// number of worker nodes
	Workers int
}

// TestCluster an active test cluster
type TestCluster struct {
	// Name of the cluster
	name string
	//  path to the Kubeconfig
	kubeconfig string
	// kind cluster provider
	provider kind.Provider
}

const baseConfig = `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane`

const portConfig = `
  extraPortMappings:
  - containerPort: %d
    hostPort: %d
    listenAddress: "0.0.0.0"
    protocol: tcp`

// try to bind to host port to check availability
func checkHostPort(port int) error {
	l, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return fmt.Errorf("host port is not available %d", port)
	}
	l.Close()
	return nil
}

func buildClusterConfig(options ClusterOptions) (string, error) {
	if options.Config != "" {
		return options.Config, nil
	}

	var config strings.Builder
	config.WriteString(baseConfig)

	for _, np := range options.NodePorts {
		err := checkHostPort(np.HostPort)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&config, portConfig, np.NodePort, np.HostPort)
	}

	for i := 0; i < options.Workers; i++ {
		fmt.Fprintf(&config, "\n- role: worker")
	}

	return config.String(), nil
}

// loadImages loads the images in the list to all cluster nodes' local repositories
// TODO: check all images are available locally before creating the cluster
// TODO: add option for attempting to pull images before loading them
func loadImages(images []string, nodes []nodes.Node) error {
	imagesTar, err := ioutil.TempFile(os.TempDir(), "image*.tar")
	if err != nil {
		return err
	}
	defer os.Remove(imagesTar.Name())

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
		image.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

// CreateCluster creates a test cluster with the given name
func CreateCluster(name string, options ClusterOptions) (*TestCluster, error) {
	provider := kind.NewProvider()

	config, err := buildClusterConfig(options)
	if err != nil {
		return nil, err
	}

	kindOptions := []kind.CreateOption{
		kind.CreateWithNodeImage("kindest/node:v1.24.0"),
		kind.CreateWithRawConfig([]byte(config)),
	}

	if options.Wait > 0 {
		kindOptions = append(kindOptions, kind.CreateWithWaitForReady(options.Wait))
	}

	err = provider.Create(
		name,
		kindOptions...,
	)

	if err != nil {
		return nil, err
	}

	// pre-load images
	if len(options.Images) > 0 {
		nodes, err := provider.ListInternalNodes(name)
		if err != nil {
			return nil, err
		}
		err = loadImages(options.Images, nodes)
		if err != nil {
			return nil, err
		}
	}

	configPath := filepath.Join(os.TempDir(), name)
	err = provider.ExportKubeConfig(name, configPath, false)
	if err != nil {
		return nil, err
	}

	return &TestCluster{
		name:       name,
		kubeconfig: configPath,
		provider:   *provider,
	}, nil
}

// Delete deletes a test cluster
func (c *TestCluster) Delete() error {
	return c.provider.Delete(
		c.name,
		c.kubeconfig,
	)
}

// Kubeconfig returns the path to the kubeconfig for the test cluster
func (c *TestCluster) Kubeconfig() (string, error) {
	if c.kubeconfig == "" {
		return "", nil
	}

	return c.kubeconfig, nil
}
