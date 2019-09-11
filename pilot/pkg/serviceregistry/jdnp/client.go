package jdnp

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"istio.io/istio/pilot/pkg/serviceregistry/kube"
	"istio.io/istio/pkg/log"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	informersv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	corev1 "k8s.io/api/core/v1"
)

const (
	dnsToVipLbInfosUrl  = "http://api-np.jd.local/V1/Dns/records?domain=%s" //pfinder-jmtp.jd.local
	appCode = "jsf-web-console"
	erp = "chenjiao7"
	secretKey = "57f8bd5cb103ec39228a6630b3d0e617"
	bigmeshJdnpServiceWhiteList = "bigmesh.jdnp.domain.white.list"
	jdnp_config = "bigmesh/app=bigmesh.jdnp.config"
	domainWhiteList = "whiteList"
	bigmesh_ns = "bigmesh-system"
)

type Client struct {
	urlStrList    []string
	domainMapUrl  map[string][]*url.URL
	refreshPeriod int
	services      map[string]*Service
	out           chan ServiceEvent
	timer        *time.Timer
	//用于获取jsf服务发现的白名单
	informer cache.SharedIndexInformer
	options kube.ControllerOptions
}

// NewClient create a new jsf registry client
func NewClient(urlStringList []string, refreshPeriod int, kubeClient kubernetes.Interface, options kube.ControllerOptions) *Client {
	if refreshPeriod < 1 {
		refreshPeriod = 30
	}
	informer := newJsfServiceConfigSharedIndexInformer(kubeClient, options)
	client := &Client{
		urlStrList:    urlStringList,
		domainMapUrl:  convertToDomainURLSlice(urlStringList),
		refreshPeriod: refreshPeriod,
		services:      make(map[string]*Service),
		out:           make(chan ServiceEvent),
		timer:         time.NewTimer( time.Second * time.Duration(refreshPeriod)),
		informer:      informer,
		options:       options,
	}
	return client
}

func newJsfServiceConfigSharedIndexInformer(client kubernetes.Interface, options kube.ControllerOptions) (informer cache.SharedIndexInformer) {
	informer = informersv1.NewFilteredConfigMapInformer(client,
		options.WatchedNamespace,
		options.ResyncPeriod,
		cache.Indexers{},
		func( opt *v1.ListOptions){
			opt.LabelSelector = jdnp_config;
		});
	return
}

func (c *Client) updateServiceNameList()  {
	if c.informer != nil && c.informer.HasSynced() {
		obj, exists, err := c.informer.GetStore().GetByKey(kube.KeyFunc(bigmeshJdnpServiceWhiteList, bigmesh_ns))
		if err != nil {
			log.Warnf("获取jsf service white list is error:%s",err)
			return
		}
		if exists {
			obj, ok := obj.(*corev1.ConfigMap);
			if ok {
				whiteListStr := obj.Data[domainWhiteList]
				if len(whiteListStr) > 0 {
					whiteListArray := strings.Split(whiteListStr, ",")
					if len(whiteListArray) > 0 {
						c.urlStrList = make([]string,0,len(whiteListArray));
						for _, domainUrl := range whiteListArray {
							if len(domainUrl) > 0 {
								c.urlStrList = append(c.urlStrList, domainUrl)
							}
						}
						c.domainMapUrl = convertToDomainURLSlice(c.urlStrList)
					}
				}
			}
		}
	}
}

func (c *Client) deleteNotExistService( lastDomainMapUrl map[string][]*url.URL ){
	if len(lastDomainMapUrl) > 0 {
		for lastKey, _ := range lastDomainMapUrl {
			isExist := false
			for curkey, _ := range c.domainMapUrl {
				if lastKey == curkey {
					isExist = true
				}
			}
			if !isExist {
				c.deleteService(lastKey)
			}
		}
	}
}

//刷新接口信息,服务以接口名称为准
func (c *Client) refreshServices() {
	lastDomainMapUrl := c.domainMapUrl
	//首先更新服务列表白名单
	c.updateServiceNameList()
	//根据服务列表白名单更新服务实例
	if len(c.urlStrList) > 0 {
		//先删除之前有，当前没有的service
		c.deleteNotExistService(lastDomainMapUrl)
		for domainName, curUrls := range c.domainMapUrl {
			if len(domainName) > 0 {
				jdNpDNSJsonObj := getDnsToVipInfoByHttp(domainName)
				if jdNpDNSJsonObj != nil && jdNpDNSJsonObj.ResStatus == 200 {
					if(len(jdNpDNSJsonObj.Data)<=0 ){
						c.deleteService(domainName)
					} else {
						serviceMap := convertToService(jdNpDNSJsonObj, curUrls)
						if len(serviceMap) <= 0 {
							c.deleteService(domainName)
						} else {
							for hostname, service := range serviceMap { //hostname:interfacename
								oldService := c.services[hostname]
								//先赋值，再触发事件
								c.services[hostname] = service
								c.triggerAlterationEvent(oldService, service)
							}
						}
					}
				}
			}
		}
	}
}

//根据服务接口名称删除相应的服务
func (c *Client) deleteService(hostname string) {
	for h, s := range c.services {
		if strings.HasSuffix(h, hostname) {
			for _,delIns := range s.instances {
				go c.notify(ServiceEvent{
					EventType:ServiceInstanceDelete,
					Instance:delIns,
				})
			}
			delete(c.services, h)
			go c.notify(ServiceEvent{
				EventType: ServiceDelete,
				Service:   s,
			})
		}
	}
}

