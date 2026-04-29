package nodeclass

import (
	"context"
	"fmt"

	"github.com/awslabs/operatorpkg/status"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/apis/v1alpha1"
)

type Controller struct {
	kubeClient client.Client
}

func NewController(kubeClient client.Client) *Controller {
	return &Controller{kubeClient: kubeClient}
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		For(&v1alpha1.NirvanaNodeClass{}).
		Named("nodeclass").
		Complete(c)
}

func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	nodeClass := &v1alpha1.NirvanaNodeClass{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, nodeClass); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	if nodeClass.StatusConditions().IsTrue(status.ConditionReady) {
		return reconcile.Result{}, nil
	}
	patch := client.MergeFrom(nodeClass.DeepCopy())
	nodeClass.StatusConditions().SetTrue(status.ConditionReady)
	if err := c.kubeClient.Status().Patch(ctx, nodeClass, patch); err != nil {
		return reconcile.Result{}, fmt.Errorf("patching status, %w", err)
	}
	return reconcile.Result{}, nil
}
