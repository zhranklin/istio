package version

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"istio.io/api/networking/v1alpha3"
	"istio.io/istio/pilot/pkg/config/kube/crd"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/util/protomarshal"
	"istio.io/pkg/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	DefaultAgentStatusPort = "15020"
	DefaultBinaryPath      = "/usr/local/bin/%v"
	SideCarReloadUrl       = "proxycmd?action=reload&binary=%v"
	SideCarUpdateUrl       = "proxycmd?action=update&binary=%v"
	SideCarVersionUrl      = "proxycmd"
	ProxyName              = "istio-proxy"
)

var NeverRetryPolicy = RetryPolicy{
	NeverRetry:     true,
	RetryTimesLeft: 0,
	RetryInterval:  0,
}

var DefaultRetryPolicy = RetryPolicy{
	NeverRetry:     false,
	RetryTimesLeft: 3,
	RetryInterval:  3 * time.Second,
}

type Client struct {
	*VmClient
	kubeClient          *kubernetes.Clientset
	domain              string
	httpClient          *http.Client
	podsSelector        PodsSelector
	statusUpdateQueue   Queue
	podsHashCheckQueue  Queue
	sidecarUpgradeQueue Queue

	enableStatusUpdateTask  bool
	enablePodsHashCheckTask bool
	statusUpdateInterval    time.Duration
	podsHashCheckInterval   time.Duration
	upgradeTimeoutSecond    time.Duration
}

type Result struct {
	StatusCode int
	Body       string
	Header     http.Header
}

type Status struct {
	PodName    string
	Version    string
	UpdateTime string
	Code       int
	Message    string
	Pod        *v1.Pod
}

type SvmOptions struct {
	Domain                      string
	KubeClient                  *kubernetes.Clientset
	EnableStatusUpdateTask      bool
	EnablePodsHashCheckTask     bool
	StatusUpdateIntervalSecond  int
	PodsHashCheckIntervalSecond int
	UpgradeTimeoutSecond        int
}

func NewSvmClient(options SvmOptions) *Client {
	return &Client{
		VmClient:     NewVmClient(options.KubeClient),
		kubeClient:   options.KubeClient,
		domain:       options.Domain,
		podsSelector: NewPodsSelector(options.KubeClient),
		httpClient: &http.Client{
			Timeout: time.Second * time.Duration(options.UpgradeTimeoutSecond),
		},
		statusUpdateQueue:       NewQueue(),
		sidecarUpgradeQueue:     NewQueue(),
		podsHashCheckQueue:      NewQueue(),
		enableStatusUpdateTask:  options.EnableStatusUpdateTask,
		enablePodsHashCheckTask: options.EnablePodsHashCheckTask,
		statusUpdateInterval:    time.Second * time.Duration(options.StatusUpdateIntervalSecond),
		podsHashCheckInterval:   time.Second * time.Duration(options.PodsHashCheckIntervalSecond),
		upgradeTimeoutSecond:    time.Second * time.Duration(options.UpgradeTimeoutSecond),
	}
}

func (c *Client) Run(stop <-chan struct{}) {
	go c.statusUpdateQueue.Run(stop)
	go c.sidecarUpgradeQueue.Run(stop)
	go c.podsHashCheckQueue.Run(stop)
	if c.enableStatusUpdateTask {
		go c.statusUpdateTask(stop)
	}
	if c.enablePodsHashCheckTask {
		go c.podsHashCheckTask(stop)
	}

}

// pods hash check timer task
func (c *Client) podsHashCheckTask(stop <-chan struct{}) {
	timer := time.Tick(c.podsHashCheckInterval)
	for {
		select {
		case <-stop:
			return
		case <-timer:
			log.Info("start pods hash check task. interval:" + c.podsHashCheckInterval.String())
			vmList, err := c.ListAll()
			if err != nil {
				continue
			}
			for _, vm := range vmList.Items {
				config, err := istioObj2Config(vm, c.domain)
				if err != nil {
					continue
				}
				c.PushPodsHashCheck(*config, NeverRetryPolicy)
			}
		}
	}
}

// status update timer task
func (c *Client) statusUpdateTask(stop <-chan struct{}) {
	timer := time.Tick(c.statusUpdateInterval)
	for {
		select {
		case <-stop:
			return
		case <-timer:
			log.Info("start status update task. interval:" + c.statusUpdateInterval.String())
			vmList, err := c.ListAll()
			if err != nil {
				continue
			}
			for _, vm := range vmList.Items {
				c.PushStatusUpdate(vm.Namespace, NeverRetryPolicy)
			}
		}
	}
}

