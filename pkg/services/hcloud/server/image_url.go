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

package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capierrors "sigs.k8s.io/cluster-api/errors"
	conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	"sigs.k8s.io/cluster-api/util/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/Assertive-Yield/cluster-api-provider-hetzner/api/v1beta1"
	secretutil "github.com/Assertive-Yield/cluster-api-provider-hetzner/pkg/secrets"
	sshclient "github.com/Assertive-Yield/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	"github.com/Assertive-Yield/cluster-api-provider-hetzner/pkg/utils"
)

const (
	// Base OS used only to create the VM before rescue+imageURLCommand run.
	preRescueOSImage = "ubuntu-24.04"
	// Directory on the controller pod that holds image-url-command binaries (mounted /shared).
	hcloudImageURLCommandDir = "/shared"

	bootStateTimeoutUnset           = 5 * time.Minute
	bootStateTimeoutInitializing    = 6 * time.Minute
	bootStateTimeoutEnablingRescue  = 5 * time.Minute
	bootStateTimeoutBootingToRescue = 6 * time.Minute
	// Must stay in sync with ImageURL docstring (7 minutes).
	bootStateTimeoutRunningImageCommand = 7 * time.Minute
)

var errSSHKeyMisconfigured = errors.New("ssh private key for rescue is misconfigured")

// reconcileImageURL implements the HCloud imageURL provisioning state machine.
// Ported from syself CAPH v1.1.x for the AY fork.
func (s *Service) reconcileImageURL(ctx context.Context) (reconcile.Result, error) {
	failureDomain, err := s.scope.GetFailureDomain()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get failure domain: %w", err)
	}
	s.scope.SetRegion(failureDomain)

	if !s.scope.IsBootstrapDataReady() {
		conditions.MarkFalse(
			s.scope.HCloudMachine,
			infrav1.BootstrapReadyCondition,
			infrav1.BootstrapNotReadyReason,
			clusterv1.ConditionSeverityInfo,
			"bootstrap not ready yet",
		)
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}
	conditions.MarkTrue(s.scope.HCloudMachine, infrav1.BootstrapReadyCondition)

	hm := s.scope.HCloudMachine
	if hm.Status.BootState == infrav1.HCloudBootStateProvisioningFailed {
		s.scope.SetReady(false)
		return reconcile.Result{}, nil
	}

	// Ensure server exists (create on Unset / empty).
	server, err := s.findServer(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get server: %w", err)
	}

	switch hm.Status.BootState {
	case infrav1.HCloudBootStateUnset:
		return s.handleImageURLBootStateUnset(ctx, server)
	case infrav1.HCloudBootStateInitializing:
		return s.handleImageURLBootStateInitializing(ctx, server)
	case infrav1.HCloudBootStateEnablingRescue:
		return s.handleImageURLBootStateEnablingRescue(ctx, server)
	case infrav1.HCloudBootStateBootingToRescue:
		return s.handleImageURLBootStateBootingToRescue(ctx, server)
	case infrav1.HCloudBootStateRunningImageCommand:
		return s.handleImageURLBootStateRunningImageCommand(ctx, server)
	case infrav1.HCloudBootStateBootingToRealOS, infrav1.HCloudBootStateOperatingSystemRunning:
		return s.handleImageURLBootingToRealOS(ctx, server, failureDomain)
	default:
		return reconcile.Result{}, fmt.Errorf("unknown BootState: %s", hm.Status.BootState)
	}
}

