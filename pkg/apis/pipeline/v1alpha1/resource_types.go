/*
Copyright 2019 The Tekton Authors

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

package v1alpha1

import (
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"golang.org/x/xerrors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

// PipelineResourceType represents the type of endpoint the pipelineResource is, so that the
// controller will know this pipelineResource should be fetched and optionally what
// additional metatdata should be provided for it.
type PipelineResourceType string

const (
	// PipelineResourceTypeGit indicates that this source is a GitHub repo.
	PipelineResourceTypeGit PipelineResourceType = "git"

	// PipelineResourceTypeStorage indicates that this source is a storage blob resource.
	PipelineResourceTypeStorage PipelineResourceType = "storage"

	// PipelineResourceTypeImage indicates that this source is a docker Image.
	PipelineResourceTypeImage PipelineResourceType = "image"

	// PipelineResourceTypeCluster indicates that this source is a k8s cluster Image.
	PipelineResourceTypeCluster PipelineResourceType = "cluster"

	// PipelineResourceTypePullRequest indicates that this source is a SCM Pull Request.
	PipelineResourceTypePullRequest PipelineResourceType = "pullRequest"

	// PipelineResourceTypeCloudEvent indicates that this source is a cloud event URI
	PipelineResourceTypeCloudEvent PipelineResourceType = "cloudEvent"
)

// AllResourceTypes can be used for validation to check if a provided Resource type is one of the known types.
var AllResourceTypes = []PipelineResourceType{PipelineResourceTypeGit, PipelineResourceTypeStorage, PipelineResourceTypeImage, PipelineResourceTypeCluster, PipelineResourceTypePullRequest, PipelineResourceTypeCloudEvent}

// PipelineResourceInterface interface to be implemented by different PipelineResource types
type PipelineResourceInterface interface {
	GetName() string
	GetType() PipelineResourceType
	Replacements() map[string]string
	GetOutputTaskModifier(ts *TaskSpec, path string) (TaskModifier, error)
	GetInputTaskModifier(ts *TaskSpec, path string) (TaskModifier, error)
}

// TaskModifier is an interface to be implemented by different PipelineResources
type TaskModifier interface {
	GetStepsToPrepend() []Step
	GetStepsToAppend() []Step
	GetVolumes() []v1.Volume
}

// InternalTaskModifier implements TaskModifier for resources that are built-in to Tekton Pipelines.
type InternalTaskModifier struct {
	StepsToPrepend []Step
	StepsToAppend  []Step
	Volumes        []v1.Volume
}

// GetStepsToPrepend returns a set of Steps to prepend to the Task.
func (tm *InternalTaskModifier) GetStepsToPrepend() []Step {
	return tm.StepsToPrepend
}

// GetStepsToPrepend returns a set of Steps to append to the Task.
func (tm *InternalTaskModifier) GetStepsToAppend() []Step {
	return tm.StepsToAppend
}

// GetVolumes returns a set of Volumes to prepend to the Task pod.
func (tm *InternalTaskModifier) GetVolumes() []v1.Volume {
	return tm.Volumes
}

// ApplyTaskModifier applies a modifier to the task by appending and prepending steps and volumes.
func ApplyTaskModifier(ts *TaskSpec, tm TaskModifier) {
	steps := tm.GetStepsToPrepend()
	ts.Steps = append(steps, ts.Steps...)

	steps = tm.GetStepsToAppend()
	ts.Steps = append(ts.Steps, steps...)

	volumes := tm.GetVolumes()
	ts.Volumes = append(ts.Volumes, volumes...)
}

// SecretParam indicates which secret can be used to populate a field of the resource
type SecretParam struct {
	FieldName  string `json:"fieldName"`
	SecretKey  string `json:"secretKey"`
	SecretName string `json:"secretName"`
}

// PipelineResourceSpec defines  an individual resources used in the pipeline.
type PipelineResourceSpec struct {
	Type   PipelineResourceType `json:"type"`
	Params []ResourceParam      `json:"params"`
	// Secrets to fetch to populate some of resource fields
	// +optional
	SecretParams []SecretParam `json:"secrets,omitempty"`
}

// PipelineResourceStatus does not contain anything because Resources on their own
// do not have a status, they just hold data which is later used by PipelineRuns
// and TaskRuns.
type PipelineResourceStatus struct {
}

// Check that PipelineResource may be validated and defaulted.
var _ apis.Validatable = (*PipelineResource)(nil)
var _ apis.Defaultable = (*PipelineResource)(nil)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineResource describes a resource that is an input to or output from a
// Task.
//
// +k8s:openapi-gen=true
type PipelineResource struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the desired state of the PipelineResource from the client
	// +optional
	Spec PipelineResourceSpec `json:"spec,omitempty"`
	// Status communicates the observed state of the PipelineResource from the controller
	// +optional
	Status PipelineResourceStatus `json:"status,omitempty"`
}

// PipelineResourceBinding connects a reference to an instance of a PipelineResource
// with a PipelineResource dependency that the Pipeline has declared
type PipelineResourceBinding struct {
	// Name is the name of the PipelineResource in the Pipeline's declaration
	Name string `json:"name,omitempty"`
	// ResourceRef is a reference to the instance of the actual PipelineResource
	// that should be used
	ResourceRef PipelineResourceRef `json:"resourceRef,omitempty"`
	// +optional
	// ResourceSpec is specification of a resource that should be created and
	// consumed by the task
	ResourceSpec *PipelineResourceSpec `json:"resourceSpec,omitempty"`
}

// PipelineResourceResult used to export the image name and digest as json
type PipelineResourceResult struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineResourceList contains a list of PipelineResources
type PipelineResourceList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PipelineResource `json:"items"`
}

// ResourceDeclaration defines an input or output PipelineResource declared as a requirement
// by another type such as a Task or Condition. The Name field will be used to refer to these
// PipelineResources within the type's definition, and when provided as an Input, the Name will be the
// path to the volume mounted containing this PipelineResource as an input (e.g.
// an input Resource named `workspace` will be mounted at `/workspace`).
type ResourceDeclaration struct {
	// Name declares the name by which a resource is referenced in the
	// definition. Resources may be referenced by name in the definition of a
	// Task's steps.
	Name string `json:"name"`
	// Type is the type of this resource;
	Type PipelineResourceType `json:"type"`
	// TargetPath is the path in workspace directory where the resource
	// will be copied.
	// +optional
	TargetPath string `json:"targetPath,omitempty"`
}

// ResourceFromType returns a PipelineResourceInterface from a PipelineResource's type.
func ResourceFromType(r *PipelineResource, images pipeline.Images) (PipelineResourceInterface, error) {
	switch r.Spec.Type {
	case PipelineResourceTypeGit:
		return NewGitResource(images.GitImage, r)
	case PipelineResourceTypeImage:
		return NewImageResource(r)
	case PipelineResourceTypeCluster:
		return NewClusterResource(images.KubeconfigWriterImage, r)
	case PipelineResourceTypeStorage:
		return NewStorageResource(images, r)
	case PipelineResourceTypePullRequest:
		return NewPullRequestResource(images.PRImage, r)
	case PipelineResourceTypeCloudEvent:
		return NewCloudEventResource(r)
	}
	return nil, xerrors.Errorf("%s is an invalid or unimplemented PipelineResource", r.Spec.Type)
}
