/*
Copyright 2025.

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

package controller

import (
	"context"

	webv1 "github.com/vjanz/website-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// WebsiteReconciler reconciles a Website object
type WebsiteReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=web.valon.dev,resources=websites,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=web.valon.dev,resources=websites/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=web.valon.dev,resources=websites/finalizers,verbs=update

// Reconcile is part of the main Kubernetes reconciliation loop.
func (r *WebsiteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1. Fetch the Website instance
	var website webv1.Website
	if err := r.Get(ctx, req.NamespacedName, &website); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Website resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Website")
		return ctrl.Result{}, err
	}

	// 2. Define the desired Deployment object
	desiredDeployment := buildDeployment(&website)

	// 3. Create or Update the Deployment
	if err := controllerutil.SetControllerReference(&website, desiredDeployment, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	var existingDeployment appsv1.Deployment
	err := r.Get(ctx, types.NamespacedName{Name: desiredDeployment.Name, Namespace: desiredDeployment.Namespace}, &existingDeployment)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating Deployment", "deployment", desiredDeployment.Name)
		if err := r.Create(ctx, desiredDeployment); err != nil {
			return ctrl.Result{}, err
		}
	} else if err == nil {
		// Update existing Deployment if spec changed
		desiredDeployment.ResourceVersion = existingDeployment.ResourceVersion
		if err := r.Update(ctx, desiredDeployment); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		return ctrl.Result{}, err
	}

	// 4. Define the desired Service object
	desiredService := buildService(&website)

	// 5. Create or Update the Service
	if err := controllerutil.SetControllerReference(&website, desiredService, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	var existingService corev1.Service
	err = r.Get(ctx, types.NamespacedName{Name: desiredService.Name, Namespace: desiredService.Namespace}, &existingService)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating Service", "service", desiredService.Name)
		if err := r.Create(ctx, desiredService); err != nil {
			return ctrl.Result{}, err
		}
	} else if err == nil {
		desiredService.ResourceVersion = existingService.ResourceVersion
		if err := r.Update(ctx, desiredService); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		return ctrl.Result{}, err
	}

	// 6. Define the desired Ingress object
	desiredIngress := buildIngress(&website)

	// 7. Create or Update the Ingress
	if err := controllerutil.SetControllerReference(&website, desiredIngress, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	var existingIngress networkingv1.Ingress
	err = r.Get(ctx, types.NamespacedName{Name: desiredIngress.Name, Namespace: desiredIngress.Namespace}, &existingIngress)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating Ingress", "ingress", desiredIngress.Name)
		if err := r.Create(ctx, desiredIngress); err != nil {
			return ctrl.Result{}, err
		}
	} else if err == nil {
		desiredIngress.ResourceVersion = existingIngress.ResourceVersion
		if err := r.Update(ctx, desiredIngress); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		return ctrl.Result{}, err
	}

	// 8. Optionally update status (omitted for brevity)

	return ctrl.Result{}, nil
}

func buildDeployment(website *webv1.Website) *appsv1.Deployment {
	labels := map[string]string{"app": website.Name}
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      website.Name + "-deployment",
			Namespace: website.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}
}

func buildService(website *webv1.Website) *corev1.Service {
	labels := map[string]string{"app": website.Name}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      website.Name + "-service",
			Namespace: website.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstrFromInt(80),
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

func buildIngress(website *webv1.Website) *networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      website.Name + "-ingress",
			Namespace: website.Namespace,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: website.Spec.Domain,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: website.Name + "-service",
											Port: networkingv1.ServiceBackendPort{
												Number: 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if website.Spec.EnableTLS {
		ingress.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      []string{website.Spec.Domain},
				SecretName: website.Name + "-tls",
			},
		}
	}

	return ingress
}

// Helper function to convert int to IntOrString type
func intstrFromInt(i int) intstr.IntOrString {
	return intstr.IntOrString{Type: intstr.Int, IntVal: int32(i)}
}

// SetupWithManager sets up the controller with the Manager.
func (r *WebsiteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webv1.Website{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Complete(r)
}
