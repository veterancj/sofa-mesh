/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package controller

import (
	"fmt"

	listers "istio.io/istio/pkg/rpccontroller/listers/rpccontroller.istio.io/v1"
	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"

	"istio.io/istio/pkg/api/rpccontroller.istio.io/v1"
	"istio.io/istio/pkg/log"
	"istio.io/istio/pkg/rpccontroller/controller/watchers"

	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"k8s.io/client-go/kubernetes"
)

const (
	rpcVersion   = "version"
	rpcInterface = "interface"
)

// RPCQueryResponse for query RPC interface
type RPCQueryResponse struct {
	Success bool                 `json:"success"`
	Data    ServiceInterfaceData `json:"data"`
}

// ServiceInterfaceData for interface data
type ServiceInterfaceData struct {
	Providers []ServiceInterface `json:"providers"`
	Protocol  string             `json:"protocol"`
}

// ServiceInterface struct
type ServiceInterface struct {
	Interface string `json:"interface"`
	Version   string `json:"version"`
	Group     string `json:"group"`
	Serialize string `json:"serialize"`
}

type rpcWatcher struct {
	storage      map[string]ServiceInterfaceData
	rcSyncChan   chan *v1.RpcService
	rsDeleteChan chan *v1.RpcService

	serviceUpdateChan chan *watchers.ServiceUpdate
	podUpdateChan     chan *watchers.PodUpdate

	rpcServiceLister listers.RpcServiceLister
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface

	serviceWatcher *watchers.ServiceWatcher
	podWatcher     *watchers.PodWatcher

	dnsInterface DNSInterface

	tryLaterKeys map[string]bool

	// map rpcservice name -> interface
	rpcInterfacesMap map[string]map[string]bool

	// map k8s service to rpcservice name
	serivice2RpcServiceMap map[string]string

	// map k8s pod to rpcservice name
	pod2RpcServiceMap map[string]string

	// map domain to ip
	domain2IP map[string]string
}

func newRPCWatcher(lister listers.RpcServiceLister, client kubernetes.Interface, config *Config, stopCh <-chan struct{}) *rpcWatcher {
	rw := &rpcWatcher{
		storage:                make(map[string]ServiceInterfaceData, 0),
		rcSyncChan:             make(chan *v1.RpcService, 10),
		rsDeleteChan:           make(chan *v1.RpcService, 10),
		serviceUpdateChan:      make(chan *watchers.ServiceUpdate, 10),
		podUpdateChan:          make(chan *watchers.PodUpdate, 10),
		rpcServiceLister:       lister,
		kubeclientset:          client,
		tryLaterKeys:           make(map[string]bool, 0),
		rpcInterfacesMap:       make(map[string]map[string]bool, 0),
		serivice2RpcServiceMap: make(map[string]string, 0),
		pod2RpcServiceMap:      make(map[string]string, 0),
		domain2IP:              make(map[string]string, 0),
	}

	serviceWatcher, err := watchers.StartServiceWatcher(client, 2*time.Second, stopCh)
	if err != nil {

	}
	rw.serviceWatcher = serviceWatcher
	serviceWatcher.RegisterHandler(rw)

	podWatcher, err := watchers.StartPodWatcher(client, 2*time.Second, stopCh)
	if err != nil {

	}
	rw.podWatcher = podWatcher
	podWatcher.RegisterHandler(rw)

	if config.CoreDnsAddress != "" {
		rw.dnsInterface = newCoreDNSREST(config.CoreDnsAddress)
	} else {
		etcdClient := newEtcdClient(config)
		rw.dnsInterface = newCoreDNSEtcd(etcdClient)
	}

	return rw
}

func (rw *rpcWatcher) Run(stopCh <-chan struct{}) {
	go rw.main(stopCh)
}

func (rw *rpcWatcher) Sync(rs *v1.RpcService) {
	rw.rcSyncChan <- rs
}

func (rw *rpcWatcher) Delete(rs *v1.RpcService) {
	rw.rsDeleteChan <- rs
}

func (rw *rpcWatcher) main(stopCh <-chan struct{}) {
	t := time.NewTicker(10 * time.Second)

	for {
		select {
		case rs := <-rw.rcSyncChan:
			rw.syncRPCServiceHandler(rs)
		case rs := <-rw.rsDeleteChan:
			rw.deleteRPCServiceHandler(rs)
		case <-t.C:
			rw.timerHandler()
		case seviceUpdate := <-rw.serviceUpdateChan:
			rw.serviceHandler(seviceUpdate)
		case podUpdate := <-rw.podUpdateChan:
			rw.podHandler(podUpdate)
		case <-stopCh:
			log.Infof("rpc watcher exit")
			return
		}
	}
}

