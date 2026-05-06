package main

import (
	"context"
	"os"

	"github.com/awslabs/operatorpkg/status"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/metrics"
	corecontrollers "sigs.k8s.io/karpenter/pkg/controllers"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	coreoperator "sigs.k8s.io/karpenter/pkg/operator"

	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/apis/v1alpha1"
	nirvanaclient "github.com/nirvana-labs/karpenter-provider-nirvana/pkg/client"
	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/cloudprovider"
	nodeclasscontroller "github.com/nirvana-labs/karpenter-provider-nirvana/pkg/controllers/nodeclass"
	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/logger"
)

func main() {
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	logJSON := os.Getenv("LOG_JSON") == "true"
	_ = logger.Init(logLevel, logJSON)

	apiKey := os.Getenv("NIRVANA_API_KEY")
	if apiKey == "" {
		log.Fatal().Msg("NIRVANA_API_KEY environment variable is required")
	}

	clusterID := os.Getenv("NIRVANA_CLUSTER_ID")
	if clusterID == "" {
		log.Fatal().Msg("NIRVANA_CLUSTER_ID environment variable is required")
	}

	nirvanaClient := nirvanaclient.New(apiKey)

	cluster, err := nirvanaClient.GetCluster(context.Background(), clusterID)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to fetch cluster from Nirvana API")
	}
	region := cluster.Region

	log.Info().
		Str("cluster_id", clusterID).
		Str("cluster_name", cluster.Name).
		Str("region", region).
		Msg("connected to Nirvana cluster")

	ctx, coreOp := coreoperator.NewOperator()

	if err := v1alpha1.AddToScheme(coreOp.GetScheme()); err != nil {
		log.Fatal().Err(err).Msg("failed to register NirvanaNodeClass scheme")
	}

	nirvanaCloudProvider := cloudprovider.New(nirvanaClient, clusterID, region)
	decoratedCloudProvider := metrics.Decorate(nirvanaCloudProvider)

	clusterState := state.NewCluster(coreOp.Clock, coreOp.GetClient(), decoratedCloudProvider)

	coreOp.
		WithControllers(ctx, corecontrollers.NewControllers(
			ctx,
			coreOp.Manager,
			coreOp.Clock,
			coreOp.GetClient(),
			coreOp.EventRecorder,
			decoratedCloudProvider,
			nirvanaCloudProvider,
			clusterState,
			coreOp.InstanceTypeStore,
		)...).
		WithControllers(ctx,
			nodeclasscontroller.NewController(coreOp.GetClient(), nirvanaClient, clusterID),
			status.NewController[*v1alpha1.NirvanaNodeClass](coreOp.GetClient(), coreOp.Manager.GetEventRecorderFor("nirvana")),
		).
		Start(ctx)

	log.Info().Msg("controller started")
}
