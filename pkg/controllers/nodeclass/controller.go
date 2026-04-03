package nodeclass

import (
	"context"
	"fmt"

	"github.com/awslabs/operatorpkg/status"
	"github.com/rs/zerolog/log"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/apis/v1alpha1"
	nirvanaclient "github.com/nirvana-labs/karpenter-provider-nirvana/pkg/client"
)

// Controller reconciles NirvanaNodeClass objects to keep their status up to date.
type Controller struct {
	kubeClient    client.Client
	nirvanaClient *nirvanaclient.Client
}

// NewController creates a new NirvanaNodeClass controller.
func NewController(kubeClient client.Client, nirvanaClient *nirvanaclient.Client) *Controller {
	return &Controller{
		kubeClient:    kubeClient,
		nirvanaClient: nirvanaClient,
	}
}

// Register implements controller.Controller for the operatorpkg controller interface.
func (c *Controller) Register(_ context.Context, mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NirvanaNodeClass{}).
		Complete(c)
}

// Reconcile updates the NirvanaNodeClass status with current pool information.
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	nodeClass := &v1alpha1.NirvanaNodeClass{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, nodeClass); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	pools, err := c.nirvanaClient.ListPools(ctx, nodeClass.Spec.ClusterID)
	if err != nil {
		log.Error().Err(err).Msg("failed to list pools for status update")
		nodeClass.StatusConditions().SetFalse(status.ConditionReady, "PoolListFailed", err.Error())
		_ = c.kubeClient.Status().Update(ctx, nodeClass)
		return reconcile.Result{}, fmt.Errorf("listing pools: %w", err)
	}

	nodeClass.Status.Pools = make([]v1alpha1.PoolStatus, len(pools))
	for i, pool := range pools {
		nodeClass.Status.Pools[i] = v1alpha1.PoolStatus{
			ID:        pool.ID,
			Name:      pool.Name,
			NodeCount: pool.NodeCount,
			VCPU:      pool.NodeConfig.CPUConfig.VCPU,
			RAMGi:     pool.NodeConfig.MemoryConfig.Size,
			StorageGi: pool.NodeConfig.BootVolume.Size,
			Status:    pool.Status,
		}
	}

	nodeClass.StatusConditions().SetTrue(status.ConditionReady)

	if err := c.kubeClient.Status().Update(ctx, nodeClass); err != nil {
		return reconcile.Result{}, fmt.Errorf("updating status: %w", err)
	}

	log.Info().
		Str("name", nodeClass.Name).
		Int("pools", len(pools)).
		Msg("reconciled NirvanaNodeClass")
	return reconcile.Result{}, nil
}
