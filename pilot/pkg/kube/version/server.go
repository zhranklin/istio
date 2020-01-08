package version

import (
	"context"
	err "errors"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/go-multierror"
	meshconfig "istio.io/api/mesh/v1alpha1"
	"istio.io/istio/pilot/cmd"
	configaggregate "istio.io/istio/pilot/pkg/config/aggregate"
	"istio.io/istio/pilot/pkg/config/kube/crd/controller"
	"istio.io/istio/pilot/pkg/config/kube/ingress"
	"istio.io/istio/pilot/pkg/model"
	controller2 "istio.io/istio/pilot/pkg/serviceregistry/kube/controller"
	"istio.io/istio/pkg/config/mesh"
	kubelib "istio.io/istio/pkg/kube"
	"istio.io/pkg/env"
	"istio.io/pkg/filewatcher"
	"istio.io/pkg/log"
	"istio.io/pkg/version"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"net/http"
	"reflect"
	"time"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

const (
	// ConfigMapKey should match the expected MeshConfig file name
	ConfigMapKey = "mesh"
)

type Args struct {
	ControllerOptions controller2.Options
	KubeConfig        string
	Namespace         string
	MeshConfigFile    string
	Port              int

	// svm config
	EnableStatusUpdateTask      bool
	StatusUpdateIntervalSecond  int
	EnablePodsHashCheckTask     bool
	EnableWatchVersionManager   bool
	PodsHashCheckIntervalSecond int
	UpgradeTimeoutSecond        int
}

type Server struct {
	kubeClient       kubernetes.Interface
	configController model.ConfigStoreCache
	istioConfigStore model.IstioConfigStore
	httpServer       *http.Server
	handler          *RegexpHanlder
	startFuncs       []startFunc
	mesh             *meshconfig.MeshConfig
	fileWatcher      filewatcher.FileWatcher
	svmClient        *Client
}

var podNamespaceVar = env.RegisterStringVar("POD_NAMESPACE", "", "")

func NewServer(args Args) (*Server, error) {
	if args.Namespace == "" {
		args.Namespace = podNamespaceVar.Get()
	}
	s := &Server{
		fileWatcher: filewatcher.NewWatcher(),
	}

	if err := s.initKubeClient(&args); err != nil {
		return nil, fmt.Errorf("kube client: %v", err)
	}
	if err := s.initMesh(&args); err != nil {
		return nil, fmt.Errorf("mesh: %v", err)
	}
	if err := s.initConfigController(&args); err != nil {
		return nil, fmt.Errorf("config controller: %v", err)
	}
	if err := s.initSvmClient(&args); err != nil {
		return nil, fmt.Errorf("version sync context: %v", err)
	}
	if err := s.initVersionManagerHandler(&args); err != nil {
		return nil, fmt.Errorf("version manager handler: %v", err)
	}
	if err := s.initHttpServer(&args); err != nil {
		return nil, fmt.Errorf("http server: %v", err)
	}
	if err := s.initHttpHandler(&args); err != nil {
		return nil, fmt.Errorf("http server handler: %v", err)
	}
	return s, nil
}

func (s *Server) Start(stop <-chan struct{}) error {
	// Now start all of the components.
	for _, fn := range s.startFuncs {
		if err := fn(stop); err != nil {
			return err
		}
	}

	return nil
}

type startFunc func(stop <-chan struct{}) error

// GetMeshConfig fetches the ProxyMesh configuration from Kubernetes ConfigMap.
func GetMeshConfig(kube kubernetes.Interface, namespace, name string) (*v1.ConfigMap, *meshconfig.MeshConfig, error) {

	if kube == nil {
		defaultMesh := mesh.DefaultMeshConfig()
		return nil, &defaultMesh, nil
	}

	cfg, err := kube.CoreV1().ConfigMaps(namespace).Get(name, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			defaultMesh := mesh.DefaultMeshConfig()
			return nil, &defaultMesh, nil
		}
		return nil, nil, err
	}

	// values in the data are strings, while proto might use a different data type.
	// therefore, we have to get a value by a key
	cfgYaml, exists := cfg.Data[ConfigMapKey]
	if !exists {
		return nil, nil, fmt.Errorf("missing configuration map key %q", ConfigMapKey)
	}

	meshConfig, err := mesh.ApplyMeshConfigDefaults(cfgYaml)
	if err != nil {
		return nil, nil, err
	}
	return cfg, meshConfig, nil
}

// initMesh creates the mesh in the pilotConfig from the input arguments.
func (s *Server) initMesh(args *Args) error {
	var meshConfig *meshconfig.MeshConfig
	var err error

	if args.MeshConfigFile != "" {
		meshConfig, err = cmd.ReadMeshConfig(args.MeshConfigFile)
		if err != nil {
			log.Warnf("failed to read mesh configuration, using default: %v", err)
		}

		// Watch the config file for changes and reload if it got modified
		s.addFileWatcher(args.MeshConfigFile, func() {
			// Reload the config file
			meshConfig, err = cmd.ReadMeshConfig(args.MeshConfigFile)
			if err != nil {
				log.Warnf("failed to read mesh configuration, using default: %v", err)
				return
			}
			if !reflect.DeepEqual(meshConfig, s.mesh) {
				log.Infof("mesh configuration updated to: %s", spew.Sdump(meshConfig))
				if !reflect.DeepEqual(meshConfig.ConfigSources, s.mesh.ConfigSources) {
					log.Infof("mesh configuration sources have changed")
					//TODO Need to re-create or reload initConfigController()
				}
				s.mesh = meshConfig
			}
		})
	}

	if meshConfig == nil {
		// Config file either wasn't specified or failed to load - use a default mesh.
		if _, meshConfig, err = GetMeshConfig(s.kubeClient, controller2.IstioNamespace, controller2.IstioConfigMap); err != nil {
			log.Warnf("failed to read the default mesh configuration: %v, from the %s config map in the %s namespace",
				err, controller2.IstioConfigMap, controller2.IstioNamespace)
			return err
		}
	}

	log.Infof("mesh configuration %s", spew.Sdump(meshConfig))
	log.Infof("version %s", version.Info.String())
	log.Infof("flags %s", spew.Sdump(args))

	s.mesh = meshConfig
	return nil
}

