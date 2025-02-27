// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// ----------------------------------------------------------------------------
//
//     ***     AUTO GENERATED CODE    ***    AUTO GENERATED CODE     ***
//
// ----------------------------------------------------------------------------
//
//     This file is automatically generated by Config Connector and manual
//     changes will be clobbered when the file is regenerated.
//
// ----------------------------------------------------------------------------

// *** DISCLAIMER ***
// Config Connector's go-client for CRDs is currently in ALPHA, which means
// that future versions of the go-client may include breaking changes.
// Please try it out and give us feedback!

package v1alpha1

import (
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BackupplanBackupConfig struct {
	/* If True, include all namespaced resources. */
	// +optional
	AllNamespaces *bool `json:"allNamespaces,omitempty"`

	/* This defines a customer managed encryption key that will be used to encrypt the "config"
	portion (the Kubernetes resources) of Backups created via this plan. */
	// +optional
	EncryptionKey *BackupplanEncryptionKey `json:"encryptionKey,omitempty"`

	/* This flag specifies whether Kubernetes Secret resources should be included
	when they fall into the scope of Backups. */
	// +optional
	IncludeSecrets *bool `json:"includeSecrets,omitempty"`

	/* This flag specifies whether volume data should be backed up when PVCs are
	included in the scope of a Backup. */
	// +optional
	IncludeVolumeData *bool `json:"includeVolumeData,omitempty"`

	/* A list of namespaced Kubernetes Resources. */
	// +optional
	SelectedApplications *BackupplanSelectedApplications `json:"selectedApplications,omitempty"`

	/* If set, include just the resources in the listed namespaces. */
	// +optional
	SelectedNamespaces *BackupplanSelectedNamespaces `json:"selectedNamespaces,omitempty"`
}

type BackupplanBackupSchedule struct {
	/* A standard cron string that defines a repeating schedule for
	creating Backups via this BackupPlan.
	If this is defined, then backupRetainDays must also be defined. */
	// +optional
	CronSchedule *string `json:"cronSchedule,omitempty"`

	/* This flag denotes whether automatic Backup creation is paused for this BackupPlan. */
	// +optional
	Paused *bool `json:"paused,omitempty"`
}

type BackupplanEncryptionKey struct {
	/* Google Cloud KMS encryption key. Format: projects/* /locations/* /keyRings/* /cryptoKeys/*. */
	GcpKmsEncryptionKey string `json:"gcpKmsEncryptionKey"`
}

type BackupplanNamespacedNames struct {
	/* The name of a Kubernetes Resource. */
	Name string `json:"name"`

	/* The namespace of a Kubernetes Resource. */
	Namespace string `json:"namespace"`
}

type BackupplanRetentionPolicy struct {
	/* Minimum age for a Backup created via this BackupPlan (in days).
	Must be an integer value between 0-90 (inclusive).
	A Backup created under this BackupPlan will not be deletable
	until it reaches Backup's (create time + backup_delete_lock_days).
	Updating this field of a BackupPlan does not affect existing Backups.
	Backups created after a successful update will inherit this new value. */
	// +optional
	BackupDeleteLockDays *int `json:"backupDeleteLockDays,omitempty"`

	/* The default maximum age of a Backup created via this BackupPlan.
	This field MUST be an integer value >= 0 and <= 365. If specified,
	a Backup created under this BackupPlan will be automatically deleted
	after its age reaches (createTime + backupRetainDays).
	If not specified, Backups created under this BackupPlan will NOT be
	subject to automatic deletion. Updating this field does NOT affect
	existing Backups under it. Backups created AFTER a successful update
	will automatically pick up the new value.
	NOTE: backupRetainDays must be >= backupDeleteLockDays.
	If cronSchedule is defined, then this must be <= 360 * the creation interval.]. */
	// +optional
	BackupRetainDays *int `json:"backupRetainDays,omitempty"`

	/* This flag denotes whether the retention policy of this BackupPlan is locked.
	If set to True, no further update is allowed on this policy, including
	the locked field itself. */
	// +optional
	Locked *bool `json:"locked,omitempty"`
}

type BackupplanSelectedApplications struct {
	/* A list of namespaced Kubernetes resources. */
	NamespacedNames []BackupplanNamespacedNames `json:"namespacedNames"`
}

type BackupplanSelectedNamespaces struct {
	/* A list of Kubernetes Namespaces. */
	Namespaces []string `json:"namespaces"`
}

type GKEBackupBackupPlanSpec struct {
	/* Defines the configuration of Backups created via this BackupPlan. */
	// +optional
	BackupConfig *BackupplanBackupConfig `json:"backupConfig,omitempty"`

	/* Defines a schedule for automatic Backup creation via this BackupPlan. */
	// +optional
	BackupSchedule *BackupplanBackupSchedule `json:"backupSchedule,omitempty"`

	/* Immutable. The source cluster from which Backups will be created via this BackupPlan. */
	Cluster string `json:"cluster"`

	/* This flag indicates whether this BackupPlan has been deactivated.
	Setting this field to True locks the BackupPlan such that no further updates will be allowed
	(except deletes), including the deactivated field itself. It also prevents any new Backups
	from being created via this BackupPlan (including scheduled Backups). */
	// +optional
	Deactivated *bool `json:"deactivated,omitempty"`

	/* User specified descriptive string for this BackupPlan. */
	// +optional
	Description *string `json:"description,omitempty"`

	/* Immutable. The region of the Backup Plan. */
	Location string `json:"location"`

	/* The project that this resource belongs to. */
	ProjectRef v1alpha1.ResourceRef `json:"projectRef"`

	/* Immutable. Optional. The name of the resource. Used for creation and acquisition. When unset, the value of `metadata.name` is used as the default. */
	// +optional
	ResourceID *string `json:"resourceID,omitempty"`

	/* RetentionPolicy governs lifecycle of Backups created under this plan. */
	// +optional
	RetentionPolicy *BackupplanRetentionPolicy `json:"retentionPolicy,omitempty"`
}

type GKEBackupBackupPlanStatus struct {
	/* Conditions represent the latest available observations of the
	   GKEBackupBackupPlan's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`
	/* etag is used for optimistic concurrency control as a way to help prevent simultaneous
	updates of a backup plan from overwriting each other. It is strongly suggested that
	systems make use of the 'etag' in the read-modify-write cycle to perform BackupPlan updates
	in order to avoid race conditions: An etag is returned in the response to backupPlans.get,
	and systems are expected to put that etag in the request to backupPlans.patch or
	backupPlans.delete to ensure that their change will be applied to the same version of the resource. */
	// +optional
	Etag *string `json:"etag,omitempty"`

	/* ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource. */
	// +optional
	ObservedGeneration *int `json:"observedGeneration,omitempty"`

	/* The number of Kubernetes Pods backed up in the last successful Backup created via this BackupPlan. */
	// +optional
	ProtectedPodCount *int `json:"protectedPodCount,omitempty"`

	/* The State of the BackupPlan. */
	// +optional
	State *string `json:"state,omitempty"`

	/* Detailed description of why BackupPlan is in its current state. */
	// +optional
	StateReason *string `json:"stateReason,omitempty"`

	/* Server generated, unique identifier of UUID format. */
	// +optional
	Uid *string `json:"uid,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GKEBackupBackupPlan is the Schema for the gkebackup API
// +k8s:openapi-gen=true
type GKEBackupBackupPlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GKEBackupBackupPlanSpec   `json:"spec,omitempty"`
	Status GKEBackupBackupPlanStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GKEBackupBackupPlanList contains a list of GKEBackupBackupPlan
type GKEBackupBackupPlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GKEBackupBackupPlan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GKEBackupBackupPlan{}, &GKEBackupBackupPlanList{})
}
