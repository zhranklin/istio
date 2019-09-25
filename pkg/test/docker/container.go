// Copyright 2019 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/docker/docker/api/types"
	dockerContainer "github.com/docker/docker/api/types/container"
	dockerNetwork "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/hashicorp/go-multierror"

	"istio.io/istio/pkg/test/scopes"
)

type ContainerPort int
type HostPort int
type PortMap map[ContainerPort]HostPort

func (m PortMap) toNatPortMap() nat.PortMap {
	out := make(nat.PortMap)
	for k, v := range m {
		out[toNatPort(k)] = []nat.PortBinding{{HostPort: strconv.Itoa(int(v))}}
	}
	return out
}

// ContainerConfig for a Container.
type ContainerConfig struct {
	Name       string
	Image      Image
	Aliases    []string
	PortMap    PortMap
	EntryPoint []string
	Cmd        []string
	Env        []string
	Hostname   string
	CapAdd     []string
	Privileged bool
	ExtraHosts []string
	Network    *Network
	Labels     map[string]string
}

var _ io.Closer = &Container{}

// Container is a wrapper around a Docker container.
type Container struct {
	ContainerConfig

	IPAddress string

	dockerClient *client.Client
	id           string
}

// NewContainer creates and starts a new Container instance.
func NewContainer(dockerClient *client.Client, config ContainerConfig) (*Container, error) {
	if config.Network == nil {
		return nil, fmt.Errorf("container must be associated with a network")
	}
	networkName := config.Network.Name

	scopes.CI.Infof("Creating Docker container for image %s in network %s", config.Image, networkName)
	exposedPorts := make(nat.PortSet)
	for k := range config.PortMap {
		exposedPorts[toNatPort(k)] = struct{}{}
	}

	resp, err := dockerClient.ContainerCreate(context.Background(),
		&dockerContainer.Config{
			Hostname:     config.Hostname,
			Image:        config.Image.String(),
			AttachStderr: true,
			AttachStdout: true,
			ExposedPorts: exposedPorts,
			Entrypoint:   config.EntryPoint,
			Cmd:          config.Cmd,
			Env:          config.Env,
			Labels:       config.Labels,
		},
		&dockerContainer.HostConfig{
			PortBindings: config.PortMap.toNatPortMap(),
			CapAdd:       config.CapAdd,
			Privileged:   config.Privileged,
			ExtraHosts:   config.ExtraHosts,
		},
		&dockerNetwork.NetworkingConfig{
			EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
				networkName: {
					Aliases: config.Aliases,
				},
			},
		},
		config.Name)
	if err != nil {
		return nil, err
	}

	c := &Container{
		dockerClient:    dockerClient,
		ContainerConfig: config,
		id:              resp.ID,
	}

	if err := dockerClient.ContainerStart(context.Background(), resp.ID, types.ContainerStartOptions{}); err != nil {
		_ = c.Close()
		return nil, err
	}

	iresp, err := dockerClient.ContainerInspect(context.Background(), resp.ID)
	if err != nil {
		_ = c.Close()
		return nil, err
	}

	c.IPAddress = iresp.NetworkSettings.Networks[networkName].IPAddress

	scopes.CI.Infof("Docker container %s (image=%s) created in network %s", resp.ID, config.Image, networkName)
	return c, nil
}

type ExecResult struct {
	StdOut   []byte
	StdErr   []byte
	ExitCode int
}

// Exec runs the given command on this container.
func (c *Container) Exec(ctx context.Context, cmd ...string) (ExecResult, error) {
	// prepare exec
	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	}
	cresp, err := c.dockerClient.ContainerExecCreate(ctx, c.id, execConfig)
	if err != nil {
		return ExecResult{}, err
	}
	execID := cresp.ID

	// run it, with stdout/stderr attached
	aresp, err := c.dockerClient.ContainerExecAttach(ctx, execID, types.ExecConfig{})
	if err != nil {
		return ExecResult{}, err
	}
	defer aresp.Close()

	// read the output
	var stdout, stderr bytes.Buffer
	outputDone := make(chan error, 1)

	go func() {
		// StdCopy demultiplexes the stream into two buffers
		_, err = stdcopy.StdCopy(&stdout, &stderr, aresp.Reader)
		outputDone <- err
	}()

	select {
	case err := <-outputDone:
		if err != nil {
			return ExecResult{}, err
		}
	case <-ctx.Done():
		return ExecResult{}, ctx.Err()
	}

	// get the exit code
	iresp, err := c.dockerClient.ContainerExecInspect(ctx, execID)
	if err != nil {
		return ExecResult{}, err
	}

	return ExecResult{
		ExitCode: iresp.ExitCode,
		StdOut:   stdout.Bytes(),
		StdErr:   stderr.Bytes(),
	}, nil
}

func (c *Container) Logs() (string, error) {
	r, err := c.dockerClient.ContainerLogs(context.Background(), c.id, types.ContainerLogsOptions{
		ShowStderr: true,
		ShowStdout: true,
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = r.Close() }()

	// Read stdout and stderr to the same buffer.
	var allOutput bytes.Buffer
	if _, err = stdcopy.StdCopy(&allOutput, &allOutput, r); err != nil {
		return "", err
	}

	return allOutput.String(), nil
}

// Close stops and removes this container.
func (c *Container) Close() error {
	scopes.CI.Infof("Closing Docker container %s", c.id)
	err := c.dockerClient.ContainerStop(context.Background(), c.id, nil)
	return multierror.Append(err, c.dockerClient.ContainerRemove(context.Background(), c.id, types.ContainerRemoveOptions{})).ErrorOrNil()
}

func toNatPort(p ContainerPort) nat.Port {
	return nat.Port(fmt.Sprintf("%d/tcp", p))
}