func (s *Service) handleImageURLBootStateUnset(ctx context.Context, server *hcloud.Server) (reconcile.Result, error) {
	hm := s.scope.HCloudMachine
	if hm.Status.BootStateSince.IsZero() {
		hm.Status.BootStateSince = hm.CreationTimestamp
		if hm.Status.BootStateSince.IsZero() {
			hm.SetBootState(infrav1.HCloudBootStateUnset)
		}
	}

	if time.Since(hm.Status.BootStateSince.Time) > bootStateTimeoutUnset {
		msg := fmt.Sprintf("boot state unset timed out after %s", bootStateTimeoutUnset)
		return s.failImageURLProvisioning(msg)
	}

	// Validate rescue SSH key is configured before creating the server.
	if _, err := s.getRescueSSHPrivateKey(ctx); err != nil {
		s.scope.Error(err, "rescue ssh key")
		if errors.Is(err, errSSHKeyMisconfigured) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
	}
	conditions.MarkTrue(hm, infrav1.SSHPrivateKeyAvailableCondition)

	if server == nil {
		created, err := s.createServerFromImageURL(ctx)
		if err != nil {
			if errors.Is(err, errServerCreateNotPossible) {
				return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
			}
			return reconcile.Result{}, fmt.Errorf("failed to create server for imageURL: %w", err)
		}
		server = created
	}

	s.scope.SetProviderID(server.ID)
	s.updateStatusPreserveBoot(server, failureDomainFromMachine(s))
	conditions.MarkTrue(hm, infrav1.ServerCreateSucceededCondition)

	// createServerFromImageURL already sets BootState=Initializing
	return reconcile.Result{RequeueAfter: 15 * time.Second}, nil
}

func (s *Service) handleImageURLBootStateInitializing(ctx context.Context, server *hcloud.Server) (reconcile.Result, error) {
	hm := s.scope.HCloudMachine
	if server == nil {
		return s.failImageURLProvisioning("server missing in Initializing state")
	}
	if time.Since(hm.Status.BootStateSince.Time) > bootStateTimeoutInitializing {
		return s.failImageURLProvisioning(fmt.Sprintf("initializing timed out after %s", bootStateTimeoutInitializing))
	}

	s.updateStatusPreserveBoot(server, failureDomainFromMachine(s))

	if server.Status != hcloud.ServerStatusRunning {
		conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition, "WaitingForServerRunning",
			clusterv1.ConditionSeverityInfo, "waiting for pre-rescue OS to be running")
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Attach network early so rescue can use private IP if needed.
	if err := s.reconcileNetworkAttachment(ctx, server); err != nil {
		return reconcile.Result{}, fmt.Errorf("network attach during imageURL init: %w", err)
	}

	// Must match syself/upstream: linux64 rescue + inject the same HCloud SSH keys used at create,
	// otherwise rescue may not accept our robot/private key and reboot-into-rescue is unreliable.
	hcloudSSHKeys, err := s.hcloudSSHKeysForServer(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}
	rescueOpts := &hcloud.ServerEnableRescueOpts{
		Type:    hcloud.ServerRescueTypeLinux64,
		SSHKeys: hcloudSSHKeys,
	}
	result, err := s.scope.HCloudClient.EnableRescueSystem(ctx, server, rescueOpts)
	if err != nil {
		return reconcile.Result{}, handleRateLimit(hm, err, "EnableRescueSystem", "failed to enable rescue system")
	}
	if result.Action != nil {
		hm.Status.ExternalIDs.ActionIDEnableRescueSystem = result.Action.ID
	}
	hm.SetBootState(infrav1.HCloudBootStateEnablingRescue)
	conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition, "EnablingRescue",
		clusterv1.ConditionSeverityInfo, "waiting for rescue system to be enabled")
	return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
}

// actionDoneEnableRescue is a sentinel stored in ExternalIDs after the enable-rescue API action finishes.
// We then wait one more reconcile before rebooting (Hetzner can ignore an immediate reboot).
const actionDoneEnableRescue int64 = -1