func (rw *rpcWatcher) rpcServiceName(rs *v1.RpcService) string {
	return fmt.Sprintf("%s/%s", rs.Namespace, rs.Name)
}

func (rw *rpcWatcher) serviceName(s *api.Service) string {
	return fmt.Sprintf("%s/%s", s.Namespace, s.Name)
}

func (rw *rpcWatcher) podName(s *api.Pod) string {
	return fmt.Sprintf("%s/%s", s.Namespace, s.GenerateName)
}

func (rw *rpcWatcher) deleteRPCServiceHandler(rs *v1.RpcService) {
	key := rw.rpcServiceName(rs)

	domains, exist := rw.rpcInterfacesMap[key]
	if !exist {
		log.Errorf("rpcservice %s interface not exist", key)
		return
	}

	for domain, _ := range domains {
		log.Infof("delete rpcservice %s, domain: %s", key, domain)
		ip, exist := rw.domain2IP[domain]
		if !exist {
			log.Errorf("cannot find domain %s ip", domain)
			continue
		}
		err := rw.dnsInterface.Delete(domain, ip, rs.Spec.DomainSuffix)
		if err != nil {
			log.Errorf("delete domain %s err: %v", domain, err)
		} else {
			delete(rw.domain2IP, domain)
		}
	}
	rw.rpcInterfacesMap[key] = nil
}

func (rw *rpcWatcher) timerHandler() {
	if len(rw.tryLaterKeys) == 0 {
		return
	}
	keys := make([]string, len(rw.tryLaterKeys))
	for key := range rw.tryLaterKeys {
		keys = append(keys, key)
	}

	rw.tryLaterKeys = make(map[string]bool, 0)

	for _, key := range keys {
		rs := rw.getRPCServiceByName(key)
		if rs == nil {
			//rw.tryLaterKeys[key] = true
			continue
		}
		rw.queryRPCInterface(key, rs)
	}
}

func (rw *rpcWatcher) queryRPCInterface(key string, rs *v1.RpcService) {
	selector := rs.Spec.Selector
	if len(selector) == 0 {
		log.Errorf("rpcservice %s/%s has empty selector", rs.Namespace, rs.Name)
		return
	}

	// find service by selector
	services, err := rw.serviceWatcher.ListBySelector(selector)
	if len(services) == 0 || err != nil {
		log.Errorf("service selector %v not found, err:%v", selector, err)
		return
	}
	if len(services) != 1 {
		log.Errorf("service selector %v found more than 1 service", selector)
		return
	}
	service := services[0]

	rw.serivice2RpcServiceMap[rw.serviceName(service)] = key

	if service.Spec.ClusterIP == "None" {
		log.Errorf("service %s/%s has no clusterIP", service.Namespace, service.Name)
		rw.deleteRPCServiceHandler(rs)
		return
	}

	pods, err := rw.podWatcher.ListBySelector(service.Labels)
	if len(pods) == 0 || err != nil {
		rw.tryLaterKeys[key] = true
		log.Errorf("service %s/%s has no pods, err: %v, try later...", service.Namespace, service.Name, err)
		return
	}
	rw.pod2RpcServiceMap[rw.podName(pods[0])] = key

	// get pod interface
	for _, pod := range pods {
		if pod.Status.Phase != "Running" {
			continue
		}

		/*
		version, exist := pod.Labels[rpcVersion]
		if !exist {
			log.Errorf("service %s/%s pod %s/%s has no %s label:%v",
				service.Namespace, service.Name, pod.Namespace, pod.Name, rpcVersion, pod.Labels)
			continue
		}

		i, exist := pod.Labels[rpcInterface]
		if !exist {
			log.Errorf("service %s/%s pod has no %s label", service.Namespace, service.Name, rpcInterface)
			continue
		}

		log.Infof("pod %s, interface %s, version %s", pod.Name, i, version)
		*/

		url := fmt.Sprintf("http://%s:10006/rpc/interfaces", pod.Status.PodIP)
		resp, err := http.Get(url)
		if err != nil || resp.StatusCode != 200 {
			continue
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Errorf("url %s resp read body error: %v", url, err)
			continue
		}

		log.Infof("url: %s, resp: %s", url, string(body))

		var rpcResponse RPCQueryResponse
		err = json.Unmarshal(body, &rpcResponse)
		if err != nil {
			log.Errorf("json Unmarshal RpcResponse error: %v", err)
			continue
		}

		if rpcResponse.Success != true {
			log.Errorf("key %s resp not success: %v, try again later...", key, rpcResponse)
			rw.tryLaterKeys[key] = true
			return
		}

		for _, inter := range rpcResponse.Data.Providers {
			/*
			if !strings.ContainsAny(inter.Interface, i) {
				log.Infof("interface %s, i %s", inter.Interface, i)
				continue
			}
			if inter.Version != version {
				log.Infof("version %s, i %s", inter.Version, version)
				continue
			}
			*/
			interfaceStr := inter.Interface

			// add <interface, clusterIP> to coreDNS
			log.Infof("update dns <%s,%s>", interfaceStr, service.Spec.ClusterIP)
			err = rw.dnsInterface.Update(interfaceStr, service.Spec.ClusterIP, rs.Spec.DomainSuffix)
			rw.domain2IP[interfaceStr] = service.Spec.ClusterIP
			if err != nil {
				log.Errorf("update dns error: %v", err)
				continue
			}
			if rw.rpcInterfacesMap[key] == nil {
				rw.rpcInterfacesMap[key] = make(map[string]bool, 0)
			}
			rw.rpcInterfacesMap[key][interfaceStr] = true
		}

		// query only one pod success to quit loop
		break
	}
}

