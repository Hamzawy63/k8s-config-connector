// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package configconnectorcontext

import (
	"context"
	"fmt"
	"strings"

	customizev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/operator/pkg/apis/core/customize/v1beta1"
	corev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/operator/pkg/apis/core/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/operator/pkg/controllers"
	"github.com/GoogleCloudPlatform/k8s-config-connector/operator/pkg/k8s"
	cnrmmanifest "github.com/GoogleCloudPlatform/k8s-config-connector/operator/pkg/manifest"
	"github.com/GoogleCloudPlatform/k8s-config-connector/operator/pkg/preflight"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/cluster"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/jitter"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
)

const controllerName = "configconnectorcontext-controller"

// ConfigConnectorContextReconciler reconciles a ConfigConnectorContext object.
//
// From the high level, the ConfigConnectorContextReconciler watches `ConfigConnectorContext` kind
// and is responsible for managing the lifecycle of per-namespace KCC components (e.g. Service, StatefulSet, ServiceAccount and RoleBindings)
// independently with multiple workers.
// ConfigConnectorContextReconciler also watches "NamespacedControllerResource" kind and apply
// customizations specified in "NamespacedControllerResource" CRs to per-namespace KCC components.
type ConfigConnectorContextReconciler struct {
	reconciler           *declarative.Reconciler
	client               client.Client
	recorder             record.EventRecorder
	labelMaker           declarative.LabelMaker
	log                  logr.Logger
	customizationWatcher *controllers.CustomizationWatcher
}

func Add(mgr ctrl.Manager, repoPath string) error {
	r, err := newReconciler(mgr, repoPath)
	if err != nil {
		return err
	}

	// Create a new ConfigConnectorContext controller.
	obj := &corev1beta1.ConfigConnectorContext{}
	_, err = builder.
		ControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(controller.Options{MaxConcurrentReconciles: 20}).
		WatchesRawSource(&source.Channel{Source: r.customizationWatcher.Events()}, &handler.EnqueueRequestForObject{}).
		For(obj, builder.OnlyMetadata).
		Build(r)
	if err != nil {
		return err
	}

	return nil
}

func newReconciler(mgr ctrl.Manager, repoPath string) (*ConfigConnectorContextReconciler, error) {
	repo := cnrmmanifest.NewLocalRepository(repoPath)
	manifestLoader := cnrmmanifest.NewPerNamespaceManifestLoader(repo)
	preflight := preflight.NewCompositePreflight([]declarative.Preflight{
		preflight.NewNameChecker(mgr.GetClient(), k8s.ConfigConnectorContextAllowedName),
		preflight.NewUpgradeChecker(mgr.GetClient(), repo),
		preflight.NewConfigConnectorContextChecker(),
	})

	r := &ConfigConnectorContextReconciler{
		reconciler: &declarative.Reconciler{},
		client:     mgr.GetClient(),
		recorder:   mgr.GetEventRecorderFor(controllerName),
		labelMaker: SourceLabel(),
		log:        ctrl.Log.WithName(controllerName),
	}

	r.customizationWatcher = controllers.NewWithDynamicClient(
		dynamic.NewForConfigOrDie(mgr.GetConfig()),
		controllers.CustomizationWatcherOptions{
			TriggerGVRs: controllers.NamespacedCustomizationCRsToWatch,
			Log:         r.log,
		})

	err := r.reconciler.Init(mgr, &corev1beta1.ConfigConnectorContext{},
		declarative.WithPreserveNamespace(),
		declarative.WithManifestController(manifestLoader),
		declarative.WithObjectTransform(r.transformNamespacedComponents()),
		declarative.WithObjectTransform(r.addLabels()),
		declarative.WithObjectTransform(r.handleCCContextLifecycle()),
		declarative.WithObjectTransform(r.applyNamespacedCustomizations()),
		declarative.WithStatus(&declarative.StatusBuilder{
			PreflightImpl: preflight,
		}),
	)
	return r, err
}