func (s *Service) handleImageURLBootStateEnablingRescue(ctx context.Context, server *hcloud.Server) (reconcile.Result, error) {
	hm := s.scope.HCloudMachine
	if server == nil {
		return s.failImageURLProvisioning("server missing in EnablingRescue state")
	}
	if time.Since(hm.Status.BootStateSince.Time) > bootStateTimeoutEnablingRescue {
		return s.failImageURLProvisioning(fmt.Sprintf("enabling rescue timed out after %s", bootStateTimeoutEnablingRescue))
	}

	s.updateStatusPreserveBoot(server, failureDomainFromMachine(s))

	actionID := hm.Status.ExternalIDs.ActionIDEnableRescueSystem
	if actionID == 0 {
		return s.failImageURLProvisioning("ActionIDEnableRescueSystem not set after EnableRescue")
	}

	if actionID != actionDoneEnableRescue {
		action, err := s.scope.HCloudClient.GetAction(ctx, actionID)
		if err != nil {
			return reconcile.Result{}, handleRateLimit(hm, err, "GetAction", "failed to get enable-rescue action")
		}
		switch action.Status {
		case hcloud.ActionStatusRunning:
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		case hcloud.ActionStatusError:
			return s.failImageURLProvisioning(fmt.Sprintf("enable rescue action failed: %v", action.ErrorMessage))
		case hcloud.ActionStatusSuccess:
			// Mark done and delay reboot — rebooting immediately after the action can be ignored by Hetzner.
			hm.Status.ExternalIDs.ActionIDEnableRescueSystem = actionDoneEnableRescue
			conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition, "EnablingRescueActionDone",
				clusterv1.ConditionSeverityInfo, "rescue enable action finished; delaying reboot")
			return reconcile.Result{RequeueAfter: 4 * time.Second}, nil
		default:
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}

	// Refresh server so RescueEnabled is current.
	refreshed, err := s.scope.HCloudClient.GetServer(ctx, server.ID)
	if err != nil {
		return reconcile.Result{}, handleRateLimit(hm, err, "GetServer", "failed to get server before rescue reboot")
	}
	if refreshed != nil {
		server = refreshed
		s.updateStatusPreserveBoot(server, failureDomainFromMachine(s))
	}
	if !server.RescueEnabled {
		conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition, "RescueNotEnabledYet",
			clusterv1.ConditionSeverityWarning, "rescue flag not set yet after enable action")
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Reboot via SSH into the pre-rescue OS (avoids HCloud reboot races; matches upstream CAPH).
	sshClient, err := s.getRescueSSHClient(ctx)
	if err != nil {
		conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition, "GetSSHClientFailed",
			clusterv1.ConditionSeverityWarning, "%s", err.Error())
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}
	if out := sshClient.Reboot(); out.Err != nil {
		conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition, "RebootViaSSHFailed",
			clusterv1.ConditionSeverityWarning, "reboot via ssh: %s", out.Err.Error())
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	hm.SetBootState(infrav1.HCloudBootStateBootingToRescue)
	conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition, "BootingToRescue",
		clusterv1.ConditionSeverityInfo, "reboot to rescue started (ssh)")
	return reconcile.Result{RequeueAfter: 15 * time.Second}, nil
}

