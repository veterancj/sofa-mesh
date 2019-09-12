#./push_image.sh sofastack/proxyv2:0.5.0-rc.1 sofastack/proxyv2:0.5.0.1 sofastack/kubectl:0.5.0-rc.1 sofastack/galley:0.5.0-rc.1 sofastack/mixer:0.5.0-rc.1 sofastack/sidecar_injector:0.5.0-rc.1 sofastack/citadel:0.5.0-rc.1 sofastack/pilot:0.5.0-rc.1 sofastack/proxy_init:0.5.0-rc.1 sofastack/galley:0.3.0 sofastack/citadel:0.3.0 sofastack/mixer:0.3.0 sofastack/sidecar_injector:0.3.0 sofastack/proxyv2:0.3.0 sofastack/pilot:0.3.0 sofastack/proxy_init:0.3.0 sofastack/e2e-dubbo-consumer:0.2.1 sofastack/e2e-dubbo-provider:0.2.1 


#./push_image.sh registry.cn-hangzhou.aliyuncs.com/google_containers/kube-proxy:v1.14.2 registry.cn-hangzhou.aliyuncs.com/google_containers/kube-apiserver:v1.14.2 registry.cn-hangzhou.aliyuncs.com/google_containers/kube-controller-manager:v1.14.2 registry.cn-hangzhou.aliyuncs.com/google_containers/kube-scheduler:v1.14.2 registry.cn-hangzhou.aliyuncs.com/google_containers/kube-addon-manager:v9.0 registry.cn-hangzhou.aliyuncs.com/google_containers/coredns:1.3.1 registry.cn-hangzhou.aliyuncs.com/google_containers/kubernetes-dashboard-amd64:v1.10.1 registry.cn-hangzhou.aliyuncs.com/google_containers/etcd:3.3.10 registry.cn-hangzhou.aliyuncs.com/google_containers/k8s-dns-sidecar-amd64:1.14.13 registry.cn-hangzhou.aliyuncs.com/google_containers/k8s-dns-kube-dns-amd64:1.14.13 registry.cn-hangzhou.aliyuncs.com/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.13 registry.cn-hangzhou.aliyuncs.com/google_containers/pause:3.1 registry.cn-hangzhou.aliyuncs.com/google_containers/storage-provisioner:v1.8.1


#./push_image.sh veterancj/pilot-debug:0.0.1 veterancj/pfinder-test-client:0.0.1 veterancj/pilot:bigmesh-0.0.1-dlv-debug veterancj/pilot:bigmesh-0.0.5-debug veterancj/pilot:bigmesh-0.0.6-debug veterancj/pilot:bigmesh-0.0.3-debug veterancj/pilot:bigmesh-0.0.3-test veterancj/pilot:bigmesh-0.0.4-test veterancj/pilot:bigmesh-0.0.1-test veterancj/pilot:bigmesh-0.0.2-test bitnami/kubectl:latest busybox:1.30.1 kiali/kiali:v0.16 istio/examples-bookinfo-reviews-v3:1.10.1 istio/examples-bookinfo-reviews-v2:1.10.1 istio/examples-bookinfo-reviews-v1:1.10.1 istio/examples-bookinfo-ratings-v1:1.10.1 istio/examples-bookinfo-details-v1:1.10.1 istio/examples-bookinfo-productpage-v1:1.10.1 istio/tcp-echo-server:1.1 prom/prometheus:v2.3.1 prom/statsd-exporter:v0.6.0 primetoninc/jre:1.8 quay.io/coreos/hyperkube:v1.7.6_coreos.0 yauritux/busybox-curl:latest imanticsiot/java8:latest 

registry=ishc.harbor.svc.hcyf.n.jd.local;
username=jstenhance
 
echo_r(){
        [ $# -ne 1 ] && return 0;
        echo -e "\033[31m$1\033[0m"
}
 
echo_g(){
        [ $# -ne 1 ] && return 0;
        echo -e "\033[32m$1\033[0m"
}
 
echo_y(){
        [ $# -ne 1 ] && return 0;
        echo -e "\033[33m$1\033[0m"
}
 
echo_b(){
        [ $# -ne 1 ] && return 0;
        echo -e "\033[34m$1\033[0m"
}
 
usage(){
        sudo docker images
        echo "Usage: $0 registry1:tag1 [registry2:tag2...]"
 
}
 
[ $# -lt 1 ] && usage && exit
echo_b "The registry server is $registry"
for image in "$@"
do
	image_suffix=${image##*/}
        echo_b "Uploading $image..."
	echo_b "var image_suffix:$image_suffix"
	echo_b "var registry:$registry"
	echo_b "var username:$username"
        docker tag $image $registry/$username/$image_suffix  #标记镜像
        docker push $registry/$username/$image_suffix #上传镜像到仓库
        docker rmi $registry/$username/$image_suffix  #删除本地标记镜像
        echo_g "Done"
done

