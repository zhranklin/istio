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

package envoy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	envoyAdmin "github.com/envoyproxy/go-control-plane/envoy/admin/v2alpha"

	"istio.io/pkg/log"
)

const (
	defaultName = "envoy"
)

// Config for an Envoy Instance.
type Config struct {
	// Options provides the command-line options to be passed to Envoy.
	Options Options

	// Name of the envoy instance, used for logging only. If not provided, defaults to "envoy".
	Name string

	// BinaryPath the path to the Envoy binary.
	BinaryPath string

	// WorkingDir to be used when running Envoy. If not set, the current working directory is used.
	WorkingDir string

	// AdminPort specifies the administration port for the Envoy server. If not set, will
	// be determined by parsing the Envoy bootstrap configuration file.
	AdminPort uint32

	// SkipBaseIDClose skips calling Close on the BaseID, if one was specified.
	SkipBaseIDClose bool
}

// Waitable specifies a waitable operation.
type Waitable interface {
	// WithTimeout specifies an upper bound on the wait time.
	WithTimeout(timeout time.Duration) Waitable

	// Do performs the wait. By default, waits indefinitely. To specify an upper bound on
	// the wait time, use WithTimeout. If the wait times out, returns the last known error
	// for retried operations or context.DeadlineExceeded if no previous error was encountered.
	Do() error
}

// Instance of an Envoy process.
type Instance interface {
	// Config returns the configuration for this Instance.
	Config() Config

	// Start the Envoy Instance. The process will be killed if the given context is canceled.
	Start(ctx context.Context) Instance

	// WaitUntilLive polls the Envoy ServerInfo endpoint and waits for it to transition to "live". If the
	// wait times out, returns the last known error or context.DeadlineExceeded if no error occurred within the
	// specified duration.
	WaitLive() Waitable

	// AdminPort gets the administration port for Envoy.
	AdminPort() uint32

	// GetServerInfo returns a structure representing a call to /server_info
	GetServerInfo() (*envoyAdmin.ServerInfo, error)

	// GetConfigDump polls Envoy admin port for the config dump and returns the response.
	GetConfigDump() (*envoyAdmin.ConfigDump, error)

	// Wait for the Instance to terminate.
	Wait() Waitable

	// Kill the process, if running.
	Kill() error

	// KillAndWait is a helper that calls Kill and then waits for the process to terminate.
	KillAndWait() Waitable

	// Shutdown initiates the graceful termination of Envoy. Returns immediately and does not
	// wait for the process to exit.
	Shutdown() error

	// ShutdownAndWait is a helper that calls Shutdown and waits for the process to terminate.
	ShutdownAndWait() Waitable

	// BaseID used to start Envoy. If not set, returns InvalidBaseID.
	BaseID() BaseID

	// Epoch used to start Envoy.
	Epoch() Epoch
}

// FactoryFunc is a function that manufactures Envoy Instances.
type FactoryFunc func(cfg Config) (Instance, error)

var _ FactoryFunc = New

