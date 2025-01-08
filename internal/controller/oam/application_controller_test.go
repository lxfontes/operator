/*
Copyright 2024.

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

package oam

import (
	"context"
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	coreoamv1beta1 "go.wasmcloud.dev/operator/api/oam/core/v1beta1"
	"go.wasmcloud.dev/x/wasmbus/wadm"
)

var _ = Describe("Application Controller", func() {
	Context("When reconciling a resource", func() {

		ctx := context.Background()

		BeforeEach(func() {

		})

		AfterEach(func() {
		})

		It("should successfully reconcile the resource", func() {
			const resourceName = "test-reconcile"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}
			By("creating the custom resource for the Kind Application")
			resource := &coreoamv1beta1.Application{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err != nil && errors.IsNotFound(err) {
				wadmManifest, err := wadm.LoadManifest("testdata/application.yaml")
				Expect(err).NotTo(HaveOccurred())
				rawManifest, err := wadmManifest.ToJSON()
				Expect(err).NotTo(HaveOccurred())
				err = json.Unmarshal(rawManifest, resource)
				Expect(err).NotTo(HaveOccurred())

				resource.Name = resourceName
				resource.Namespace = "default"

				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("Reconciling the created resource")
			controllerReconciler := &ApplicationReconciler{
				Client:  k8sClient,
				Scheme:  k8sClient.Scheme(),
				Bus:     bus,
				Lattice: "default",
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			Expect(resource.Status.ObservedVersion).To(Not(Equal("")))
			Expect(resource.Status.ObservedGeneration).To(Equal(resource.Generation))
			Expect(resource.Status.Phase).NotTo(Equal(coreoamv1beta1.ApplicationPhase("")))

			By("Deleting resource")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should update the resource status", func() {
			const resourceName = "test-update"
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}
			By("creating the custom resource for the Kind Application")
			resource := &coreoamv1beta1.Application{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err != nil && errors.IsNotFound(err) {
				wadmManifest, err := wadm.LoadManifest("testdata/application.yaml")
				Expect(err).NotTo(HaveOccurred())
				rawManifest, err := wadmManifest.ToJSON()
				Expect(err).NotTo(HaveOccurred())
				err = json.Unmarshal(rawManifest, resource)
				Expect(err).NotTo(HaveOccurred())

				resource.Name = resourceName
				resource.Namespace = "default"

				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("Reconciling the created resource until it is ready")
			controllerReconciler := &ApplicationReconciler{
				Client:  k8sClient,
				Scheme:  k8sClient.Scheme(),
				Bus:     bus,
				Lattice: "default",
			}

			attempts := 15
			deployed := false
			for i := 0; i < attempts; i++ {
				<-time.After(5 * time.Second)
				_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				err = k8sClient.Get(ctx, typeNamespacedName, resource)
				Expect(err).NotTo(HaveOccurred())
				if resource.Status.Phase == coreoamv1beta1.ApplicationPhase("deployed") {
					deployed = true
					break
				}
			}
			Expect(deployed).To(BeTrue())
			// component + provider + link
			Expect(resource.Status.ScalerStatus).To(HaveLen(3))

			By("Deleting resource")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
	})
})