func (s *Server) initKubeClient(args *Args) error {
	client, kuberr := kubelib.CreateClientset(args.KubeConfig, "")
	if kuberr != nil {
		return multierror.Prefix(kuberr, "failed to connect to Kubernetes API.")
	}
	s.kubeClient = client
	return nil
}

func (s *Server) initConfigController(args *Args) error {
	controller, err := s.makeKubeConfigController(args)
	if err != nil {
		return err
	}

	s.configController = controller

	// Defer starting the controller until after the service is created.
	s.addStartFunc(func(stop <-chan struct{}) error {
		go s.configController.Run(stop)
		return nil
	})

	// If running in ingress mode (requires k8s), wrap the config controller.
	if s.mesh.IngressControllerMode != meshconfig.MeshConfig_OFF {
		// Wrap the config controller with a cache.
		configController, err := configaggregate.MakeCache([]model.ConfigStoreCache{
			s.configController,
			ingress.NewController(s.kubeClient, s.mesh, args.ControllerOptions),
		})
		if err != nil {
			return err
		}

		// Update the config controller
		s.configController = configController

		if ingressSyncer, errSyncer := ingress.NewStatusSyncer(s.mesh, s.kubeClient,
			args.Namespace, args.ControllerOptions); errSyncer != nil {
			log.Warnf("Disabled ingress status syncer due to %v", errSyncer)
		} else {
			s.addStartFunc(func(stop <-chan struct{}) error {
				go ingressSyncer.Run(stop)
				return nil
			})
		}
	}

	// Create the config store.
	s.istioConfigStore = model.MakeIstioStore(s.configController)

	return nil
}

func (s *Server) initSvmClient(args *Args) error {
	kubeClient, ok := s.kubeClient.(*kubernetes.Clientset)
	if !ok {
		errMsg := "type assertion failed. original type:[kubernetes.Interface], assert type:[*kubernetes.Clientset]"
		log.Warn(errMsg)
		return err.New(errMsg)
	}
	option := SvmOptions{
		Domain:                      args.ControllerOptions.DomainSuffix,
		KubeClient:                  kubeClient,
		EnableStatusUpdateTask:      args.EnableStatusUpdateTask,
		EnablePodsHashCheckTask:     args.EnablePodsHashCheckTask,
		StatusUpdateIntervalSecond:  args.StatusUpdateIntervalSecond,
		PodsHashCheckIntervalSecond: args.PodsHashCheckIntervalSecond,
		UpgradeTimeoutSecond:        args.UpgradeTimeoutSecond,
	}
	s.svmClient = NewSvmClient(option)
	s.addStartFunc(func(stop <-chan struct{}) error {
		go s.svmClient.Run(stop)
		return nil
	})
	return nil
}

func (s *Server) initVersionManagerHandler(args *Args) error {
	if args.EnableWatchVersionManager {
		s.configController.RegisterEventHandler("version-manager", func(config model.Config, event model.Event) {
			switch event {
			case model.EventAdd:
				log.Infof("add version-manager config: [%v]", config.Spec)
			case model.EventUpdate:
				log.Infof("update version-manager config: [%v]", config.Spec)
			}
			s.svmClient.PushPodsHashCheck(config, NeverRetryPolicy)
		})
	}
	return nil
}

func (s *Server) initHttpServer(args *Args) error {
	s.handler = &RegexpHanlder{}
	mux := http.ServeMux{}
	mux.HandleFunc("/", s.route)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", args.Port),
		Handler: &mux,
	}

	s.addStartFunc(func(stop <-chan struct{}) error {
		go func() {
			if err := server.ListenAndServe(); err != nil {
				log.Warna(err)
			}
		}()
		go func() {
			<-stop
			if err := server.Shutdown(context.TODO()); err != nil {
				log.Warna(err)
			}
		}()
		return nil
	})
	return nil
}

func (s *Server) makeKubeConfigController(args *Args) (model.ConfigStoreCache, error) {
	configClient, err := controller.NewClient(args.KubeConfig, "", model.ConfigDescriptor{model.VersionManager}, args.ControllerOptions.DomainSuffix)
	if err != nil {
		return nil, multierror.Prefix(err, "failed to open a config client.")
	}

	if err = configClient.RegisterResources(); err != nil {
		return nil, multierror.Prefix(err, "failed to register custom resources.")
	}

	return controller.NewController(configClient, args.ControllerOptions), nil
}

func (s *Server) addStartFunc(fn startFunc) {
	s.startFuncs = append(s.startFuncs, fn)
}

func (s *Server) addFileWatcher(file string, callback func()) {
	_ = s.fileWatcher.Add(file)
	go func() {
		var timerC <-chan time.Time
		for {
			select {
			case <-timerC:
				timerC = nil
				callback()
			case <-s.fileWatcher.Events(file):
				// Use a timer to debounce configuration updates
				if timerC == nil {
					timerC = time.After(100 * time.Millisecond)
				}
			}
		}
	}()
}