// upgrade one envoy
func (c *Client) PushSidecarUpgradeQueue(pods *[]v1.Pod, version string, policy RetryPolicy) {
	if pods == nil || version == "" {
		return
	}
	for _, pod := range *pods {
		p := pod
		task := Task{
			handler: func() error {
				err := c.doUpgradeSidecar(&p, version)
				return err
			},
			retryPolicy: policy,
		}
		c.sidecarUpgradeQueue.Push(task)
	}
}

// update pods status
func (c *Client) PushStatusUpdate(namespace string, policy RetryPolicy) {
	task := Task{
		handler: func() error {
			vm, err := c.Get(namespace)
			if err != nil {
				log.Warna(err)
				return err
			}
			spec, err := map2Obj(vm.Spec)
			if err != nil {
				log.Warna(err)
				// do not retry
				return nil
			}
			// check timestamp
			if spec.Status == nil {
				spec.Status = &v1alpha3.Status{
					SyncTime:         "",
					PodVersionStatus: nil,
				}
			}
			layout := "2006-01-02 15:04:05"
			newTimestamp := time.Now()
			oldTimestamp, err := time.ParseInLocation(layout, spec.Status.SyncTime, time.Local)
			//oldTimestamp, err := strconv.ParseInt(spec.Status.SyncTime, 10, 64)
			if spec.Status == nil || newTimestamp.Before(oldTimestamp) {
				// do not retry
				return nil
			}
			status, err := c.ProductStatus(namespace)
			if err != nil {
				log.Warna(err)
				return err
			}
			newStatus := &v1alpha3.Status{
				SyncTime:         newTimestamp.Format(layout),
				PodVersionStatus: make([]*v1alpha3.PodVersionStatus, 0),
			}
			for _, s := range status {
				newStatus.PodVersionStatus = append(newStatus.PodVersionStatus, &v1alpha3.PodVersionStatus{
					PodName:        s.PodName,
					CurrentVersion: s.Version,
					LastUpdateTime: s.UpdateTime,
					StatusCode:     int32(s.Code),
					StatusMessage:  s.Message,
				})
			}
			jsonMap, err := protomarshal.ToJSONMap(newStatus)
			if err != nil {
				log.Warna(err)
				return err
			}
			patchs := make([]rfc6902PatchOperation, 0)
			patchs = append(patchs, rfc6902PatchOperation{
				Op:    "replace",
				Path:  "/spec/status",
				Value: jsonMap,
			})
			_, err = c.Patch(vm.Name, vm.Namespace, patchs)
			return err
		},
		retryPolicy: policy,
	}
	c.statusUpdateQueue.Push(task)
}

// check crd pods hash, if have difference, then upgrade envoy
func (c *Client) PushPodsHashCheck(config model.Config, policy RetryPolicy) {
	task := Task{
		handler: func() error {
			vm, err := config2Spec(config)
			if err != nil {
				log.Warna(err)
				return err
			}
			if vm.SidecarVersionSpec == nil {
				return nil
			}
			namespace := config.Namespace
			if namespace == "" {
				namespace = "default"
			}

			versionsSpec := vm.SidecarVersionSpec
			retryPolicy, err := convertPolicy(vm.RetryPolicy)
			if err != nil {
				log.Warna(err)
				return err
			}
			// to check have update or not
			haveUpdate := false
			podVersionMap := make(map[*[]v1.Pod]string)
			for _, spec := range versionsSpec {
				version := spec.ExpectedVersion
				hash := spec.PodsHash
				// check for updates
				newHash := c.calculateStatusHash(spec)
				if hash == newHash {
					continue
				}
				// update pod hash
				spec.PodsHash = newHash
				haveUpdate = true

				// select pods need to update
				pods, err := c.SelectPods(namespace, spec)
				if err != nil {
					log.Warna(err)
					continue
				}
				if pods == nil {
					continue
				}
				podVersionMap[pods] = version
			}
			if haveUpdate {
				// update crd first. if success, upgrade envoy second
				err := c.updateVersionManager(config)
				if err != nil {
					log.Warna(err)
					return err
				}

				for pods, version := range podVersionMap {
					c.PushSidecarUpgradeQueue(pods, version, retryPolicy)
				}
				c.PushStatusUpdate(namespace, DefaultRetryPolicy)
			}
			return nil
		},
		retryPolicy: policy,
	}
	c.podsHashCheckQueue.Push(task)
}

