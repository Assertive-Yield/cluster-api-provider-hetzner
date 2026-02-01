/*
Copyright 2022 The Kubernetes Authors.

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

package v1beta1

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupWebhookWithManager initializes webhook manager for HetznerBareMetalMachine.
func (bmMachine *HetznerBareMetalMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(bmMachine).
		WithDefaulter(&hetznerBareMetalMachineDefaulter{}).
		WithValidator(&hetznerBareMetalMachineValidator{}).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerbaremetalmachine,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachines,verbs=create;update,versions=v1beta1,name=mutation.hetznerbaremetalmachine.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

// hetznerBareMetalMachineDefaulter implements webhook.CustomDefaulter.
type hetznerBareMetalMachineDefaulter struct{}

var _ webhook.CustomDefaulter = &hetznerBareMetalMachineDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (d *hetznerBareMetalMachineDefaulter) Default(_ context.Context, _ runtime.Object) error {
	return nil
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerbaremetalmachine,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalmachines,verbs=create;update,versions=v1beta1,name=validation.hetznerbaremetalmachine.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

// hetznerBareMetalMachineValidator implements webhook.CustomValidator.
type hetznerBareMetalMachineValidator struct{}

var _ webhook.CustomValidator = &hetznerBareMetalMachineValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (v *hetznerBareMetalMachineValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	bmMachine, ok := obj.(*HetznerBareMetalMachine)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a HetznerBareMetalMachine but got a %T", obj))
	}

	if bmMachine.Spec.SSHSpec.PortAfterCloudInit == 0 {
		bmMachine.Spec.SSHSpec.PortAfterCloudInit = bmMachine.Spec.SSHSpec.PortAfterInstallImage
	}

	allErrs := validateHetznerBareMetalMachineSpecCreate(bmMachine.Spec)

	return nil, aggregateObjErrors(bmMachine.GroupVersionKind().GroupKind(), bmMachine.Name, allErrs)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (v *hetznerBareMetalMachineValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldHetznerBareMetalMachine, ok := oldObj.(*HetznerBareMetalMachine)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a HetznerBareMetalMachine but got a %T", oldObj))
	}
	bmMachine, ok := newObj.(*HetznerBareMetalMachine)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a HetznerBareMetalMachine but got a %T", newObj))
	}

	allErrs := validateHetznerBareMetalMachineSpecUpdate(oldHetznerBareMetalMachine.Spec, bmMachine.Spec)

	return nil, aggregateObjErrors(bmMachine.GroupVersionKind().GroupKind(), bmMachine.Name, allErrs)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (v *hetznerBareMetalMachineValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
