## 严选Service Mesh部署方案（基于ISTIO release 1.1.5）

### 部署说明

根据严选的部署要求以及我们给出的部署方案，采用的是最小部署，即只部署`istio pilot`组件；

`pilot`采用的是修改后的版本，修改内容如下：

在原组件的基础上添加了`"--portMapping=http|8550:80"`参数支持，作用是将严选默认的访问方式类似于127.0.0.1:8550/[svc]/path映射到80端口的rds上

另外与之配合的是envoy做了修改，支持从上述的路径中解析出具体的目标服务信息，从而用以istio里寻找对应的xds配置信息；

### 部署方案：

1、从官方下载 tag为1.1.5的release版本 <https://github.com/istio/istio/releases/tag/1.1.5>



```
wget https://github.com/istio/istio/releases/download/1.1.5/istio-1.1.5-linux.tar.gz
tar -zxvf istio-1.1.5-linux.tar.gz 
```

2、上传至需要安装的目标机器上，并解压

3、可选步骤，如果之前没有安装过helm，请先安装helm

```bash
wget https://storage.googleapis.com/kubernetes-helm/helm-v2.9.1-linux-amd64.tar.gz
tar zxvf helm-v2.9.1-linux-amd64.tar.gz
cp linux-amd64/helm /usr/bin
```

4、可选步骤，如果之前没有安装过istio，请先创建istio-system namespace

```bash
$ kubectl create namespace istio-system
```

5、安装ISTIO需要的[Custom Resource Definitions](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions) (CRDs) 

```bash
$ helm template install/kubernetes/helm/istio-init --name istio-init --namespace istio-system | kubectl apply -f -
```

可以通过以下命令检查是否安装成功：

```bash
$ kubectl get crds | grep 'istio.io\|certmanager.k8s.io' | wc -l
53
```

如果返回结果是53说明CRDs创建成功，可能略有延迟，约30秒左右



6、根据官方的最小安装指导

```bash
$ helm template install/kubernetes/helm/istio --name istio --namespace istio-system \
    --values install/kubernetes/helm/istio/values-istio-minimal.yaml > yanxuan_isito_install.yaml
```

经验证，最小化部署中自动注入功能有问题，需要进一步解决



6.1、带网关的安装命令

```bash
$ helm template install/kubernetes/helm/istio --name istio --namespace istio-system \
    --values install/kubernetes/helm/istio/values-istio-minimal.yaml > yanxuan_isito_install_with_gateways.yaml
```

```bash
$ helm template install/kubernetes/helm/istio --name istio --namespace istio-system --set kiali.enabled=true --values install/kubernetes/helm/istio/values-istio-yanxuan.yaml > yanxuan_isito_install.yaml
```

hub.c.163.com/qingzhou/pilot:e5438fe0f4d171302a5216d773abfc63477d949a



7、修改生成的`yanxuan_istio_install.yaml`文件

1. `configmap`中修改`includeOutboundIPRanges`默认的*为空字符串

2. ~~修改`istio proxy init `的容器镜像为以下地址：`hub.c.163.com/qingzhou/istio/proxy_init:1.1.5-yx~~`

3. 修改`isito proxy `的容器镜像为以下地址：`hub.c.163.com/qingzhou/istio/proxyv2:1.1.5-yx`

4. 修改`pilot`镜像为修改过的镜像地址：`hub.c.163.com/qingzhou/istio/pilot:v1.1.5-yx-1`

   并且根据用户实际使用的服务名前后缀添加启动参数，使用方式如下：

   ```
   stable-image: hub.c.163.com/qingzhou/pilot:1.1.5-yx-1
   1.增加了backup负载均衡策略，backup地址通常指向云外网关地址，当云内服务故障时会尝
     试从云外网关访问云外服务。启动backup机制需为pilot-discovery添加如下启动项：
     --backupAddress=10.10.10.10:8888
     其中10.10.10.10:8888替换为云外网关的地址( ip:port )
   2.增加了domain扩展机制，可以为集群内的服务设置特定的前后缀。例如：在配置了前后缀
     后，服务a可以使用扩展服务名{{prefix.servicename-b.suffix}}调用服务b，
     servicename-b为b的servicename简写。启用扩展服务名，需为pilot-discovery添加如下
     启动项：
     --nsfHostPrefix={{prefix}}
     --nsfHostSuffix={{suffix}}
   3.修复了判断k8sSvc的bug
   ```

5. 切换`ingress`的工作模式从`router`为`sidecar`，修改方式为`yaml`文件里修改`ingress`的的启动参数，去除`--router`

8、应用修改后的`yaml`文件

```bash
$ kubectl apply -f yanxuan_isito_install.yaml
```

