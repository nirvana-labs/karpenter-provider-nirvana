package main

import (
	"os"

	"github.com/rs/zerolog/log"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/metrics"
	corecontrollers "sigs.k8s.io/karpenter/pkg/controllers"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	coreoperator "sigs.k8s.io/karpenter/pkg/operator"

	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/apis/v1alpha1"
	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/controllers/nodeclass"
	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/logger"
	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/operator"
)

func main() {
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	logJSON := os.Getenv("LOG_JSON") == "true"
	_ = logger.Init(logLevel, logJSON)

	// Initialize the Karpenter core operator (sets up manager, metrics, health checks, indexers).
	ctx, coreOp := coreoperator.NewOperator()

	// Register our CRD scheme with the core operator's manager.
	v1alpha1.AddToScheme(coreOp.GetScheme())

	// Build the Nirvana-specific operator (API client, cooldown, instance provider, cloud provider).
	nirvanaOp, err := operator.NewOperator(ctx, coreOp)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to create nirvana operator")
	}

	// Wrap our cloud provider with Karpenter's metrics decorator.
	cloudProvider := metrics.Decorate(nirvanaOp.CloudProvider)

	// Build the cluster state tracker that Karpenter core uses for scheduling decisions.
	clusterState := state.NewCluster(coreOp.Clock, coreOp.GetClient(), cloudProvider)

	// Register Karpenter core controllers (provisioning, disruption, termination, drift, etc.).
	coreOp.
		WithControllers(ctx, corecontrollers.NewControllers(
			ctx,
			coreOp.Manager,
			coreOp.Clock,
			coreOp.GetClient(),
			coreOp.EventRecorder,
			cloudProvider,
			nirvanaOp.CloudProvider,
			clusterState,
			coreOp.InstanceTypeStore,
		)...).
		WithControllers(ctx, nodeclass.NewController(coreOp.GetClient(), nirvanaOp.NirvanaClient)).
		Start(ctx)
}
