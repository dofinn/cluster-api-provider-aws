/*
Copyright 2018 The Kubernetes Authors.

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

package machine

import (
	"context"
	"fmt"

	machinev1 "github.com/openshift/machine-api-operator/pkg/apis/machine/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	scopeFailFmt      = "%s: failed to create scope for machine: %v"
	createEventAction = "Create"
	updateEventAction = "Update"
	deleteEventAction = "Delete"
	noEventAction     = ""
)

// Actuator is responsible for performing machine reconciliation.
type Actuator struct {
	client        runtimeclient.Client
	eventRecorder record.EventRecorder
}

// ActuatorParams holds parameter information for Actuator.
type ActuatorParams struct {
	Client        runtimeclient.Client
	EventRecorder record.EventRecorder
}

// NewActuator returns an actuator.
func NewActuator(params ActuatorParams) *Actuator {
	return &Actuator{
		client:        params.Client,
		eventRecorder: params.EventRecorder,
	}
}

// Set corresponding event based on error. It also returns the original error
// for convenience, so callers can do "return handleMachineError(...)".
func (a *Actuator) handleMachineError(machine *machinev1.Machine, err error, eventAction string) error {
	klog.Errorf("%v error: %v", machine.GetName(), err)
	if eventAction != noEventAction {
		a.eventRecorder.Eventf(machine, corev1.EventTypeWarning, "Failed"+eventAction, "%v", err)
	}
	return err
}

// Create creates a machine and is invoked by the machine controller.
func (a *Actuator) Create(ctx context.Context, machine *machinev1.Machine) error {
	klog.Infof("%s: actuator creating machine", machine.GetName())
	scope, err := newMachineScope(machineScopeParams{
		Context: ctx,
		client:  a.client,
		machine: machine,
	})
	if err != nil {
		return a.handleMachineError(machine, err, createEventAction)
	}
	if err := newReconciler(scope).create(); err != nil {
		if err := scope.patchMachine(); err != nil {
			return err
		}
		return a.handleMachineError(machine, err, createEventAction)
	}
	a.eventRecorder.Eventf(machine, corev1.EventTypeNormal, createEventAction, "Created Machine %v", machine.GetName())
	return scope.patchMachine()
}

// Exists determines if the given machine currently exists.
// A machine which is not terminated is considered as existing.
func (a *Actuator) Exists(ctx context.Context, machine *machinev1.Machine) (bool, error) {
	klog.Infof("%s: actuator checking if machine exists", machine.GetName())
	scope, err := newMachineScope(machineScopeParams{
		Context: ctx,
		client:  a.client,
		machine: machine,
	})
	if err != nil {
		return false, fmt.Errorf(scopeFailFmt, machine.GetName(), err)
	}
	return newReconciler(scope).exists()
}

// Update attempts to sync machine state with an existing instance.
func (a *Actuator) Update(ctx context.Context, machine *machinev1.Machine) error {
	klog.Infof("%s: actuator updating machine", machine.GetName())
	scope, err := newMachineScope(machineScopeParams{
		Context: ctx,
		client:  a.client,
		machine: machine,
	})
	if err != nil {
		fmtErr := fmt.Sprintf(scopeFailFmt, machine.GetName(), err)
		return a.handleMachineError(machine, fmt.Errorf(fmtErr), updateEventAction)
	}
	if err := newReconciler(scope).update(); err != nil {
		// Update machine and machine status in case it was modified
		if err := scope.patchMachine(); err != nil {
			return err
		}
		return a.handleMachineError(machine, err, createEventAction)
	}
	a.eventRecorder.Eventf(machine, corev1.EventTypeNormal, updateEventAction, "Updated Machine %v", machine.GetName())
	return scope.patchMachine()
}

// Delete deletes a machine and updates its finalizer
func (a *Actuator) Delete(ctx context.Context, machine *machinev1.Machine) error {
	klog.Infof("%s: actuator deleting machine", machine.GetName())
	scope, err := newMachineScope(machineScopeParams{
		Context: ctx,
		client:  a.client,
		machine: machine,
	})
	if err != nil {
		fmtErr := fmt.Sprintf(scopeFailFmt, machine.GetName(), err)
		return a.handleMachineError(machine, fmt.Errorf(fmtErr), deleteEventAction)
	}
	if err := newReconciler(scope).delete(); err != nil {
		if err := scope.patchMachine(); err != nil {
			return err
		}
		return a.handleMachineError(machine, err, deleteEventAction)
	}
	a.eventRecorder.Eventf(machine, corev1.EventTypeNormal, deleteEventAction, "Deleted machine %v", machine.GetName())
	return scope.patchMachine()
}
