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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupWebhookWithManager initializes webhook manager for HetznerBareMetalRemediationTemplate.
func (r *HetznerBareMetalRemediationTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(&hetznerBareMetalRemediationTemplateDefaulter{}).
		WithValidator(&hetznerBareMetalRemediationTemplateValidator{}).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerbaremetalremediationtemplate,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalremediationtemplates,verbs=create;update,versions=v1beta1,name=mhetznerbaremetalremediationtemplate.kb.io,admissionReviewVersions=v1

// hetznerBareMetalRemediationTemplateDefaulter implements webhook.CustomDefaulter.
type hetznerBareMetalRemediationTemplateDefaulter struct{}

var _ webhook.CustomDefaulter = &hetznerBareMetalRemediationTemplateDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (d *hetznerBareMetalRemediationTemplateDefaulter) Default(_ context.Context, _ runtime.Object) error {
	return nil
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerbaremetalremediationtemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerbaremetalremediationtemplates,verbs=create;update,versions=v1beta1,name=vhetznerbaremetalremediationtemplate.kb.io,admissionReviewVersions=v1

// hetznerBareMetalRemediationTemplateValidator implements webhook.CustomValidator.
type hetznerBareMetalRemediationTemplateValidator struct{}

var _ webhook.CustomValidator = &hetznerBareMetalRemediationTemplateValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (v *hetznerBareMetalRemediationTemplateValidator) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (v *hetznerBareMetalRemediationTemplateValidator) ValidateUpdate(_ context.Context, _, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (v *hetznerBareMetalRemediationTemplateValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