// New creates a new Envoy Instance with the given options.
func New(cfg Config) (Instance, error) {
	if cfg.Name == "" {
		cfg.Name = defaultName
	}

	// Process the binary path.
	if cfg.BinaryPath == "" {
		return nil, errors.New("must specify an Envoy binary")
	}

	// Create the config object from the options.
	ctx := newConfigContext()
	if err := cfg.Options.validate(ctx); err != nil {
		return nil, err
	}

	// Extract the admin port from the configuration.
	adminPort := cfg.AdminPort
	if adminPort <= 0 {
		var err error
		adminPort, err = ctx.getAdminPort()
		if err != nil {
			return nil, err
		}
	}

	// Create a new command with the specified options.
	args := cfg.Options.ToArgs()
	cmd := exec.Command(cfg.BinaryPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if cfg.WorkingDir != "" {
		cmd.Dir = cfg.WorkingDir
	}

	return &instance{
		name:      cfg.Name,
		config:    cfg,
		cmd:       cmd,
		adminPort: adminPort,
		baseID:    ctx.baseID,
		epoch:     ctx.epoch,
		waitCh:    make(chan struct{}, 1),
	}, nil
}

type instance struct {
	config    Config
	name      string
	waitErr   error
	cmd       *exec.Cmd
	waitCh    chan struct{}
	adminPort uint32
	baseID    BaseID
	epoch     Epoch
}

func (i *instance) Config() Config {
	return i.config
}

func (i *instance) Start(ctx context.Context) Instance {
	log.Infof("Starting Envoy(%s) with command: %v", i.name, i.cmd.Args)

	// Start Envoy.
	if err := i.cmd.Start(); err != nil {
		i.waitErr = err
		i.close()
		return i
	}

	// Asynchronously wait for the command to terminate.
	doneCh := make(chan error, 1)
	go func() {
		// Send the
		doneCh <- i.cmd.Wait()
		close(doneCh)
	}()

	go func() {
		// Free all resources and close doneCh when we exit.
		defer i.close()

		select {
		case <-ctx.Done():
			log.Infof("Aborting Envoy(%s): %v", i.name, ctx.Err())
			i.waitErr = ctx.Err()

			// Context aborted ... kill the process.
			if err := i.Kill(); err != nil {
				log.Warnf("Failed killing Envoy(%s): %v", i.name, err)
			}
			return
		case i.waitErr = <-doneCh:
			log.Infof("Envoy(%s) exited with error: %v", i.name, i.waitErr)
			return
		}
	}()

	return i
}

func (i *instance) WaitLive() Waitable {
	return &waitableImpl{
		instance:    i,
		retryPeriod: 200 * time.Millisecond,
		retryHandler: func() error {
			info, err := GetServerInfo(i.adminPort)
			if err != nil {
				return err
			}

			if info.State != envoyAdmin.ServerInfo_LIVE {
				return fmt.Errorf("envoy not live. Server State: %s", info.State)
			}
			return nil
		},
	}
}

func (i *instance) AdminPort() uint32 {
	return i.adminPort
}

func (i *instance) BaseID() BaseID {
	return i.baseID
}

func (i *instance) Epoch() Epoch {
	return i.epoch
}

func (i *instance) GetServerInfo() (*envoyAdmin.ServerInfo, error) {
	return GetServerInfo(i.adminPort)
}

func (i *instance) GetConfigDump() (*envoyAdmin.ConfigDump, error) {
	return GetConfigDump(i.adminPort)
}

func (i *instance) Wait() Waitable {
	return &waitableImpl{
		instance: i,
	}
}

func (i *instance) Kill() error {
	if i.cmd.Process == nil {
		return errors.New("envoy process was not started")
	}

	return i.cmd.Process.Kill()
}

func (i *instance) KillAndWait() Waitable {
	return &waitableImpl{
		instance:    i,
		creationErr: i.Kill(),
	}
}

func (i *instance) Shutdown() error {
	return Shutdown(i.adminPort)
}

func (i *instance) ShutdownAndWait() Waitable {
	return &waitableImpl{
		instance:    i,
		creationErr: i.Shutdown(),
	}
}

func (i *instance) close() {
	if !i.config.SkipBaseIDClose {
		if err := i.baseID.Close(); err != nil {
			log.Infof("Failed freeing BaseID for Envoy(%s): %v", i.name, err)
		}
	}

	close(i.waitCh)
}

var _ Waitable = &waitableImpl{}

type waitableImpl struct {
	*instance

	creationErr error

	retryPeriod  time.Duration
	retryHandler func() error

	timeout time.Duration
}

func (w *waitableImpl) Do() error {
	if w.creationErr != nil {
		return w.creationErr
	}

	// Create a dummy time channel to be used if needed.
	dummyTimeCh := make(chan time.Time, 1)
	defer close(dummyTimeCh)

	// Create the timeout channel
	var timeoutCh <-chan time.Time
	if w.timeout > 0 {
		// Create a timer for the specified duration.
		timer := time.NewTimer(w.timeout)
		defer timer.Stop()
		timeoutCh = timer.C
	} else {
		// No timeout was specified, just create dummy channel.
		timeoutCh = dummyTimeCh
	}

	var retryCh <-chan time.Time
	isRetrying := w.retryPeriod > 0
	if isRetrying {
		// Create a ticker for the specified duration.
		ticker := time.NewTicker(w.retryPeriod)
		defer ticker.Stop()
		retryCh = ticker.C
	} else {
		// No ticker was specified, just use a dummy channel.
		retryCh = dummyTimeCh
	}

	var lastErr error
	for {
		select {
		case <-w.waitCh:
			if w.waitErr != nil {
				return w.waitErr
			}
			if lastErr != nil {
				return lastErr
			}
			if isRetrying {
				// Envoy exited before the retry operation was successful,
				return errors.New("envoy process exited before wait completed")
			}
			return nil
		case <-retryCh:
			lastErr = w.retryHandler()
			if lastErr == nil {
				return nil
			}
		case <-timeoutCh:
			if lastErr != nil {
				return lastErr
			}
			// The timeout occurred before any other events.
			return context.DeadlineExceeded
		}
	}
}

func (w *waitableImpl) WithTimeout(d time.Duration) Waitable {
	out := *w
	out.timeout = d
	return &out
}