func (s *Service) handleImageURLBootStateBootingToRescue(ctx context.Context, server *hcloud.Server) (reconcile.Result, error) {
	hm := s.scope.HCloudMachine
	if server == nil {
		return s.failImageURLProvisioning("server missing in BootingToRescue state")
	}
	if time.Since(hm.Status.BootStateSince.Time) > bootStateTimeoutBootingToRescue {
		return s.failImageURLProvisioning(fmt.Sprintf("booting to rescue timed out after %s", bootStateTimeoutBootingToRescue))
	}

	// Refresh server and status/addresses.
	refreshed, err := s.scope.HCloudClient.GetServer(ctx, server.ID)
	if err != nil {
		return reconcile.Result{}, handleRateLimit(hm, err, "GetServer", "failed to get server")
	}
	if refreshed == nil {
		return s.failImageURLProvisioning("server disappeared while booting to rescue")
	}
	server = refreshed
	s.updateStatusPreserveBoot(server, failureDomainFromMachine(s))

	sshClient, err := s.getRescueSSHClient(ctx)
	if err != nil {
		conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition, "SSHToRescuePending",
			clusterv1.ConditionSeverityInfo, "%s", err.Error())
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	hostnameOut := sshClient.GetHostName()
	if hostnameOut.Err != nil {
		conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition, "SSHToRescuePending",
			clusterv1.ConditionSeverityInfo, "waiting for rescue ssh: %s", hostnameOut.Err.Error())
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}
	if hostname := strings.TrimSpace(hostnameOut.StdOut); hostname != "rescue" {
		conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition, "WaitingForRescueHostname",
			clusterv1.ConditionSeverityInfo, "remote hostname %q, expected rescue", hostname)
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	imageURLCommandPath, err := utils.ResolveImageURLCommandPath(hcloudImageURLCommandDir, hm.Spec.ImageURLCommand)
	if err != nil {
		conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition, infrav1.ImageURLCommandNotAccessibleReason,
			clusterv1.ConditionSeverityError, "%s", err.Error())
		return s.failImageURLProvisioning(err.Error())
	}

	bootstrapData, err := s.scope.GetRawBootstrapData(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("bootstrap data: %w", err)
	}

	exitStatus, stdoutStderr, err := sshClient.StartImageURLCommand(ctx, imageURLCommandPath, hm.Spec.ImageURL, bootstrapData, s.scope.Name(), []string{"sda"})
	if err != nil {
		record.Warnf(hm, "StartImageURLCommandFailed", "%s", err.Error())
		conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition, infrav1.ImageURLCommandFailedReason,
			clusterv1.ConditionSeverityWarning, "StartImageURLCommand: %s", err.Error())
		return reconcile.Result{RequeueAfter: 15 * time.Second}, nil
	}
	if exitStatus != 0 {
		msg := fmt.Sprintf("StartImageURLCommand non-zero exit %d: %s", exitStatus, stdoutStderr)
		return s.failImageURLProvisioning(msg)
	}

	hm.SetBootState(infrav1.HCloudBootStateRunningImageCommand)
	conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition, infrav1.ImageURLCommandRunningReason,
		clusterv1.ConditionSeverityInfo, "image-url-command running")
	return reconcile.Result{RequeueAfter: 20 * time.Second}, nil
}

func (s *Service) handleImageURLBootStateRunningImageCommand(ctx context.Context, server *hcloud.Server) (reconcile.Result, error) {
	hm := s.scope.HCloudMachine
	if server == nil {
		return s.failImageURLProvisioning("server missing in RunningImageCommand state")
	}
	if time.Since(hm.Status.BootStateSince.Time) > bootStateTimeoutRunningImageCommand {
		return s.failImageURLProvisioning(fmt.Sprintf("image-url-command timed out after %s", bootStateTimeoutRunningImageCommand))
	}

	s.updateStatusPreserveBoot(server, failureDomainFromMachine(s))

	sshClient, err := s.getRescueSSHClient(ctx)
	if err != nil {
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	state, logFile, err := sshClient.StateOfImageURLCommand(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("StateOfImageURLCommand: %w", err)
	}

	switch state {
	case sshclient.ImageURLCommandStateRunning, sshclient.ImageURLCommandStateNotStarted:
		conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition, infrav1.ImageURLCommandRunningReason,
			clusterv1.ConditionSeverityInfo, "image-url-command still running")
		return reconcile.Result{RequeueAfter: 20 * time.Second}, nil
	case sshclient.ImageURLCommandStateFailed:
		record.Warn(hm, "ImageURLCommandFailed", logFile)
		return s.failImageURLProvisioning(fmt.Sprintf("ImageURLCommand failed: %s", logFile))
	case sshclient.ImageURLCommandStateFinishedSuccessfully:
		record.Event(hm, "ImageURLCommandSucceeded", "IMAGE_URL_DONE received")
		// Reboot into installed OS (command already wrote the disk).
		if err := s.scope.HCloudClient.RebootServer(ctx, server); err != nil {
			return reconcile.Result{}, handleRateLimit(hm, err, "RebootServer", "reboot after ImageURLCommand failed")
		}
		hm.SetBootState(infrav1.HCloudBootStateBootingToRealOS)
		conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition, "BootingToRealOS",
			clusterv1.ConditionSeverityInfo, "rebooting into installed OS")
		return reconcile.Result{RequeueAfter: 20 * time.Second}, nil
	default:
		return reconcile.Result{}, fmt.Errorf("unknown ImageURLCommandState: %q", state)
	}
}

