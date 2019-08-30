package jdnp

import (
	"strconv"
)

type ServiceEventType int

const (
	ServiceAdd ServiceEventType = iota
	ServiceDelete
	ServiceInstanceAdd
	ServiceInstanceDelete
)

type ServiceEvent struct {
	EventType ServiceEventType
	Service   *Service
	Instance  *Instance
}

type Port struct {
	Protocol string
	Port     string
}

type Service struct {
	name      string
	ports     []*Port
	instances []*Instance
}

type Instance struct {
	Service *Service
	Host    string
	Port    *Port
	Labels  map[string]string
}

func (p *Port) Portoi() int {
	port, err := strconv.Atoi(p.Port)
	if err != nil {
		return 0
	}
	return port
}

func (s *Service) AddPort(port *Port) {
	exist := false
	for _, p := range s.ports {
		if p.Port == port.Port && p.Protocol == port.Protocol {
			exist = true
			break
		}
	}
	if !exist {
		s.ports = append(s.ports, port)
	}
}

func (s *Service) Name() string {
	return s.name
}

func (s *Service) Ports() []*Port {
	return s.ports
}

func (s *Service) Instances() []*Instance {
	return s.instances
}

type JdNpDNSJsonObj struct {
	AppCode string `json:"appCode"`
	ResStatus int `json:"resStatus"`
	ResMsg string `json:"resMsg"`
	Data []string `json:"data"`
}