func (rw *rpcWatcher) getRPCServiceByName(key string) *v1.RpcService {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		log.Errorf("SplitMetaNamespaceKey rpcservice %s/%s error: %v", namespace, name, err)
		return nil
	}

	rs, err := rw.rpcServiceLister.RpcServices(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("rpcservice '%s' in work queue no longer exists", key))
		}
		log.Errorf("get rpcservice %s/%s error: %v", namespace, name, err)
		return nil
	}

	return rs
}

func (rw *rpcWatcher) syncRPCServiceHandler(rs *v1.RpcService) {
	key := rw.rpcServiceName(rs)
	log.Infof("rpc service:%s", key)
	rw.queryRPCInterface(key, rs)
}

func (rw *rpcWatcher) OnServiceUpdate(serviceUpdate *watchers.ServiceUpdate) {
	if serviceUpdate.Op != watchers.SYNCED {
		rw.serviceUpdateChan <- serviceUpdate
	}
}

func (rw *rpcWatcher) serviceHandler(serviceUpdate *watchers.ServiceUpdate) {
	service := serviceUpdate.Service
	op := serviceUpdate.Op

	log.Debugf("service: %s/%s, op: %s/%d", service.Namespace, service.Name, watchers.OperationString[op], op)
	key, exist := rw.serivice2RpcServiceMap[rw.serviceName(service)]
	if !exist {
		log.Debugf("service %s/%s not found rpc service", service.Namespace, service.Name)
		return
	}
	rs := rw.getRPCServiceByName(key)

	if rs == nil {
		log.Debugf("rpcservice %s not found", key)
		return
	}

	switch op {
	case watchers.ADD:
		rw.syncRPCServiceHandler(rs)
	case watchers.UPDATE:
		rw.syncRPCServiceHandler(rs)
	case watchers.REMOVE:
		rw.deleteRPCServiceHandler(rs)
	}
}

func (rw *rpcWatcher) podHandler(podUpdate *watchers.PodUpdate) {
	pod := podUpdate.Pod
	op := podUpdate.Op

	log.Debugf("pod: %s/%s, labels: %s, op: %s/%d", pod.Namespace, pod.Name, rw.podName(pod), watchers.OperationString[op], op)

	key, exist := rw.pod2RpcServiceMap[rw.podName(pod)]
	if !exist {
		log.Debugf("pod %s/%s not found rpc service", pod.Namespace, pod.Name)
		return
	}

	rs := rw.getRPCServiceByName(key)
	if rs == nil {
		log.Debugf("rpcservice %s not found", key)
		return
	}

	switch op {
	case watchers.ADD:
		rw.syncRPCServiceHandler(rs)
	case watchers.UPDATE:
		rw.syncRPCServiceHandler(rs)
	case watchers.REMOVE:
		rw.syncRPCServiceHandler(rs)
	}
}

func (rw *rpcWatcher) OnPodUpdate(podUpdate *watchers.PodUpdate) {
	if podUpdate.Op != watchers.SYNCED {
		rw.podUpdateChan <- podUpdate
	}
}