func (r *ConfigConnectorContextReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	r.log.Info("reconciling ConfigConnectorContext", "name", req.Name, "namespace", req.Namespace)
	_, err := r.getConfigConnectorContext(ctx, req.NamespacedName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Info("ConfigConnectorContext not found in API server; skipping the reconciliation", "name", req.NamespacedName)
			return reconcile.Result{}, nil
		}
	}
	_, reconciliationErr := r.reconciler.Reconcile(ctx, req)
	if reconciliationErr != nil {
		if err := r.handleReconcileFailed(ctx, req.NamespacedName, reconciliationErr); err != nil {
			return reconcile.Result{}, fmt.Errorf("error handling reconciled failed: %v, original reconciliation error: %w", err, reconciliationErr)
		}
		return reconcile.Result{}, reconciliationErr
	}
	// Setup watch for customization CRDs if not already done so in the previous reconciliations.
	// When there is a change detected on a customization CR, raises an event on ConfigConnectorContext CR.
	if err := r.customizationWatcher.EnsureWatchStarted(ctx, req.NamespacedName); err != nil {
		r.log.Error(err, "ensure watch start for customization CRDs failed")
		// Don't fail entire reconciliation if we cannot start watch for customization CRDs.
		// return reconcile.Result{}, err
	}
	jitteredPeriod := jitter.GenerateWatchJitteredTimeoutPeriod()
	r.log.Info("successfully finished reconcile", "ConfigConnectorContext", req.NamespacedName, "time to next reconciliation", jitteredPeriod)
	return reconcile.Result{RequeueAfter: jitteredPeriod}, r.handleReconcileSucceeded(ctx, req.NamespacedName)
}

func (r *ConfigConnectorContextReconciler) getConfigConnectorContext(ctx context.Context, nn types.NamespacedName) (*corev1beta1.ConfigConnectorContext, error) {
	ccc := &corev1beta1.ConfigConnectorContext{}
	if err := r.client.Get(ctx, nn, ccc); err != nil {
		return nil, err
	}
	return ccc, nil
}

func (r *ConfigConnectorContextReconciler) handleReconcileFailed(ctx context.Context, nn types.NamespacedName, reconcileErr error) error {
	ccc, err := r.getConfigConnectorContext(ctx, nn)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Info("ConfigConnectorContext not found in API server; skipping the handling of failed reconciliation", "namespace", nn.Namespace, "name", nn.Name)
			return nil
		}
		r.log.Info("error getting ConfigConnectorContext object", "namespace", nn.Namespace, "name", nn.Name, "reconcile error", reconcileErr)
		return fmt.Errorf("error getting ConfigConnectorContext object %v/%v: %w", nn.Namespace, nn.Name, err)
	}

	msg := fmt.Sprintf(k8s.ReconcileErrMsgTmpl, reconcileErr)
	r.recorder.Event(ccc, corev1.EventTypeWarning, k8s.UpdateFailed, msg)
	r.log.Info("surfacing error messages in status...", "namespace", nn.Namespace, "name", nn.Name, "error", msg)
	ccc.SetCommonStatus(v1alpha1.CommonStatus{
		Healthy: false,
		Errors:  []string{msg},
	})
	return r.updateConfigConnectorContextStatus(ctx, ccc)
}

func (r *ConfigConnectorContextReconciler) handleReconcileSucceeded(ctx context.Context, nn types.NamespacedName) error {
	ccc, err := r.getConfigConnectorContext(ctx, nn)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Info("ConfigConnectorContext not found in API server; skipping the handling of successful reconciliation", "namespace", nn.Namespace, "name", nn.Name)
			return nil
		}
		return fmt.Errorf("error getting ConfigConnectorContext object %v/%v: %w", nn.Namespace, nn.Name, err)
	}

	r.recorder.Event(ccc, corev1.EventTypeNormal, k8s.UpToDate, k8s.UpToDateMessage)
	ccc.SetCommonStatus(v1alpha1.CommonStatus{
		Healthy: true,
		Errors:  []string{},
	})
	return r.updateConfigConnectorContextStatus(ctx, ccc)
}

func (r *ConfigConnectorContextReconciler) updateConfigConnectorContextStatus(ctx context.Context, ccc *corev1beta1.ConfigConnectorContext) error {
	if err := r.client.Status().Update(ctx, ccc); err != nil {
		return fmt.Errorf("failed to update ConfigConnectorContext %v/%v on API server: %w", ccc.Namespace, ccc.Name, err)
	}
	return nil
}