func (s *Service) handleImageURLBootingToRealOS(ctx context.Context, server *hcloud.Server, failureDomain string) (reconcile.Result, error) {
	hm := s.scope.HCloudMachine
	if server == nil {
		var err error
		server, err = s.findServer(ctx)
		if err != nil {
			return reconcile.Result{}, err
		}
		if server == nil {
			return s.failImageURLProvisioning("server missing while booting real OS")
		}
	}

	// Refresh
	refreshed, err := s.scope.HCloudClient.GetServer(ctx, server.ID)
	if err != nil {
		return reconcile.Result{}, handleRateLimit(hm, err, "GetServer", "failed to get server")
	}
	if refreshed != nil {
		server = refreshed
	}

	s.scope.SetProviderID(server.ID)
	s.updateStatusPreserveBoot(server, failureDomain)

	switch server.Status {
	case hcloud.ServerStatusOff:
		return s.handleServerStatusOff(ctx, server)
	case hcloud.ServerStatusStarting:
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition, infrav1.ServerStartingReason,
			clusterv1.ConditionSeverityInfo, "server is starting")
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
	case hcloud.ServerStatusRunning:
		// continue
	default:
		s.scope.SetReady(false)
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	if err := s.reconcileNetworkAttachment(ctx, server); err != nil {
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition, infrav1.NetworkAttachFailedReason,
			clusterv1.ConditionSeverityError, "%s", err.Error())
		return reconcile.Result{}, err
	}

	if hm.Status.BootState != infrav1.HCloudBootStateOperatingSystemRunning {
		hm.SetBootState(infrav1.HCloudBootStateOperatingSystemRunning)
	}
	conditions.MarkTrue(hm, infrav1.ServerProvisionedCondition)

	if !s.scope.IsControlPlane() {
		conditions.MarkTrue(hm, infrav1.ServerAvailableCondition)
		s.scope.SetReady(true)
		return reconcile.Result{}, nil
	}

	res, err := s.reconcileLoadBalancerAttachment(ctx, server)
	if err != nil {
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition, infrav1.LoadBalancerAttachFailedReason,
			clusterv1.ConditionSeverityError, "%s", err.Error())
		return res, err
	}

	s.scope.SetReady(true)
	conditions.MarkTrue(hm, infrav1.ServerAvailableCondition)
	return res, nil
}

func (s *Service) createServerFromImageURL(ctx context.Context) (*hcloud.Server, error) {
	hm := s.scope.HCloudMachine

	if _, err := utils.ResolveImageURLCommandPath(hcloudImageURLCommandDir, hm.Spec.ImageURLCommand); err != nil {
		err = fmt.Errorf("imageURLCommand %q is invalid or not accessible by the controller pod: %w", hm.Spec.ImageURLCommand, err)
		conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition, infrav1.ImageURLCommandNotAccessibleReason,
			clusterv1.ConditionSeverityError, "%s", err.Error())
		s.scope.SetError(err.Error(), capierrors.CreateMachineError)
		return nil, errServerCreateNotPossible
	}

	// Temporarily set ImageName to the pre-rescue OS for getServerImage, without persisting mutability issues:
	// getServerImage reads Spec.ImageName — call a dedicated lookup.
	image, err := s.getServerImageByName(ctx, preRescueOSImage)
	if err != nil {
		return nil, fmt.Errorf("failed to get pre-rescue OS image %q: %w", preRescueOSImage, err)
	}

	// Still attach bootstrap as HCloud user-data. The temporary pre-rescue OS ignores it;
	// after imageURLCommand writes the real OS (e.g. Talos) and we reboot, the platform
	// re-reads the same user-data from the HCloud metadata service (required for Talos).
	userData, err := s.scope.GetRawBootstrapData(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get raw bootstrap data for imageURL create: %w", err)
	}

	server, err := s.createServerWithImageAndUserData(ctx, image, userData)
	if err != nil {
		return nil, err
	}

	hm.SetBootState(infrav1.HCloudBootStateInitializing)
	return server, nil
}

