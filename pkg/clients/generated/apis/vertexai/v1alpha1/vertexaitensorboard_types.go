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

type TensorboardEncryptionSpec struct {
	/* Immutable. The Cloud KMS resource identifier of the customer managed encryption key used to protect a resource.
	Has the form: projects/my-project/locations/my-region/keyRings/my-kr/cryptoKeys/my-key. The key needs to be in the same region as where the resource is created. */
	KmsKeyName string `json:"kmsKeyName"`
}

type VertexAITensorboardSpec struct {
	/* Description of this Tensorboard. */
	// +optional
	Description *string `json:"description,omitempty"`

	/* User provided name of this Tensorboard. */
	DisplayName string `json:"displayName"`

	/* Immutable. Customer-managed encryption key spec for a Tensorboard. If set, this Tensorboard and all sub-resources of this Tensorboard will be secured by this key. */
	// +optional
	EncryptionSpec *TensorboardEncryptionSpec `json:"encryptionSpec,omitempty"`

	/* The project that this resource belongs to. */
	ProjectRef v1alpha1.ResourceRef `json:"projectRef"`

	/* Immutable. The region of the tensorboard. eg us-central1. */
	Region string `json:"region"`

	/* Immutable. Optional. The service-generated name of the resource. Used for acquisition only. Leave unset to create a new resource. */
	// +optional
	ResourceID *string `json:"resourceID,omitempty"`
}

type VertexAITensorboardStatus struct {
	/* Conditions represent the latest available observations of the
	   VertexAITensorboard's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`
	/* Consumer project Cloud Storage path prefix used to store blob data, which can either be a bucket or directory. Does not end with a '/'. */
	// +optional
	BlobStoragePathPrefix *string `json:"blobStoragePathPrefix,omitempty"`

	/* The timestamp of when the Tensorboard was created in RFC3339 UTC "Zulu" format, with nanosecond resolution and up to nine fractional digits. */
	// +optional
	CreateTime *string `json:"createTime,omitempty"`

	/* Name of the Tensorboard. */
	// +optional
	Name *string `json:"name,omitempty"`

	/* ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource. */
	// +optional
	ObservedGeneration *int `json:"observedGeneration,omitempty"`

	/* The number of Runs stored in this Tensorboard. */
	// +optional
	RunCount *string `json:"runCount,omitempty"`

	/* The timestamp of when the Tensorboard was last updated in RFC3339 UTC "Zulu" format, with nanosecond resolution and up to nine fractional digits. */
	// +optional
	UpdateTime *string `json:"updateTime,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VertexAITensorboard is the Schema for the vertexai API
// +k8s:openapi-gen=true
type VertexAITensorboard struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VertexAITensorboardSpec   `json:"spec,omitempty"`
	Status VertexAITensorboardStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VertexAITensorboardList contains a list of VertexAITensorboard
type VertexAITensorboardList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VertexAITensorboard `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VertexAITensorboard{}, &VertexAITensorboardList{})
}
