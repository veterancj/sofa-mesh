package jdnp

import (
	"fmt"
	"github.com/pkg/errors"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pilot/pkg/serviceregistry/kube"
	"k8s.io/client-go/kubernetes"
	"strings"
	"istio.io/istio/pkg/log"
)

type serviceHandler func(*model.Service, model.Event)
type instanceHandler func(*model.ServiceInstance, model.Event)

// Controller communicate with jsfRegistry.
type Controller struct {
	client           *Client
	serviceHandlers  []serviceHandler
	instanceHandlers []instanceHandler
}

// NewController create a Controller instance
func NewController(domainNameStr string, refreshPeriod int, kubeClient kubernetes.Interface, options kube.ControllerOptions) (*Controller, error) {
	domainNames := strings.Split(domainNameStr, ",")
	client := NewClient(domainNames, refreshPeriod, kubeClient, options)
	controller := &Controller{
		client: client,
	}
	return controller, nil
}

// AppendServiceHandler notifies about changes to the service catalog.
func (c *Controller) AppendServiceHandler(f func(*model.Service, model.Event)) error {
	c.serviceHandlers = append(c.serviceHandlers, f)
	return nil
}

// AppendInstanceHandler notifies about changes to the service instances
// for a service.
func (c *Controller) AppendInstanceHandler(f func(*model.ServiceInstance, model.Event)) error {
	c.instanceHandlers = append(c.instanceHandlers, f)
	return nil
}

// Run until a signal is received
func (c *Controller) Run(stop <-chan struct{}) {
	if err := c.client.Start(stop); err != nil {
		log.Warnf("Can not connect to jsf np registry %s", c.client)
		return
	}

	for {
		select {
		case event := <-c.client.Events():
			switch event.EventType {
			case ServiceAdd:
				log.Infof("Service %s added", event.Service)
				service := toService(event.Service)
				for _, handler := range c.serviceHandlers {
					go handler(service, model.EventAdd)
				}
			case ServiceDelete:
				log.Infof("Service %s deleted", event.Service)
				service := toService(event.Service)
				for _, handler := range c.serviceHandlers {
					go handler(service, model.EventDelete)
				}
			case ServiceInstanceAdd:
				log.Infof("Service instance %v added", event.Instance)
				instance, err := toInstance(event.Instance)
				if err != nil {
					break
				}
				for _, handler := range c.instanceHandlers {
					go handler(instance, model.EventAdd)
				}
			case ServiceInstanceDelete:
				log.Infof("Service instance %v deleted", event.Instance)
				instance, err := toInstance(event.Instance)
				if err != nil {
					break
				}
				for _, handler := range c.instanceHandlers {
					go handler(instance, model.EventDelete)
				}
			}
		case <-stop:
			c.client.Stop()
			// also close zk
		}
	}
}

// Services list all service within zookeeper registry
func (c *Controller) Services() ([]*model.Service, error) {
	services := c.client.Services()
	result := make([]*model.Service, 0, len(services))
	for _, service := range services {
		result = append(result, toService(service))
	}
	return result, nil
}

// GetService retrieve dedicated service with specific host name
func (c *Controller) GetService(hostname model.Hostname) (*model.Service, error) {
	s := c.client.Service(string(hostname))
	if s == nil {
		return nil, errors.Errorf("service %s not exist", hostname)
	}
	return toService(s), nil
}

// GetServiceAttributes implements a service catalog operation.
func (sd *Controller) GetServiceAttributes(hostname model.Hostname) (*model.ServiceAttributes, error) {
	svc, err := sd.GetService(hostname)
	if svc != nil {
		exportTo := make(map[model.Visibility]bool)
		exportTo[model.VisibilityPublic] = true
		return &model.ServiceAttributes{
			Name:      string(hostname),
			Namespace: model.IstioDefaultConfigNamespace,
			ExportTo: exportTo}, nil
	}
	return nil, err
}

// WorkloadHealthCheckInfo retrieves set of health check info by instance IP.
// This will be implemented later
func (c *Controller) WorkloadHealthCheckInfo(addr string) model.ProbeList {
	return nil
}

// Instances list all instance for a specific host name
func (c *Controller) Instances(hostname model.Hostname, ports []string,
	labels model.LabelsCollection) ([]*model.ServiceInstance, error) {
	instances := c.client.Instances(string(hostname))
	result := make([]*model.ServiceInstance, 0)
	for _, instance := range instances {
		i, err := toInstance(instance)
		if err != nil {
			continue
		}
		for _, name := range ports {
			if name == instance.Port.Protocol && labels.HasSubsetOf(i.Labels) {
				result = append(result, i)
			}
		}
	}
	return result, nil
}

// Instances list all instance for a specific host name
func (c *Controller) InstancesByPort(hostname model.Hostname, port int,
	labels model.LabelsCollection) ([]*model.ServiceInstance, error) {
	instances := c.client.Instances(string(hostname))
	result := make([]*model.ServiceInstance, 0)
	for _, instance := range instances {
		i, err := toInstance(instance)
		if err != nil {
			continue
		}
		if labels.HasSubsetOf(i.Labels) && portMatch(i, port) {
			result = append(result, i)
		}
	}
	return result, nil
}

func (c *Controller) GetProxyServiceInstances(proxy *model.Proxy) ([]*model.ServiceInstance, error) {
	instances := c.client.InstancesByHost(proxy.IPAddresses)
	result := make([]*model.ServiceInstance, 0, len(instances))
	for _, instance := range instances {
		i, err := toInstance(instance)
		if err == nil {
			result = append(result, i)
		}
	}
	return result, nil
}

func (c *Controller) GetProxyWorkloadLabels(proxy *model.Proxy) (model.LabelsCollection, error) {
	return nil,fmt.Errorf("not supported!")
}

func (c *Controller) ManagementPorts(addr string) model.PortList {
	return nil
}

// GetIstioServiceAccounts implements model.ServiceAccounts operation TODO
func (c *Controller) GetIstioServiceAccounts(hostname model.Hostname, ports []int) []string {
	return []string{
		"spiffe://cluster.local/ns/default/sa/default",
	}
}

// returns true if an instance's port matches with any in the provided list
func portMatch(instance *model.ServiceInstance, port int) bool {
	return port == 0 || port == instance.Endpoint.ServicePort.Port
}

func toService(s *Service) *model.Service {
	ports := make([]*model.Port, 0, len(s.Ports()))
	for _, p := range s.Ports() {
		port := toPort(p)
		ports = append(ports, port)
	}
	service := &model.Service{
		Hostname:   model.Hostname(s.Name()),
		Resolution: model.ClientSideLB,
		Ports:      ports,
		Attributes: model.ServiceAttributes{
			Name: s.Name(),
			Namespace: model.IstioDefaultConfigNamespace,
			ExportTo: map[model.Visibility]bool{model.VisibilityPublic:true},
		},
	}
	return service
}

// The endpoint in sofa rpc registry looks like bolt://192.168.1.100:22000?xxx=yyy
func toInstance(instance *Instance) (*model.ServiceInstance, error) {
	networkEndpoint := model.NetworkEndpoint{
		Family:      model.AddressFamilyTCP,
		Address:     instance.Host,
		Port:        instance.Port.Portoi(),
		ServicePort: toPort(instance.Port),
	}

	return &model.ServiceInstance{
		Endpoint: networkEndpoint,
		Service:  toService(instance.Service),
		Labels:   instance.Labels,
	}, nil
}

func toPort(port *Port) *model.Port {
	return &model.Port{
		Name:     port.Protocol,
		Protocol: model.ParseProtocol(port.Protocol),
		Port:     port.Portoi(),
	}
}