func (r *ConfigConnectorContextReconciler) transformNamespacedComponents() declarative.ObjectTransform {
	return func(ctx context.Context, o declarative.DeclarativeObject, m *manifest.Objects) error {
		ccc, ok := o.(*corev1beta1.ConfigConnectorContext)
		if !ok {
			return fmt.Errorf("expected the resource to be a ConfigConnectorContext, but it was not. Object: %v", o)
		}
		transformedObjects, err := transformNamespacedComponentTemplates(ctx, r.client, ccc, m.Items)
		if err != nil {
			return fmt.Errorf("error transforming namespaced components: %w", err)
		}
		m.Items = transformedObjects
		return nil
	}
}

// Add labels that will be used for the controller to dynamically watch on deployed KCC components.
func (r *ConfigConnectorContextReconciler) addLabels() declarative.ObjectTransform {
	return func(ctx context.Context, o declarative.DeclarativeObject, manifest *manifest.Objects) error {
		labels := r.labelMaker(ctx, o)
		for _, o := range manifest.Items {
			o.AddLabels(labels)
		}
		return nil
	}
}

// Handle the lifecycle of the given per-namespace components under different conditions:
// 1) If the ConfigConnector object is not found or pending deletion, this is the uninstallation case, finalize the deletion of per-namespace components.
// 2) If the ConfigConnector object says the cluster mode, finalize the deletion of per-namespace components,
//
//	returns some error like “ConfigConnector runs in cluster mode, this CCC is ignored and should be deleted”.
//
// 3) If the ConfigConnector object says the namespaced mode, and if this ConfigConnectorContext object is active, verify that the controller manager workload for the cluster mode is deleted and the ‘cnrm-system’ namespace is created,
//
//	then ensure per-namespace components are created.
//
// 4) If the ConfigConnector object says the namespaced mode, and if this ConfigConnectorContext object is pending deletion, verify that all KCC resource CRs are deleted, then finalize the deletion of per-namespace components.
func (r *ConfigConnectorContextReconciler) handleCCContextLifecycle() declarative.ObjectTransform {
	return func(ctx context.Context, o declarative.DeclarativeObject, m *manifest.Objects) error {
		ccc, ok := o.(*corev1beta1.ConfigConnectorContext)
		if !ok {
			return fmt.Errorf("expected the resource to be a ConfigConnectorContext, but it was not. Object: %v", o)
		}
		var isCCObjectNotFound bool
		cc, err := controllers.GetConfigConnector(ctx, r.client, controllers.ValidConfigConnectorNamespacedName)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("error getting the ConfigConnector object %v: %w", controllers.ValidConfigConnectorNamespacedName, err)
			}
			isCCObjectNotFound = true
		}
		if isCCObjectNotFound || !cc.GetDeletionTimestamp().IsZero() {
			return r.finalizeSystemComponentsDeletion(ctx, ccc, m)
		}
		if cc.GetMode() == k8s.ClusterMode {
			return r.handleCCContextLifecycleForClusterMode(ctx, ccc, m)
		}
		return r.handleCCContextLifecycleForNamespacedMode(ctx, ccc, m)
	}
}

func (r *ConfigConnectorContextReconciler) finalizeCCContextDeletion(ctx context.Context, ccc *corev1beta1.ConfigConnectorContext, m *manifest.Objects) error {
	r.log.Info("ConfigConnectorContext for namespace is marked for deletion; verifying all CNRM resources have been deleted...", "namespace", ccc.Namespace)
	kindToCount, err := getCNRMResourceCounts(ctx, r.client, ccc.Namespace)
	if err != nil {
		return fmt.Errorf("error verifying the Config Connector resource counts in namespace '%v': %w", ccc.Namespace, err)
	}
	if len(kindToCount) > 0 {
		r.log.Info("Cannot finalize deletion of ConfigConnectorContext: there are still Config Connector resource(s) in the namespace.",
			"namespace", ccc.Namespace, "numKindsWithResources", len(kindToCount))
		return formatCNRMResourcesPresentError(kindToCount)
	}
	if err := r.finalizeNamespacedComponentsDeletion(ctx, ccc, m); err != nil {
		return err
	}
	if err := cluster.DeleteNamespaceID(k8s.OperatorNamespaceIDConfigMapNN, r.client, ctx, ccc.Namespace); err != nil {
		return err
	}
	r.log.Info("Successfully finalized ConfigConnectorContext deletion...", "name", ccc.Name, "namespace", ccc.Namespace)
	// Nothing needs to apply when it's a delete ops.
	m.Items = nil
	return nil
}