func (c *Client) SelectPods(namespace string, spec *v1alpha3.SidecarVersionSpec) (*[]v1.Pod, error) {
	switch spec.Selector.(type) {
	case *v1alpha3.SidecarVersionSpec_ViaDeployment:
		v := spec.Selector.(*v1alpha3.SidecarVersionSpec_ViaDeployment)
		pods, err := c.podsSelector.Selector(ViaDeployment{
			name:      v.ViaDeployment.Name,
			namespace: namespace,
		})
		if err != nil {
			log.Warna(err)
			return nil, err
		}
		return pods, nil
	case *v1alpha3.SidecarVersionSpec_ViaService:
		v := spec.Selector.(*v1alpha3.SidecarVersionSpec_ViaService)
		pods, err := c.podsSelector.Selector(ViaService{
			name:      v.ViaService.Name,
			namespace: namespace,
		})
		if err != nil {
			log.Warna(err)
			return nil, err
		}
		return pods, nil
	case *v1alpha3.SidecarVersionSpec_ViaStatefulSet:
		v := spec.Selector.(*v1alpha3.SidecarVersionSpec_ViaStatefulSet)
		pods, err := c.podsSelector.Selector(ViaStatefulSet{
			name:      v.ViaStatefulSet.Name,
			namespace: namespace,
		})
		if err != nil {
			log.Warna(err)
			return nil, err
		}
		return pods, nil
	case *v1alpha3.SidecarVersionSpec_ViaLabelSelector:
		v := spec.Selector.(*v1alpha3.SidecarVersionSpec_ViaLabelSelector)
		pods, err := c.podsSelector.Selector(ViaLabelSelector{
			namespace: namespace,
			labels:    v.ViaLabelSelector.Labels,
		})
		if err != nil {
			log.Warna(err)
			return nil, err
		}
		return pods, nil
	}
	return nil, nil
}

func (c *Client) calculateStatusHash(spec *v1alpha3.SidecarVersionSpec) string {
	//todo: implement
	return time.Now().String()
}

func (c *Client) doUpgradeSidecar(pod *v1.Pod, version string) error {
	if hasProxy(pod, c.GetPilotAgentContainer(pod)) {
		path := fmt.Sprintf(SideCarUpdateUrl, version)
		result, err := c.makeRequest(pod, path)
		if err != nil {
			log.Warnf("make request for pod :[%v] failure, path:[%v], err :[%v]", pod.Name, path, err)
			return err
		}
		log.Infof("make request for pod :[%v] success, path:[%v], result[%v]", pod.Name, path, result)
	}
	return nil
}

func (c *Client) CurrentVersion(pod *v1.Pod) (string, error) {
	s, err := c.GetProxyStatus(pod)
	if err != nil {
		return "", err
	}
	return s.Version, nil
}

func (c *Client) ProductStatus(namespace string) ([]*Status, error) {
	pods, err := c.kubeClient.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		log.Warna(err)
		return nil, err
	}
	svcStatus := make([]*Status, 0)
	for _, pod := range pods.Items {
		//Reduce structure copy for better performance
		p := pod
		s, err := c.GetProxyStatus(&p)
		if err != nil {
			log.Warna(err)
			continue
		}
		svcStatus = append(svcStatus, s)
	}
	return svcStatus, nil
}

func (c *Client) GetProxyStatus(pod *v1.Pod) (*Status, error) {
	if !hasProxy(pod, c.GetPilotAgentContainer(pod)) {
		return nil, errors.New(fmt.Sprintf("does not exist proxy in Pod. Pod:[%v], expected proxy:[%v]", pod.Name, c.GetPilotAgentContainer(pod)))
	}
	result, err := c.makeRequest(pod, SideCarVersionUrl)
	if err != nil {
		log.Warnf("make request for Pod :[%v] failure, path:[%v], err :[%v]", pod.Name, SideCarVersionUrl, err)
		return nil, err
	}
	if result.StatusCode != 200 {
		log.Warnf("make request for Pod :[%v] failure, path:[%v], errorCode :[%v]", pod.Name, SideCarVersionUrl, result.StatusCode)
		return nil, errors.New(fmt.Sprintf("make request for Pod :[%v] failure, path:[%v], errorCode :[%v]", pod.Name, SideCarVersionUrl, result.StatusCode))
	}
	log.Infof("make request for Pod :[%v] success, path:[%v], Result[%v]", pod.Name, SideCarVersionUrl, result)
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(result.Body), &obj); err != nil {
		log.Warna(err)
		return nil, err
	}
	version, ok := obj["binary"].(string)
	updateTime, ok := obj["updateTime"].(string)
	if !ok {
		return nil, errors.New(fmt.Sprintf("the proxycmd return format is not as expected:[%v]", result.Body))
	}
	status := Status{
		Pod:        pod,
		PodName:    pod.Name,
		Version:    version,
		UpdateTime: updateTime,
		Code:       0,  //todo:implement
		Message:    "", //todo:implement
	}
	log.Debugf("proxy status : [%v]", status)
	return &status, nil
}

