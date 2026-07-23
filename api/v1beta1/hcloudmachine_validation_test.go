/*
Copyright 2024 The Kubernetes Authors.

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
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type args struct {
	oldSpec HCloudMachineSpec
	newSpec HCloudMachineSpec
}

func TestValidateHCloudMachineSpec(t *testing.T) {
	tests := []struct {
		name string
		args args
		want *field.Error
	}{
		{
			name: "Immutable Type",
			args: args{
				oldSpec: HCloudMachineSpec{
					Type:      "cpx11",
					ImageName: "ubuntu-24.04",
				},
				newSpec: HCloudMachineSpec{
					Type:      "cx21",
					ImageName: "ubuntu-24.04",
				},
			},
			want: field.Invalid(field.NewPath("spec", "type"), "cx21", "field is immutable"),
		},
		{
			name: "Immutable ImageName",
			args: args{
				oldSpec: HCloudMachineSpec{
					ImageName: "ubuntu-24.04",
				},
				newSpec: HCloudMachineSpec{
					ImageName: "centos-7",
				},
			},
			want: field.Invalid(field.NewPath("spec", "imageName"), "centos-7", "field is immutable"),
		},
		{
			name: "Immutable SSHKeys",
			args: args{
				oldSpec: HCloudMachineSpec{
					ImageName: "ubuntu-24.04",
					SSHKeys: []SSHKey{
						{
							Name:        "ssh-key-1",
							Fingerprint: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
						},
					},
				},
				newSpec: HCloudMachineSpec{
					ImageName: "ubuntu-24.04",
					SSHKeys: []SSHKey{
						{
							Name:        "ssh-key-1",
							Fingerprint: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
						},
						{
							Name:        "ssh-key-2",
							Fingerprint: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
						},
					},
				},
			},
			want: field.Invalid(field.NewPath("spec", "sshKeys"), []SSHKey{
				{
					Name:        "ssh-key-1",
					Fingerprint: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
				},
				{
					Name:        "ssh-key-2",
					Fingerprint: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC",
				},
			}, "field is immutable"),
		},
		{
			name: "Immutable PlacementGroupName",
			args: args{
				oldSpec: HCloudMachineSpec{
					ImageName:          "ubuntu-24.04",
					PlacementGroupName: createPlacementGroupName("placement-group-1"),
				},
				newSpec: HCloudMachineSpec{
					ImageName:          "ubuntu-24.04",
					PlacementGroupName: createPlacementGroupName("placement-group-2"),
				},
			},
			want: field.Invalid(field.NewPath("spec", "placementGroupName"), "placement-group-2", "field is immutable"),
		},
		{
			name: "No Errors",
			args: args{
				oldSpec: HCloudMachineSpec{
					Type:               "cpx11",
					ImageName:          "ubuntu-24.04",
					SSHKeys:            []SSHKey{{Name: "ssh-key-1"}},
					PlacementGroupName: createPlacementGroupName("placement-group-1"),
				},
				newSpec: HCloudMachineSpec{
					Type:               "cpx11",
					ImageName:          "ubuntu-24.04",
					SSHKeys:            []SSHKey{{Name: "ssh-key-1"}},
					PlacementGroupName: createPlacementGroupName("placement-group-1"),
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateHCloudMachineSpec(tt.args.oldSpec, tt.args.newSpec)

			if len(got) == 0 {
				assert.Empty(t, got)
			}

			if len(got) > 1 {
				t.Errorf("got length: %d greater than 1", len(got))
			}

			// assert if length of got is 1
			if len(got) == 1 {
				assert.Equal(t, tt.want.Type, got[0].Type)
				assert.Equal(t, tt.want.Field, got[0].Field)
				assert.Equal(t, tt.want.Detail, got[0].Detail)
			}
		})
	}
}

func TestValidateHCloudMachineSpecCreate_ImageURL(t *testing.T) {
	tests := []struct {
		name    string
		spec    HCloudMachineSpec
		wantErr bool
		field   string
	}{
		{
			name:    "imageName only ok",
			spec:    HCloudMachineSpec{ImageName: "ubuntu-24.04"},
			wantErr: false,
		},
		{
			name: "imageURL + command ok",
			spec: HCloudMachineSpec{
				ImageURL:        "https://factory.talos.dev/image/example/v1.13.7/hcloud-amd64.raw.xz",
				ImageURLCommand: "image-url-command-talos",
			},
			wantErr: false,
		},
		{
			name:    "both empty",
			spec:    HCloudMachineSpec{},
			wantErr: true,
			field:   "spec.imageName",
		},
		{
			name: "mutual exclusive",
			spec: HCloudMachineSpec{
				ImageName:       "ubuntu-24.04",
				ImageURL:        "https://example.com/img.raw",
				ImageURLCommand: "image-url-command-x",
			},
			wantErr: true,
			field:   "spec.imageName",
		},
		{
			name: "imageURL without command",
			spec: HCloudMachineSpec{
				ImageURL: "https://example.com/img.raw",
			},
			wantErr: true,
			field:   "spec.imageURLCommand",
		},
		{
			name: "bad command name",
			spec: HCloudMachineSpec{
				ImageURL:        "https://example.com/img.raw",
				ImageURLCommand: "not-a-valid-name",
			},
			wantErr: true,
			field:   "spec.imageURLCommand",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateHCloudMachineSpecCreate(tt.spec)
			if tt.wantErr {
				assert.NotEmpty(t, got)
				if len(got) > 0 {
					assert.Equal(t, tt.field, got[0].Field)
				}
			} else {
				assert.Empty(t, got)
			}
		})
	}
}

func createPlacementGroupName(name string) *string {
	return &name
}
