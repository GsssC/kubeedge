# Autonomic Kube-API Endpoint
## Overview
边缘的Kubernetes客户端(以下称Kube Client, 典型有Kubelet、Kubeproxy、CNI-Plugin等)直连云中心的API Server, 会因为网络波动产生诸如重启内存数据丢失, Informer异常relist等问题，影响边缘业务连续性，增大网络带宽消耗和限制节点规模。

Autonomic Kube-API Endpoint(以下简称AKE)特性是为了满足边缘Kube Client的[Kube API](https://kubernetes.io/zh/docs/concepts/overview/kubernetes-api/)访问请求，而在边缘设立的一个类API Server的Kube API访问端点，可以视为一个小型化的API Server，提供可靠、稳定的Kube API请求处理服务。其根据Kube API请求自主地向云端申请下发相关Kube API对象，持久化至边缘数据库后再响应给Kube Client。除了能在网络正常时提供所有Read类(Get、List、Watch)和Write类(Create、Delete、Update、Patch)Kube API请求，其还能在断网期间根据边缘持久化数据库，持续服务边缘客户端的Read类Kube API请求，在重连后自动由云向边同步Kube API数据。在未来，也考虑开放断网时的Write类请求处理能力，断网重连后由边向云同步断网期间边缘数据库中Kube API对象的更改，增强KubeEdge的离线自治能力。

在正式引入AKE的实现细节前，我们需要了解探讨一些背景问题，这决定了AKE特性的架构设计。
## Backgroud
### Problems if Kube Client directly connecte to API Server
Kubernetes是一个面向数据中心的跨宿主机容器编排解决方案，在最初设计时并没有考虑边缘不稳定、低吞吐的低质量网络环境, 以及边缘节点的特殊需求。问题如下:
- 节点重启导致内存Kube API数据丢失
- Informer机制的重同步问题
- 缺少物联网设备接入能力
- Kubelet需要轻量化

#### 节点重启是边缘场景中较为常见的现象
目前典型的Kube Client，例如Kubelet等，是未将Kube API数据持久化的，节点重启伴随着进程重启，触发边缘Kube Client向云端重发Kube API请求。在云边网络良好情况下，这只是增加了网络的传输带宽以及云上API Server的负载压力；但是在云边网络断开时，边缘Kube Client由于无法联通API Server而停止工作。

#### Informer机制的重同步(Re-ListWatch)问题。
Informer是旨在减轻大规模集群下API Server的Read类请求(Get、List、Watch)访问压力的组件。其运行于Kube Client进程内部，义如其名，本质上是一个带有数据变更通知能力的缓存，缓存的数据和API Server后端的Etcd具有最终一致性，以group version resource namspaces labelSelector fieldSelector限定同步范围。Kube Client内部所有对API Server的Read类请求访问请求都可以通过对应的Informer满足，以减少对API Server的直接访问。特别地，Informer和Etcd之间数据的最终一致性通过List请求和Watch请求共同保障。其中List负责初始化缓存；Watch负责监听数据变更并依次同步修改缓存，是一条长连接。网络波动造成Watch长连接断连，同步失谐，Informer会尝试重同步，即重发List和Watch请求。在云边不稳定的网络环境下，这种异常发生频繁，直接提高了API Server的访问压力，增加了云边控制面的带宽消耗。

#### 物联网设备接入能力和Kubelet轻量化
这两个功能的实现并不是本文关注重点，但是值得注意的是这两个能力都需要访问获取Kube API对象方能正常工作。在KubeEdge中，轻量化kubelet模块edged需要访问获取Pod、Configmap、Secret、Service、Endpoint共6种Kube API；Device侧功能需要访问获取Device Model和Devic共2种自定义资源(Kubernetes CRD)。

总结上述4点问题，无论是Kubernetes原生Kube Client(Kubelet、KubeProxy等)又或者是根据边缘需求定制的软件(Edged、DeviceTwin)，都需要一个可靠稳定的Kube API访问端点，应对网络波动、网络断开等各类异常场景。KubeEdge在设计之初便考虑了上述前2个问题，采用Sqlite持久化Kube API对象，采用[可靠消息推送模型](./reliable-message-delivery.md)避免云边间出现Watch长连接，同时仍能持续接收Kube API对象变更事件。但是目前只能满足EdgeCore进程内部的Kube API访问需求，并且只提供了上文所述共8中API的访问获取，在调用上也并未规范。
## Goals
- 支持外部进程访问。Support out-process accessing by implementing an Lite Kube API HTTP Server on edgenode
- 规范内部进程访问。Normalize in-process accessing by implementing an [clientset.Interface](https://github.com/kubernetes/kubernetes/blob/c90330d8f4ca9fd980df24044960a4d8bb28a780/staging/src/k8s.io/client-go/kubernetes/clientset.go#L70)
- 支持Get List Watch Create Delete Update Patch等动词。
- 支持内置和自定义API。Support both build-in and CRD API
- 支持请求带选项。
## Architecture

## Detail of call

## Appendix
### 
## Reference
