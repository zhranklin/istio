package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"istio.io/istio/pilot/pkg/kube/version"
	"istio.io/istio/pkg/cmd"
	"istio.io/istio/pkg/spiffe"
	"istio.io/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	serverArgs = version.Args{}

	loggingOptions = log.DefaultOptions()

	rootCmd = &cobra.Command{
		Use:   "version-manager",
		Short: "Start Istio sidecar version manager service.",
		Args:  cobra.ExactArgs(0),
		RunE: func(c *cobra.Command, args []string) error {
			cmd.PrintFlags(c.Flags())
			if err := log.Configure(loggingOptions); err != nil {
				return err
			}

			spiffe.SetTrustDomain(serverArgs.ControllerOptions.TrustDomain)

			stop := make(chan struct{})

			managerServer, err := version.NewServer(serverArgs)
			if err != nil {
				return fmt.Errorf("failed to create version-manager service: %v", err)
			}

			if err := managerServer.Start(stop); err != nil {
				return fmt.Errorf("failed to start version-manager service: %v", err)
			}

			cmd.WaitSignal(stop)
			return nil
		},
		SilenceUsage: true,
	}
)

func init() {
	rootCmd.PersistentFlags().StringVar(&serverArgs.KubeConfig, "kubeconfig", "",
		"Use a Kubernetes configuration file instead of in-cluster configuration")
	rootCmd.PersistentFlags().StringVarP(&serverArgs.Namespace, "namespace", "n", "",
		"Select a namespace where the controller resides. If not set, uses ${POD_NAMESPACE} environment variable")
	rootCmd.PersistentFlags().StringVar(&serverArgs.MeshConfigFile, "meshConfig", "/etc/istio/config/mesh",
		fmt.Sprintf("File name for Istio mesh configuration. If not specified, a default mesh will be used."))

	// Config Controller options
	rootCmd.PersistentFlags().StringVarP(&serverArgs.ControllerOptions.WatchedNamespace, "appNamespace",
		"a", metav1.NamespaceAll,
		"Restrict the applications namespace the controller manages; if not set, controller watches all namespaces")
	rootCmd.PersistentFlags().DurationVar(&serverArgs.ControllerOptions.ResyncPeriod, "resync", 60*time.Second,
		"Controller resync interval")
	rootCmd.PersistentFlags().StringVar(&serverArgs.ControllerOptions.DomainSuffix, "domain", "cluster.local",
		"DNS domain suffix")
	rootCmd.PersistentFlags().StringVar(&serverArgs.ControllerOptions.TrustDomain, "trust-domain", "",
		"The domain serves to identify the system with spiffe")
	rootCmd.PersistentFlags().IntVar(&serverArgs.Port, "port", 8080, "Server port")
	// Attach the Istio logging options to the command.
	rootCmd.PersistentFlags().BoolVar(&serverArgs.EnableStatusUpdateTask, "enableStatusUpdateTask", false,
		"if true versionmanager crd will update status in specified interval")
	rootCmd.PersistentFlags().IntVar(&serverArgs.StatusUpdateIntervalSecond, "statusUpdateIntervalSecond", 30,
		"status update task execute interval")
	rootCmd.PersistentFlags().BoolVar(&serverArgs.EnablePodsHashCheckTask, "enablePodsHashCheckTask", true,
		"if true versionmanager crd witl check pods hash in specified interval")
	rootCmd.PersistentFlags().IntVar(&serverArgs.PodsHashCheckIntervalSecond, "podsHashCheckIntervalSecond", 15,
		"pods hash check task execute interval")
	rootCmd.PersistentFlags().BoolVar(&serverArgs.EnableWatchVersionManager, "enableWatchVersionManager", false,
		"if true will watch versionmanager update or create")
	rootCmd.PersistentFlags().IntVar(&serverArgs.UpgradeTimeoutSecond, "upgradeTimeoutSecond", 2,
		"upgrade sidecar request timeout second")
	loggingOptions.AttachCobraFlags(rootCmd)

	cmd.AddFlags(rootCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Errora(err)
		os.Exit(1)
	}
}