func (s *Service) createServerWithImageAndUserData(ctx context.Context, image *hcloud.Image, userData []byte) (*hcloud.Server, error) {
	// Same as createServer but with explicit image/userData (userData may be nil for imageURL).
	automount := false
	startAfterCreate := true
	opts := hcloud.ServerCreateOpts{
		Name:   s.serverName(),
		Labels: s.createLabels(),
		Image:  image,
		Location: &hcloud.Location{
			Name: string(s.scope.HCloudMachine.Status.Region),
		},
		ServerType: &hcloud.ServerType{
			Name: string(s.scope.HCloudMachine.Spec.Type),
		},
		Automount:        &automount,
		StartAfterCreate: &startAfterCreate,
		UserData:         string(userData),
		PublicNet: &hcloud.ServerCreatePublicNet{
			EnableIPv4: s.scope.HCloudMachine.Spec.PublicNetwork.EnableIPv4,
			EnableIPv6: s.scope.HCloudMachine.Spec.PublicNetwork.EnableIPv6,
		},
	}

	if s.scope.HCloudMachine.Spec.PlacementGroupName != nil {
		var foundPlacementGroupInStatus bool
		for _, pgSts := range s.scope.HetznerCluster.Status.HCloudPlacementGroups {
			if *s.scope.HCloudMachine.Spec.PlacementGroupName == pgSts.Name {
				foundPlacementGroupInStatus = true
				opts.PlacementGroup = &hcloud.PlacementGroup{
					ID:   pgSts.ID,
					Name: pgSts.Name,
					Type: hcloud.PlacementGroupType(pgSts.Type),
				}
			}
		}
		if !foundPlacementGroupInStatus {
			conditions.MarkFalse(s.scope.HCloudMachine,
				infrav1.ServerCreateSucceededCondition,
				infrav1.InstanceHasNonExistingPlacementGroupReason,
				clusterv1.ConditionSeverityError,
				"Placement group %q does not exist in cluster",
				*s.scope.HCloudMachine.Spec.PlacementGroupName,
			)
			return nil, errServerCreateNotPossible
		}
	}

	sshKeySpecs := s.scope.HCloudMachine.Spec.SSHKeys
	if len(sshKeySpecs) == 0 {
		sshKeySpecs = s.scope.HetznerCluster.Spec.SSHKeys.HCloud
	}
	sshKeyName := s.scope.HetznerSecret().Data[s.scope.HetznerCluster.Spec.HetznerSecret.Key.SSHKey]
	if len(sshKeyName) > 0 {
		keyExists := false
		for _, key := range sshKeySpecs {
			if string(sshKeyName) == key.Name {
				keyExists = true
				break
			}
		}
		if !keyExists {
			sshKeySpecs = append(sshKeySpecs, infrav1.SSHKey{Name: string(sshKeyName)})
		}
	}

	sshKeysAPI, err := s.scope.HCloudClient.ListSSHKeys(ctx, hcloud.SSHKeyListOpts{})
	if err != nil {
		return nil, handleRateLimit(s.scope.HCloudMachine, err, "ListSSHKeys", "failed listing ssh keys from hcloud")
	}
	opts.SSHKeys, err = filterHCloudSSHKeys(sshKeysAPI, sshKeySpecs)
	if err != nil {
		conditions.MarkFalse(
			s.scope.HCloudMachine,
			infrav1.ServerCreateSucceededCondition,
			infrav1.SSHKeyNotFoundReason,
			clusterv1.ConditionSeverityError,
			"%s",
			err.Error(),
		)
		return nil, errServerCreateNotPossible
	}

	if net := s.scope.HetznerCluster.Status.Network; net != nil {
		opts.Networks = []*hcloud.Network{{ID: net.ID}}
	}
	if !s.scope.HetznerCluster.Spec.HCloudNetwork.Enabled {
		opts.PublicNet.EnableIPv4 = true
	}

	server, err := s.scope.HCloudClient.CreateServer(ctx, opts)
	if err != nil {
		msg := fmt.Sprintf("failed to create HCloud server for imageURL: %v", err)
		conditions.MarkFalse(s.scope.HCloudMachine, infrav1.ServerCreateSucceededCondition,
			infrav1.ServerCreateFailedReason, clusterv1.ConditionSeverityWarning, "%s", msg)
		record.Warn(s.scope.HCloudMachine, "FailedCreateHCloudServer", msg)
		return nil, err
	}

	s.scope.HCloudMachine.Status.SSHKeys = sshKeySpecs
	record.Eventf(s.scope.HCloudMachine, "SuccessfulCreate", "Created new server %s with ID %d (imageURL path)", server.Name, server.ID)
	return server, nil
}

