package version

import (
	"fmt"
	"istio.io/pkg/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"strings"
)

type PodsSelector interface {
	Selector(policy SelectPolicy) (*[]v1.Pod, error)
}

type SelectPolicy interface {
	Selector(kubeClient *kubernetes.Clientset) (*[]v1.Pod, error)
}

type podsSelectorImpl struct {
	kubeClient *kubernetes.Clientset
}

func NewPodsSelector(kubeClient *kubernetes.Clientset) PodsSelector {
	return &podsSelectorImpl{kubeClient: kubeClient}
}

func (v *podsSelectorImpl) Selector(policy SelectPolicy) (*[]v1.Pod, error) {
	return policy.Selector(v.kubeClient)
}

type ViaDeployment struct {
	name      string
	namespace string
}

type ViaService struct {
	name      string
	namespace string
}

type ViaStatefulSet struct {
	name      string
	namespace string
}

type ViaLabelSelector struct {
	namespace string
	labels    map[string]string
}

func (v ViaDeployment) Selector(kubeClient *kubernetes.Clientset) (*[]v1.Pod, error) {
	deploy, err := kubeClient.AppsV1().Deployments(v.namespace).Get(v.name, metav1.GetOptions{})
	if err != nil {
		log.Warna(err)
		return nil, err
	}
	//todo: select by matchExpression
	labelSelector := deploy.Spec.Selector.MatchLabels
	return ViaLabelSelector{
		namespace: v.namespace,
		labels:    labelSelector,
	}.Selector(kubeClient)
}

func (v ViaService) Selector(kubeClient *kubernetes.Clientset) (*[]v1.Pod, error) {
	service, err := kubeClient.CoreV1().Services(v.name).Get(v.name, metav1.GetOptions{})
	if err != nil {
		log.Warna(err)
		return nil, err
	}
	labelSelector := service.Spec.Selector
	return ViaLabelSelector{
		namespace: v.namespace,
		labels:    labelSelector,
	}.Selector(kubeClient)
}

func (v ViaStatefulSet) Selector(kubeClient *kubernetes.Clientset) (*[]v1.Pod, error) {
	statefulset, err := kubeClient.AppsV1().StatefulSets(v.namespace).Get(v.name, metav1.GetOptions{})
	if err != nil {
		log.Warna(err)
		return nil, err
	}
	//todo: select by matchExpression
	labelSelector := statefulset.Spec.Selector.MatchLabels
	return ViaLabelSelector{
		namespace: v.namespace,
		labels:    labelSelector,
	}.Selector(kubeClient)
}

func (v ViaLabelSelector) Selector(kubeClient *kubernetes.Clientset) (*[]v1.Pod, error) {
	labels := make([]string, 0)
	for k, v := range v.labels {
		labels = append(labels, fmt.Sprintf("%v=%v", k, v))
	}
	labelStr := strings.Join(labels, ",")
	podList, err := kubeClient.CoreV1().Pods(v.namespace).List(metav1.ListOptions{
		LabelSelector: labelStr,
	})
	if err != nil {
		log.Warna(err)
		return nil, err
	}
	return &podList.Items, nil
}
