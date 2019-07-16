package jsf

import (
	"encoding/json"
	"io/ioutil"
	"istio.io/istio/pkg/log"
	"net/http"
	"strings"
	"time"
)

const (
	jsfInterfaceOpenapiUrl  = "http://g.jsf.jd.local/com.jd.jsf.openapi.service.JSFOpenAPI/jsf-open-api/getServerStatusListByInterface/1012889/jsf"
	contentType = "application/x-www-form-urlencoded"
)

type Client struct {
	serviceNameList []string
	refreshPeriod int
	services map[string]*Service
	out      chan ServiceEvent
	ticker *time.Ticker
}

// NewClient create a new jsf registry client
func NewClient(serviceNameList []string, refreshPeriod int) *Client {
	if refreshPeriod <= 60 {
		refreshPeriod = 5 * 60
	}
	client := &Client{
		serviceNameList: serviceNameList,
		refreshPeriod: refreshPeriod,
		services: make(map[string]*Service),
		out:      make(chan ServiceEvent),
		ticker:	time.NewTicker( time.Second * time.Duration(refreshPeriod)),
	}
	return client
}

//刷新接口信息,服务以接口名称为准
func (c *Client) refreshServices() {
	if len(c.serviceNameList) > 0 {
		for _, serviceName := range c.serviceNameList {
			if len(serviceName) > 0 {
				go func() {
					serviceJsonObj := getJsfInterfaceInfoByHttp(serviceName)
					if serviceJsonObj != nil {
						if(!serviceJsonObj.Success && serviceJsonObj.Code == 404){
							 c.deleteService(serviceName)
						} else {
							serviceMap := convertToService(serviceJsonObj)
							if len(serviceMap) <= 0 {
								c.deleteService(serviceName)
							} else {
								for hostname, service := range serviceMap { //hostname:interfacename
									c.services[hostname] = service
									//触发服务及实例变更事件
									c.triggerAlterationEvent(service)
								}
							}
						}
					}
				}()
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

func (c *Client) triggerAlterationEvent( s *Service ){
	if(s == nil){
		return ;
	}
	var curInstances []*Instance
	_, exist := c.services[s.name]
	if(!exist){
		go c.notify(ServiceEvent{
			EventType: ServiceAdd,
			Service: s,
		})
		curInstances = make([]*Instance,0)
	} else {
		curInstances = c.services[s.name].instances
	}
	//删除相应的服务实例
	deleteInsSet := subtract(curInstances, s.instances)
	for _, delIns := range deleteInsSet {
		go c.notify(ServiceEvent{
			EventType:ServiceInstanceDelete,
			Instance:delIns,
		})
	}
	//添加相应的服务实例
	addInsSet := subtract(s.instances, curInstances)
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

//通过http接口定时轮询jsf接口信息
func getJsfInterfaceInfoByHttp( interfaceName string ) *ServiceJsonObj {
	if(len(interfaceName) <= 0) {
		return nil
	}
	requestContent := string("{\"appId\":\"1012889\",\"erp\":\"chenjiao7\",\"token\":\"1012889\",\"interfaceName\":\""+interfaceName+"\"}");
	resp, err := http.Post(jsfInterfaceOpenapiUrl, contentType, strings.NewReader(requestContent))
	if err != nil {
		log.Errora("http request is error.",err)
		return nil
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errora("ReadAll resp body is error.",err)
		return nil
	}

	var serviceJsonObj ServiceJsonObj
	err = json.Unmarshal(body, &serviceJsonObj)
	if err != nil {
		log.Errora("json Unmarshal is error.",err)
		return nil
	}
	return &serviceJsonObj
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
func (c *Client) InstancesByHost(host string) []*Instance {
	instances := make([]*Instance, 0)

	for _, service := range c.services {
		for _, instance := range service.instances {
			if instance.Host == host {
				instances = append(instances, instance)
			}
		}
	}

	return instances
}

func (c *Client) Start() error {
	if c.ticker != nil {
		<- c.ticker.C
		go c.refreshServices()
	}
	return nil
}

// Stop registry client and close all channels
func (c *Client) Stop() {
	close(c.out)
}

func (c *Client) notify(event ServiceEvent) {
	c.out <- event
	c.ticker.Stop()
}
