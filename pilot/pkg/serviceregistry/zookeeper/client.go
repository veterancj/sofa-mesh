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

package zookeeper

import (
	"github.com/samuel/go-zookeeper/zk"
	"net/url"
	"path"
	"strings"
)

type Client struct {
	conn     *zk.Conn
	root     string
	services map[string]*Service
	out      chan ServiceEvent

	scache  *pathCache
	pcaches map[string]*pathCache
}

// NewClient create a new zk registry client
func NewClient(root string, conn *zk.Conn) *Client {
	client := &Client{
		root:     root,
		conn:     conn,
		services: make(map[string]*Service),
		out:      make(chan ServiceEvent),
		pcaches:  make(map[string]*pathCache),
	}
	return client
}

// Events channel is a stream of Service and instance updates
func (c *Client) Events() <-chan ServiceEvent {
	return c.out
}

// Service retrieve the service by its hostname
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

// Stop registry client and close all channels
func (c *Client) Stop() {
	c.scache.stop()
	for _, pcache := range c.pcaches {
		pcache.stop()
	}
	close(c.out)
}

// Start client and watch the registry
func (c *Client) Start() error {
	cache, err := newPathCache(c.conn, c.root)
	if err != nil {
		return err
	}
	c.scache = cache
	go c.eventLoop()
	return nil
}

func (c *Client) eventLoop() {
	for vent := range c.scache.events() {
		switch vent.eventType {
		case pathCacheEventAdded:
			hostname := path.Base(vent.path)
			ppath := path.Join(vent.path, "providers")
			cache, err := newPathCache(c.conn, ppath)
			if err != nil {
				continue
			}
			c.pcaches[hostname] = cache
			go func() {
				for vent := range cache.events() {
					switch vent.eventType {
					case pathCacheEventAdded:
						c.addInstance(hostname, path.Base(vent.path))
					case pathCacheEventDeleted:
						c.deleteInstance(hostname, path.Base(vent.path))
					}
				}
			}()
		case pathCacheEventDeleted:
			// TODO in reality, this should not happened.
			hostname := path.Base(vent.path)
			c.deleteService(hostname)
		}
	}
}

func (c *Client) makeInstance(hostname string, rawUrl string) (*Instance, error) {
	cleanUrl, err := url.QueryUnescape(rawUrl)
	if err != nil {
		return nil, err
	}
	ep, err := url.Parse(cleanUrl)
	if err != nil {
		// log waring
		return nil, err
	}

	instance := &Instance{
		Port:   &Port{Protocol: ep.Scheme, Port: ep.Port()},
		Host:   ep.Hostname(),
		Labels: make(map[string]string),
	}

	for key, values := range ep.Query() {
		if values != nil {
			instance.Labels[key] = values[0]
		}
	}
	return instance, nil
}

func (c *Client) deleteInstance(hostname string, rawUrl string) {
	i, err := c.makeInstance(hostname, rawUrl)
	if err != nil {
		return
	}

	h := makeHostname(hostname, i)

	if s, ok := c.services[h]; ok {
		delete(s.instances, rawUrl)
		go c.notify(ServiceEvent{
			EventType: ServiceInstanceDeleted,
			Instance:  i,
		})
		// TODO should we unregister the service when all all the instances are offline?
		//if len(s.instances) == 0 {
		//	c.deleteService(i.Service)
		//}

	}
}

func (c *Client) addInstance(hostname string, rawUrl string) {
	i, err := c.makeInstance(hostname, rawUrl)
	if err != nil {
		return
	}

	s := c.addService(hostname, i)
	i.Service = s
	s.instances[rawUrl] = i
	go c.notify(ServiceEvent{
		EventType: ServiceInstanceAdded,
		Instance:  i,
	})
}

func (c *Client) addService(hostname string, instance *Instance) *Service {
	h := makeHostname(hostname, instance)
	s, ok := c.services[h]
	if !ok {
		s = &Service{
			name:      h,
			ports:     make([]*Port, 0),
			instances: make(map[string]*Instance),
		}
		c.services[h] = s
		s.AddPort(instance.Port)
		go c.notify(ServiceEvent{
			EventType: ServiceAdded,
			Service:   s,
		})
	}
	return s
}

func (c *Client) deleteService(hostname string) {
	cache, ok := c.pcaches[hostname]
	if ok {
		cache.stop()
	}

	for h, s := range c.services {
		if strings.HasSuffix(h, hostname) {
			delete(c.services, h)
			go c.notify(ServiceEvent{
				EventType: ServiceDeleted,
				Service:   s,
			})
		}
	}
}

func (c *Client) notify(event ServiceEvent) {
	c.out <- event
}

func makeHostname(hostname string, instance *Instance) string {
	return hostname + ":" + instance.Labels["version"]
}