func (s *Service) getServerImageByName(ctx context.Context, imageName string) (*hcloud.Image, error) {
	// Temporarily swap Spec.ImageName for lookup via existing helper.
	hm := s.scope.HCloudMachine
	orig := hm.Spec.ImageName
	hm.Spec.ImageName = imageName
	defer func() { hm.Spec.ImageName = orig }()
	return s.getServerImage(ctx)
}

// hcloudSSHKeysForServer resolves HCloud API SSH key objects to attach to EnableRescue
// (same set used when creating the pre-rescue server).
func (s *Service) hcloudSSHKeysForServer(ctx context.Context) ([]*hcloud.SSHKey, error) {
	sshKeySpecs := s.scope.HCloudMachine.Spec.SSHKeys
	if len(sshKeySpecs) == 0 {
		sshKeySpecs = s.scope.HetznerCluster.Spec.SSHKeys.HCloud
	}
	if s.scope.HetznerSecret() != nil {
		sshKeyName := s.scope.HetznerSecret().Data[s.scope.HetznerCluster.Spec.HetznerSecret.Key.SSHKey]
		if len(sshKeyName) > 0 {
			keyExists := false
			for _, key := range sshKeySpecs {
				if string(sshKeyName) == key.Name {
					keyExists = true
					break
				}
			}
			if !keyExists {
				sshKeySpecs = append(sshKeySpecs, infrav1.SSHKey{Name: string(sshKeyName)})
			}
		}
	}
	sshKeysAPI, err := s.scope.HCloudClient.ListSSHKeys(ctx, hcloud.SSHKeyListOpts{})
	if err != nil {
		return nil, handleRateLimit(s.scope.HCloudMachine, err, "ListSSHKeys", "failed listing ssh keys for rescue")
	}
	keys, err := filterHCloudSSHKeys(sshKeysAPI, sshKeySpecs)
	if err != nil {
		return nil, fmt.Errorf("resolve ssh keys for rescue: %w", err)
	}
	return keys, nil
}