func (c *Client) GetPilotAgentContainer(pod *v1.Pod) string {
	for _, c := range pod.Spec.Containers {
		switch c.Name {
		case "egressgateway", "ingress", "ingressgateway":
			return c.Name
		}
	}
	return ProxyName
}

func (*Client) CoverVersion(container *v1.Container, version string) {
	if version == "" {
		return
	}
	binaryPath := fmt.Sprintf(DefaultBinaryPath, version)
	for i, arg := range container.Args {
		switch arg {
		case "--binaryPath":
			if i+1 < len(container.Args) && !strings.HasPrefix(container.Args[i+1], "--") {
				container.Args[i+1] = binaryPath
			}
		}
	}
}

func convertPolicy(policy *v1alpha3.RetryPolicy) (RetryPolicy, error) {
	return RetryPolicy{
		NeverRetry:     policy.NeverRetry,
		RetryTimesLeft: int(policy.RetryTime),
		RetryInterval:  time.Second*time.Duration(policy.RetryInterval.Seconds) + time.Nanosecond*time.Duration(policy.RetryInterval.Nanos),
	}, nil
}

func (c *Client) updateVersionManager(config model.Config) error {
	istioObj, err := crd.ConvertConfig(model.VersionManager, config)
	if err != nil {
		log.Warna(err)
		return err
	}
	vm, ok := istioObj.(*crd.VersionManager)
	if !ok {
		return errors.New("type assertion failed. original type:[crd.IstioObj], assert type:[crd.VersionManager]")
	}
	_, err = c.Update(vm)
	return err
}

func (c *Client) makeRequest(pod *v1.Pod, path string) (*Result, error) {
	//todo: set timeout
	url := joinPath(c.getAuthority(pod), path)
	response, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	result := Result{}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	result.Body = string(body)
	result.StatusCode = response.StatusCode
	result.Header = response.Header
	return &result, nil
}

func (c *Client) buildRequestGet(uri string) *rest.Request {
	return c.kubeClient.RESTClient().Get().RequestURI(uri)
}

func (c *Client) getAuthority(pod *v1.Pod) string {
	port := DefaultAgentStatusPort
	podIp := pod.Status.PodIP

	for _, container := range pod.Spec.Containers {
		if container.Name == c.GetPilotAgentContainer(pod) {
			for index, arg := range container.Args {
				if arg == "--statusPort" && index+1 < len(container.Args) {
					port = container.Args[index+1]
				}
			}
		}
	}

	return fmt.Sprintf("http://%v:%v/", podIp, port)
}

func joinPath(paths ...string) (joinStr string) {
	if len(paths) == 0 {
		return joinStr
	}
	for _, path := range paths {
		if re := regexp.MustCompile("^(.*)/$"); re.MatchString(joinStr) {
			joinStr = re.ReplaceAllString(joinStr, `$1`)
		}
		if re := regexp.MustCompile("^/(.*)$"); re.MatchString(path) {
			path = re.ReplaceAllString(path, `$1`)
		}
		joinStr = fmt.Sprintf("%v/%v", joinStr, path)
	}
	if re := regexp.MustCompile("^/(.*)$"); re.MatchString(joinStr) {
		joinStr = re.ReplaceAllString(joinStr, `$1`)
	}
	return joinStr
}

func hasProxy(pod *v1.Pod, proxyName string) bool {
	for _, container := range pod.Spec.Containers {
		if container.Name == proxyName {
			log.Infof("pod :[%v] has proxy:[%v]", pod.Name, proxyName)
			return true
		}
	}
	log.Infof("pod :[%v] has not proxy:[%v]", pod.Name, proxyName)
	return false
}