func (r *ConfigConnectorContextReconciler) finalizeNamespacedComponentsDeletion(ctx context.Context, ccc *corev1beta1.ConfigConnectorContext, m *manifest.Objects) error {
	r.log.Info("finalizing namespaced components deletion...", "namespace", ccc.Namespace)
	if err := removeNamespacedComponents(ctx, r.client, m.Items); err != nil {
		return fmt.Errorf("error finalizing ConfigConnectorContext %v/%v deletion: %v", ccc.Namespace, ccc.Name, err)
	}
	if controllers.RemoveOperatorFinalizer(ccc) {
		if err := r.client.Update(ctx, ccc); err != nil {
			return fmt.Errorf("error removing %v finalizer in ConfigConnectorContext object %v/%v: %v", k8s.OperatorFinalizer, ccc.Namespace, ccc.GetName(), err)
		}
	}
	return nil
}

func (r *ConfigConnectorContextReconciler) handleCCContextLifecycleForClusterMode(ctx context.Context, ccc *corev1beta1.ConfigConnectorContext, m *manifest.Objects) error {
	// On the cluster mode, clean up namespaced components associated with the ConfigConnectorContext.
	if err := r.finalizeNamespacedComponentsDeletion(ctx, ccc, m); err != nil {
		return err
	}
	return fmt.Errorf("ConfigConnector is in cluster-mode, this ConfigConnectorContext object does not serve any purpose and should be removed")
}

func (r *ConfigConnectorContextReconciler) handleCCContextLifecycleForNamespacedMode(ctx context.Context, ccc *corev1beta1.ConfigConnectorContext, m *manifest.Objects) error {
	if !ccc.GetDeletionTimestamp().IsZero() {
		return r.finalizeCCContextDeletion(ctx, ccc, m)
	}
	// Verify that the controller manager pod for cluster mode is removed, then continue the reconciliation.
	// This is done to avoid having more than one controller reconciling the same object.
	if err := r.verifyControllerManagerPodForClusterModeIsDeleted(ctx); err != nil {
		return err
	}
	if err := r.verifyCNRMSystemNamespaceIsActive(ctx); err != nil {
		return err
	}

	if !controllers.EnsureOperatorFinalizer(ccc) {
		if err := r.client.Update(ctx, ccc); err != nil {
			return fmt.Errorf("error adding %v finalizer in ConfigConnectorContext object %v: %v", k8s.OperatorFinalizer, client.ObjectKeyFromObject(ccc), err)
		}
	}
	return nil
}

func (r *ConfigConnectorContextReconciler) finalizeSystemComponentsDeletion(ctx context.Context, ccc *corev1beta1.ConfigConnectorContext, m *manifest.Objects) error {
	r.log.Info("deleting namespaced components on uninstallation", "namespace", ccc.Namespace)
	if err := r.finalizeNamespacedComponentsDeletion(ctx, ccc, m); err != nil {
		return err
	}
	m.Items = nil
	if ccc.GetDeletionTimestamp().IsZero() {
		return fmt.Errorf("the ConfigConnector object %v is not found or pending deletion; this ConfigConnectorContext object does not serve any purpose and should be removed", controllers.ValidConfigConnectorNamespacedName)
	}
	return nil
}

func (r *ConfigConnectorContextReconciler) verifyControllerManagerPodForClusterModeIsDeleted(ctx context.Context) error {
	sts := &appsv1.StatefulSet{}
	sts.Namespace = k8s.CNRMSystemNamespace
	sts.Name = k8s.KCCControllerManagerComponent
	stsKey := client.ObjectKeyFromObject(sts)

	pod := &corev1.Pod{}
	pod.Namespace = k8s.CNRMSystemNamespace
	pod.Name = k8s.ControllerManagerPodForClusterMode
	podKey := client.ObjectKeyFromObject(pod)

	r.log.Info("verifying that cluster mode workload is deleted...", "StatefulSet", stsKey, "Pod", podKey)
	err := r.client.Get(ctx, stsKey, sts)
	if err == nil {
		return fmt.Errorf("statefulset %v is not yet deleted, reenquee the reconcilation for another attempt later", stsKey)
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("error getting the StatefulSet %v: %w", stsKey, err)
	}

	err = r.client.Get(ctx, podKey, pod)
	if err == nil {
		return fmt.Errorf("pod %v is not yet deleted, reenquee the reconcilation for another attempt later", stsKey)
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("error getting the pod %v: %w", podKey, err)
	}

	return nil
}