func (c *Client) triggerAlterationEvent( old, new *Service ){
	if(new == nil){
		return ;
	}
	var curInstances []*Instance
	if(old == nil){
		go c.notify(ServiceEvent{
			EventType: ServiceAdd,
			Service: new,
		})
		curInstances = make([]*Instance,0)
	} else {
		curInstances = old.instances
	}
	//删除相应的服务实例
	deleteInsSet := subtract(curInstances, new.instances)
	for _, delIns := range deleteInsSet {
		go c.notify(ServiceEvent{
			EventType:ServiceInstanceDelete,
			Instance:delIns,
		})
	}
	//添加相应的服务实例
	addInsSet := subtract(new.instances, curInstances)
	for _, addIns := range addInsSet {
		go c.notify(ServiceEvent{
			EventType:ServiceInstanceAdd,
			Instance:addIns,
		})
	}

}

func contain(insSlice []*Instance, ins *Instance) bool {
	if(ins != nil && len(insSlice) > 0) {
		for _, curIns := range insSlice {
			if(curIns.Host == ins.Host && curIns.Port.Port == ins.Port.Port && curIns.Port.Protocol == ins.Port.Protocol){
				return true
			}
		}
	}
	return false
}

func subtract( a []*Instance, b []*Instance ) []*Instance {
	r := make([]*Instance, 0, len(a))
	for _, aIns := range a {
		if !contain(b, aIns) {
			r = append(r, aIns)
		}
	}
	return r
}

//通过http接口定时轮询域名对应的vip服务实例
func getDnsToVipInfoByHttp( domain string ) *JdNpDNSJsonObj {
	if(len(domain) <= 0) {
		return nil
	}

	now := time.Now()
	curSec := now.Unix()
	timestamp := strconv.FormatInt(curSec,10)
	log.Infof("getDnsToVipInfoByHttp timestamp: %s", timestamp)
	timeStr := now.Format("150420060102")
	log.Infof("getDnsToVipInfoByHttp \ntimeStr: %s", timeStr)
	signStr := sign(erp, secretKey, timeStr)
	log.Infof("getDnsToVipInfoByHttp \nsignStr: %s", signStr)
	curDnsToVipLbInfosUrl := fmt.Sprintf(dnsToVipLbInfosUrl, domain)
	req, err := http.NewRequest("GET", curDnsToVipLbInfosUrl, nil)
	if err != nil {
		log.Errora("getDnsToVipInfoByHttp http GET method is error.",err)
		return nil
	}

	req.Header.Set("appCode", appCode)
	req.Header.Set( "erp", erp)
	req.Header.Set("timestamp", timestamp)
	req.Header.Set("sign", signStr)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errora("getDnsToVipInfoByHttp http GET method client do is error.",err)
		return nil
	}
	defer resp.Body.Close()

	respByte, err:= ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errora("getDnsToVipInfoByHttp respByte is error.",err)
		return nil
	}

	var jdNpDNSJsonObj JdNpDNSJsonObj
	if err = json.Unmarshal(respByte, &jdNpDNSJsonObj); err != nil {
		log.Errora("getDnsToVipInfoByHttp json Unmarshal is error.",err)
		return nil
	}

	return &jdNpDNSJsonObj
}

func sign(erp,token,timeStr string) string {
	var buffer bytes.Buffer
	buffer.WriteString(erp)
	buffer.WriteString("#")
	buffer.WriteString(token)
	buffer.WriteString("NP")
	buffer.WriteString(timeStr)
	md5 := md5.New()
	_, err := md5.Write(buffer.Bytes())
	if err != nil {
		log.Errorf("getDnsToVipInfoByHttp sign md5 write is error.%s", err)
		return "";
	}
	signStr := hex.EncodeToString(md5.Sum(nil))
	return signStr
}


// Events channel is a stream of Service and instance updates
func (c *Client) Events() <-chan ServiceEvent {
	return c.out
}

// Service retrieve the service by its name
func (c *Client) Service(hostname string) *Service {
	return c.services[hostname]
}

// Services list all of the current registered services
func (c *Client) Services() []*Service {
	services := make([]*Service, 0, len(c.services))
	for _, service := range c.services {
		services = append(services, service)
	}
	return services
}

// Instances query service instances with labels
func (c *Client) Instances(hostname string) []*Instance {
	instances := make([]*Instance, 0)
	service, ok := c.services[hostname]
	if !ok {
		return instances
	}

	for _, instance := range service.instances {
		instances = append(instances, instance)
	}

	return instances
}

// Instances query service instances with labels
func (c *Client) InstancesByHost(hosts []string) []*Instance {
	instances := make([]*Instance, 0)

	for _, service := range c.services {
		for _, instance := range service.instances {
			for _, host := range hosts {
				if instance.Host == host {
					instances = append(instances, instance)
				}
			}
		}
	}

	return instances
}

func (c *Client) Start(stop <-chan struct{}) error {
	if c.informer != nil {
		go func() {
			c.informer.Run(stop)
		}()
	}
	if c.timer != nil {
		go func() {
			for {
				select {
				case <- c.timer.C:
					c.refreshServices()
					c.timer.Reset(time.Second * time.Duration(c.refreshPeriod))
				case <- stop:
					c.Stop()
					return
				}
			}
		}()
	}
	return nil
}

// Stop registry client and close all channels
func (c *Client) Stop() {
	close(c.out)
	c.timer.Stop()
}

func (c *Client) notify(event ServiceEvent) {
	c.out <- event
}
