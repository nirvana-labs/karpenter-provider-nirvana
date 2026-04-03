package operator

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	coreoperator "sigs.k8s.io/karpenter/pkg/operator"

	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/apis/v1alpha1"
	nirvanaclient "github.com/nirvana-labs/karpenter-provider-nirvana/pkg/client"
	nirvanacp "github.com/nirvana-labs/karpenter-provider-nirvana/pkg/cloudprovider"
	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/cooldown"
	"github.com/nirvana-labs/karpenter-provider-nirvana/pkg/providers/instance"
)

const (
	defaultCooldownMultiplier = 2.0
	defaultPollInterval       = 10 * time.Second
	defaultMaxPollDuration    = 30 * time.Minute
	cooldownCleanupInterval   = 5 * time.Minute
)

// Operator holds the Karpenter core operator and all Nirvana-specific components.
type Operator struct {
	*coreoperator.Operator

	CloudProvider    *nirvanacp.CloudProvider
	CooldownManager  *cooldown.Manager
	InstanceProvider *instance.Provider
	NirvanaClient    *nirvanaclient.Client
	Scheme           *runtime.Scheme
}

// NewOperator wraps the Karpenter core operator and wires up all Nirvana-specific
// provider components from environment configuration.
func NewOperator(ctx context.Context, coreOp *coreoperator.Operator) (*Operator, error) {
	apiKey := os.Getenv("NIRVANA_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("NIRVANA_API_KEY environment variable is required")
	}

	clusterID := os.Getenv("NIRVANA_CLUSTER_ID")
	if clusterID == "" {
		return nil, fmt.Errorf("NIRVANA_CLUSTER_ID environment variable is required")
	}

	// Build the Nirvana API client.
	nirvanaClient := nirvanaclient.New(apiKey)
	nirvanaClient.PollInterval = envDuration("NIRVANA_POLL_INTERVAL", defaultPollInterval)
	nirvanaClient.MaxPollDuration = envDuration("NIRVANA_MAX_POLL_DURATION", defaultMaxPollDuration)

	// Build the cooldown manager.
	multiplier := envFloat64("NIRVANA_COOLDOWN_MULTIPLIER", defaultCooldownMultiplier)
	cm := cooldown.NewManager(multiplier)
	cm.StartCleanup(ctx, cooldownCleanupInterval)

	// Build the instance provider.
	ip := instance.New(nirvanaClient, cm, clusterID)

	// Build the cloud provider.
	cp := nirvanacp.New(coreOp.GetClient(), ip)

	// Register CRD types.
	scheme := runtime.NewScheme()
	utilruntime.Must(v1alpha1.AddToScheme(scheme))

	log.Info().
		Str("cluster_id", clusterID).
		Float64("cooldown_multiplier", multiplier).
		Str("poll_interval", nirvanaClient.PollInterval.String()).
		Str("max_poll_duration", nirvanaClient.MaxPollDuration.String()).
		Msg("nirvana operator initialized")

	return &Operator{
		Operator:         coreOp,
		CloudProvider:    cp,
		CooldownManager:  cm,
		InstanceProvider: ip,
		NirvanaClient:    nirvanaClient,
		Scheme:           scheme,
	}, nil
}

func envDuration(key string, defaultVal time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return defaultVal
	}
	return d
}

func envFloat64(key string, defaultVal float64) float64 {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return defaultVal
	}
	return f
}