func (s *Service) getRescueSSHPrivateKey(ctx context.Context) (string, error) {
	robotSecretName := s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.Name
	if robotSecretName == "" {
		conditions.MarkFalse(
			s.scope.HCloudMachine,
			infrav1.SSHPrivateKeyAvailableCondition,
			infrav1.SSHPrivateKeySecretRefNotConfiguredReason,
			clusterv1.ConditionSeverityError,
			"HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.Name is empty",
		)
		return "", fmt.Errorf("%w: RobotRescueSecretRef.Name is empty", errSSHKeyMisconfigured)
	}

	secretManager := secretutil.NewSecretManager(s.scope.Logger, s.scope.Client, s.scope.APIReader)
	robotSecret, err := secretManager.ObtainSecret(ctx, types.NamespacedName{
		Name:      robotSecretName,
		Namespace: s.scope.Namespace(),
	})
	if err != nil {
		if apierrors.IsNotFound(err) {
			conditions.MarkFalse(
				s.scope.HCloudMachine,
				infrav1.SSHPrivateKeyAvailableCondition,
				infrav1.SSHPrivateKeySecretNotFoundReason,
				clusterv1.ConditionSeverityWarning,
				"secret %s/%s not found", s.scope.Namespace(), robotSecretName,
			)
		}
		return "", fmt.Errorf("failed to get secret %q: %w", robotSecretName, err)
	}

	keyField := s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.Key.PrivateKey
	privateKey := string(robotSecret.Data[keyField])
	if privateKey == "" {
		conditions.MarkFalse(
			s.scope.HCloudMachine,
			infrav1.SSHPrivateKeyAvailableCondition,
			infrav1.SSHPrivateKeyFieldEmptyReason,
			clusterv1.ConditionSeverityError,
			"key %q in secret %q is missing or empty", keyField, robotSecretName,
		)
		return "", fmt.Errorf("%w: private key field empty", errSSHKeyMisconfigured)
	}
	return privateKey, nil
}

func (s *Service) getRescueSSHClient(ctx context.Context) (sshclient.Client, error) {
	if s.scope.SSHClientFactory == nil {
		return nil, errors.New("SSHClientFactory is nil on machine scope")
	}
	privateKey, err := s.getRescueSSHPrivateKey(ctx)
	if err != nil {
		return nil, err
	}
	hm := s.scope.HCloudMachine
	if len(hm.Status.Addresses) == 0 {
		return nil, errors.New("HCloudMachine.Status.Addresses empty; cannot SSH")
	}
	return s.scope.SSHClientFactory.NewClient(sshclient.Input{
		IP:         hm.Status.Addresses[0].Address,
		PrivateKey: privateKey,
		Port:       22,
	}), nil
}

func (s *Service) failImageURLProvisioning(msg string) (reconcile.Result, error) {
	hm := s.scope.HCloudMachine
	hm.SetBootState(infrav1.HCloudBootStateProvisioningFailed)
	s.scope.SetReady(false)
	s.scope.SetError(msg, capierrors.CreateMachineError)
	conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition, infrav1.BootStateTimedOutReason,
		clusterv1.ConditionSeverityError, "%s", msg)
	record.Warn(hm, "ImageURLProvisioningFailed", msg)
	return reconcile.Result{}, nil
}

func (s *Service) updateStatusPreserveBoot(server *hcloud.Server, failureDomain string) {
	hm := s.scope.HCloudMachine
	bootState := hm.Status.BootState
	bootSince := hm.Status.BootStateSince
	externalIDs := hm.Status.ExternalIDs
	conds := hm.Status.Conditions.DeepCopy()
	sshKeys := hm.Status.SSHKeys
	ready := hm.Status.Ready
	failureReason := hm.Status.FailureReason
	failureMessage := hm.Status.FailureMessage

	hm.Status = statusFromHCloudServer(server)
	s.scope.SetRegion(failureDomain)
	hm.Status.BootState = bootState
	hm.Status.BootStateSince = bootSince
	hm.Status.ExternalIDs = externalIDs
	hm.Status.Conditions = conds
	hm.Status.SSHKeys = sshKeys
	hm.Status.Ready = ready
	hm.Status.FailureReason = failureReason
	hm.Status.FailureMessage = failureMessage
}

func failureDomainFromMachine(s *Service) string {
	return string(s.scope.HCloudMachine.Status.Region)
}
