/*
Copyright 2021 The Kubernetes Authors.

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
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

// log is for logging in this package.
var hetznerclustertemplatelog = utils.GetDefaultLogger("info").WithName("hetznerclustertemplate-resource")

// SetupWebhookWithManager initializes webhook manager for HetznerClusterTemplate.
func (r *HetznerClusterTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(&hetznerClusterTemplateDefaulter{}).
		WithValidator(&hetznerClusterTemplateValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerclustertemplate,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerclustertemplates,verbs=create;update,versions=v1beta1,name=mutation.hetznerclustertemplate.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

// hetznerClusterTemplateDefaulter implements webhook.CustomDefaulter.
type hetznerClusterTemplateDefaulter struct{}

var _ webhook.CustomDefaulter = &hetznerClusterTemplateDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (d *hetznerClusterTemplateDefaulter) Default(_ context.Context, obj runtime.Object) error {
	r, ok := obj.(*HetznerClusterTemplate)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a HetznerClusterTemplate but got a %T", obj))
	}

	hetznerclustertemplatelog.V(1).Info("default", "name", r.Name)
	return nil
}

// +kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-hetznerclustertemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=hetznerclustertemplates,verbs=create;update,versions=v1beta1,name=validation.hetznerclustertemplate.infrastructure.cluster.x-k8s.io,admissionReviewVersions={v1,v1beta1}

// hetznerClusterTemplateValidator implements webhook.CustomValidator.
type hetznerClusterTemplateValidator struct{}

var _ webhook.CustomValidator = &hetznerClusterTemplateValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (v *hetznerClusterTemplateValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	r, ok := obj.(*HetznerClusterTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a HetznerClusterTemplate but got a %T", obj))
	}

	hetznerclustertemplatelog.V(1).Info("validate create", "name", r.Name)
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (v *hetznerClusterTemplateValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	old, ok := oldObj.(*HetznerClusterTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected an HetznerClusterTemplate but got a %T", oldObj))
	}
	r, ok := newObj.(*HetznerClusterTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected an HetznerClusterTemplate but got a %T", newObj))
	}

	hetznerclustertemplatelog.V(1).Info("validate update", "name", r.Name)

	if !reflect.DeepEqual(r.Spec, old.Spec) {
		return nil, apierrors.NewBadRequest("HetznerClusterTemplate.Spec is immutable")
	}
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (v *hetznerClusterTemplateValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	r, ok := obj.(*HetznerClusterTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a HetznerClusterTemplate but got a %T", obj))
	}

	hetznerclustertemplatelog.V(1).Info("validate delete", "name", r.Name)
	return nil, nil
}
