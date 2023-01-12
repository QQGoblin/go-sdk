//go:build !windows
// +build !windows

/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package initsystem

import (
	"fmt"
	"k8s.io/klog/v2"
	"os/exec"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/pkg/errors"
)

const (
	DefaultStopIntervals = 1 * time.Second
	DefaultStopAttempts  = 120
)

// SystemdInitSystem defines systemd
type SystemdInitSystem struct{}

// EnableCommand return a string describing how to enable a service
func (sysd SystemdInitSystem) ServiceEnable(service string) error {
	args := []string{"enable", service}
	return exec.Command("systemctl", args...).Run()
}

// DisableCommand return a string describing how to enable a service
func (sysd SystemdInitSystem) ServiceDisable(service string) error {
	args := []string{"disable", service}
	return exec.Command("systemctl", args...).Run()
}

// reloadSystemd reloads the systemd daemon
func (sysd SystemdInitSystem) reloadSystemd() error {
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %v", err)
	}
	return nil
}

// ServiceStart tries to start a specific service
func (sysd SystemdInitSystem) ServiceStart(service string) error {
	// Before we try to start any service, make sure that systemd is ready
	if err := sysd.reloadSystemd(); err != nil {
		return err
	}
	args := []string{"start", service}
	return exec.Command("systemctl", args...).Run()
}

// ServiceRestart tries to reload the environment and restart the specific service
func (sysd SystemdInitSystem) ServiceRestart(service string) error {
	// Before we try to restart any service, make sure that systemd is ready
	if err := sysd.reloadSystemd(); err != nil {
		return err
	}
	args := []string{"restart", service}
	return exec.Command("systemctl", args...).Run()
}

// ServiceStop tries to stop a specific service
func (sysd SystemdInitSystem) ServiceStop(service string) error {
	args := []string{"stop", service}
	return exec.Command("systemctl", args...).Run()
}

// ServiceExists ensures the service is defined for this init system.
func (sysd SystemdInitSystem) ServiceExists(service string) bool {
	args := []string{"status", service}
	outBytes, _ := exec.Command("systemctl", args...).Output()
	output := string(outBytes)
	return !strings.Contains(output, "Loaded: not-found")
}

// ServiceIsEnabled ensures the service is enabled to start on each boot.
func (sysd SystemdInitSystem) ServiceIsEnabled(service string) bool {
	args := []string{"is-enabled", service}
	err := exec.Command("systemctl", args...).Run()
	return err == nil
}

// ServiceIsActive will check is the service is "active".
func (sysd SystemdInitSystem) ServiceIsActive(service string) bool {
	output := sysd.serviceStatus(service)

	return output == "active"
}

// ServiceIsInActive will check is the service is "inactive".
func (sysd SystemdInitSystem) ServiceIsInActive(service string) bool {

	output := sysd.serviceStatus(service)
	if output == "inactive" || output == "unknown" || output == "failed" {
		return true
	}
	return false
}

func (sysd SystemdInitSystem) serviceStatus(service string) string {
	args := []string{"is-active", service}
	// Ignoring error here, command returns non-0 if in "activating" status:
	outBytes, _ := exec.Command("systemctl", args...).Output()
	return strings.TrimSpace(string(outBytes))
}

// GetInitSystem returns an InitSystem for the current system, or nil
// if we cannot detect a supported init system.
// This indicates we will skip init system checks, not an error.
func GetInitSystem() (InitSystem, error) {
	// Assume existence of systemctl in path implies this is a systemd system:
	_, err := exec.LookPath("systemctl")
	if err == nil {
		return &SystemdInitSystem{}, nil
	}

	return nil, fmt.Errorf("no supported init system detected, skipping checking for services")
}

func EnsureStopService(service string) error {

	klog.Infof("stop service %s", service)
	systemctl, err := GetInitSystem()
	if err != nil {
		return err
	}

	if !systemctl.ServiceExists(service) {
		return nil
	}

	if err := systemctl.ServiceStop(service); err != nil {
		return err
	}

	if err := systemctl.ServiceDisable(service); err != nil {
		return err
	}

	f := func() error {
		if !systemctl.ServiceIsInActive(service) {
			return errors.New(fmt.Sprintf("error stop service<%s>", service))
		}
		return nil
	}

	if err := backoff.Retry(f, backoff.WithMaxRetries(backoff.NewConstantBackOff(DefaultStopIntervals), DefaultStopAttempts)); err != nil {
		return err
	}
	return nil
}