func (r *ConfigConnectorContextReconciler) verifyCNRMSystemNamespaceIsActive(ctx context.Context) error {
	r.log.Info("verifying that ConfigConnector system namespace is active...", "system namespace", k8s.CNRMSystemNamespace)
	n := &corev1.Namespace{}
	key := types.NamespacedName{
		Name: k8s.CNRMSystemNamespace,
	}
	if err := r.client.Get(ctx, key, n); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("ConfigConnector system namespace %v is not created by configconnector controller yet, reenquee the reconcilation for another attempt later", k8s.CNRMSystemNamespace)
		} else {
			return fmt.Errorf("error getting the ConfigConnector system namespace %v: %w", key, err)
		}
	}
	if !n.GetDeletionTimestamp().IsZero() {
		return fmt.Errorf("ConfigConnector system namespace %v is pending deletion, stop the reconcilation", k8s.CNRMSystemNamespace)
	}
	return nil
}

// applyNamespacedCustomizations fetches and applies all namespace-scoped customization CRDs.
func (r *ConfigConnectorContextReconciler) applyNamespacedCustomizations() declarative.ObjectTransform {
	return func(ctx context.Context, o declarative.DeclarativeObject, m *manifest.Objects) error {
		ccc, ok := o.(*corev1beta1.ConfigConnectorContext)
		if !ok {
			return fmt.Errorf("expected the resource to be a ConfigConnectorContext, but it was not. Object: %v", o)
		}
		// List all the customization CRs in the same namespace as ConfigConnectorContext object.
		crs, err := controllers.ListNamespacedControllerResources(ctx, r.client, ccc.Namespace)
		if err != nil {
			return err
		}
		// Apply all the customization CRs in the same namespace as ConfigConnectorContext object.
		for _, cr := range crs {
			if cr.Namespace != ccc.Namespace {
				// this shouldn't happen!
				r.log.Error(fmt.Errorf("unexpected namespace for NamespacedControllerResource object"), "expected namespace", ccc.Namespace, "got namespace", cr.Namespace)
			}
			r.log.Info("applying namespace-scoped controller resource customization", "Namespace", cr.Namespace, "Name", cr.Name)
			if err := r.applyNamespacedControllerResourceCustomization(ctx, &cr, m); err != nil {
				return err
			}
		}
		return nil
	}
}

// applyNamespacedControllerResourceCustomization applies customizations specified in NamespacedControllerResource CR.
func (r *ConfigConnectorContextReconciler) applyNamespacedControllerResourceCustomization(ctx context.Context, cr *customizev1beta1.NamespacedControllerResource, m *manifest.Objects) error {
	if cr.Name != "cnrm-controller-manager" {
		msg := fmt.Sprintf("resource customization for controller %s is not supported", cr.Name)
		r.log.Info(msg)
		return r.handleApplyCustomizationFailed(ctx, cr.Namespace, cr.Name, msg)
	}
	controllerGVK := schema.GroupVersionKind{
		Group:   appsv1.SchemeGroupVersion.Group,
		Version: appsv1.SchemeGroupVersion.Version,
		Kind:    "StatefulSet",
	}
	if err := controllers.ApplyContainerResourceCustomization(true, m, cr.Name, controllerGVK, cr.Spec.Containers, nil); err != nil {
		r.log.Error(err, "failed to apply customization", "Namespace", cr.Namespace, "Name", cr.Name)
		return r.handleApplyCustomizationFailed(ctx, cr.Namespace, cr.Name, fmt.Sprintf("failed to apply customization %s: %v", cr.Name, err))
	}
	return r.handleApplyCustomizationSucceeded(ctx, cr.Namespace, cr.Name)
}

