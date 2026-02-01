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

// SetupWebhookWithManager initializes webhook manager for HCloudRemediation.
func (r *HCloudRemediation) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(&hcloudRemediationDefaulter{}).
		WithValidator(&hcloudRemediationValidator{}).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hcloudremediation,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hcloudremediations,verbs=create;update,versions=v1beta1,name=mutation.hcloudremediation.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

// hcloudRemediationDefaulter implements webhook.CustomDefaulter.
type hcloudRemediationDefaulter struct{}

var _ webhook.CustomDefaulter = &hcloudRemediationDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (d *hcloudRemediationDefaulter) Default(_ context.Context, _ runtime.Object) error {
	return nil
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hcloudremediation,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hcloudremediations,verbs=create;update,versions=v1beta1,name=validation.hcloudremediation.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

// hcloudRemediationValidator implements webhook.CustomValidator.
type hcloudRemediationValidator struct{}

var _ webhook.CustomValidator = &hcloudRemediationValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (v *hcloudRemediationValidator) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (v *hcloudRemediationValidator) ValidateUpdate(_ context.Context, _, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (v *hcloudRemediationValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