func (r *ConfigConnectorContextReconciler) handleApplyCustomizationFailed(ctx context.Context, namespace, name string, msg string) error {
	cr, err := controllers.GetNamespacedControllerResource(ctx, r.client, namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Info("NamespacedControllerResource object not found; skipping the handling of failed customization apply", "namespace", namespace, "name", name)
			return nil
		}
		// Don't fail entire reconciliation if we cannot get NamespacedControllerResource object.
		// return fmt.Errorf("error getting NamespacedControllerResource object %v/%v: %v", namespace, name, err)
		r.log.Error(err, "error getting NamespacedControllerResource object %v", "Namespace", namespace, "Name", name)
		return nil
	}
	cr.Status.CommonStatus = v1alpha1.CommonStatus{
		Healthy: false,
		Errors:  []string{msg},
	}
	return r.updateNamespacedControllerResourceStatus(ctx, cr)
}

func (r *ConfigConnectorContextReconciler) handleApplyCustomizationSucceeded(ctx context.Context, namespace, name string) error {
	cr, err := controllers.GetNamespacedControllerResource(ctx, r.client, namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Info("NamespacedControllerResource object not found; skipping the handling of succeeded customization apply", "namespace", namespace, "name", name)
			return nil
		}
		// Don't fail entire reconciliation if we cannot get NamespacedControllerResource object.
		// return fmt.Errorf("error getting NamespacedControllerResource object %v/%v: %v", namespace, name, err)
		r.log.Error(err, "error getting NamespacedControllerResource object %v", "Namespace", namespace, "Name", name)
		return nil
	}
	cr.SetCommonStatus(v1alpha1.CommonStatus{
		Healthy: true,
		Errors:  []string{},
	})
	return r.updateNamespacedControllerResourceStatus(ctx, cr)
}

func (r *ConfigConnectorContextReconciler) updateNamespacedControllerResourceStatus(ctx context.Context, cr *customizev1beta1.NamespacedControllerResource) error {
	if err := r.client.Status().Update(ctx, cr); err != nil {
		r.log.Error(err, "failed to update NamespacedControllerResource", "Namespace", cr.Namespace, "Name", cr.Name, "Object", cr)
		// Don't fail entire reconciliation if we cannot update NamespacedControllerResource status.
		// return fmt.Errorf("failed to update NamespacedControllerResource %v/%v: %v", cr.Namespace, cr.Name, err)
	}
	return nil
}

func getCNRMResourceCounts(ctx context.Context, kubeClient client.Client, namespace string) (map[string]int64, error) {
	kindToCount := make(map[string]int64)
	pageToken := ""
	var list []apiextensions.CustomResourceDefinition
	var err error
	for ok := true; ok; ok = pageToken != "" {
		list, pageToken, err = k8s.ListCRDs(ctx, kubeClient, pageToken)
		if err != nil {
			return nil, err
		}
		for _, crd := range list {
			if len(crd.Spec.Versions) == 0 {
				continue
			}
			gvk := schema.GroupVersionKind{
				Group:   crd.Spec.Group,
				Version: crd.Spec.Versions[0].Name,
				Kind:    crd.Spec.Names.Kind,
			}
			count, err := countResourcesForGVK(ctx, kubeClient, namespace, gvk)
			if err != nil {
				return nil, err
			}
			if count == 0 {
				continue
			}
			kindToCount[gvk.Kind] = count
		}
	}
	return kindToCount, nil
}

func countResourcesForGVK(ctx context.Context, kubeClient client.Client, namespace string, gvk schema.GroupVersionKind) (int64, error) {
	listOpts := &client.ListOptions{
		Limit:     1000,
		Namespace: namespace,
		Raw:       &metav1.ListOptions{},
	}
	resourceCount := int64(0)
	for ok := true; ok; ok = listOpts.Continue != "" {
		list := unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk)
		if err := kubeClient.List(ctx, &list, listOpts); err != nil {
			return 0, fmt.Errorf("error listing loadedManifest for gvk '%v': %w", gvk, err)
		}
		resourceCount += int64(len(list.Items))
		listOpts.Continue = list.GetContinue()
	}
	return resourceCount, nil
}

func formatCNRMResourcesPresentError(kindToCount map[string]int64) error {
	totalCount := int64(0)
	kindCountStrings := make([]string, 0, len(kindToCount))
	for kind, count := range kindToCount {
		totalCount += count
		kindCountStrings = append(kindCountStrings, fmt.Sprintf("%v %v(s)", count, kind))
	}
	msg := "cannot finalize deletion until all Config Connector resources in namespace have been removed: there are %v Config Connector resource(s) in namespace (%v)"
	return fmt.Errorf(msg, totalCount, strings.Join(kindCountStrings, ", "))
}
