package jdnp

import (
	"istio.io/istio/pilot/pkg/serviceregistry/kube"
	kubelib "istio.io/istio/pkg/kube"
	"istio.io/istio/pkg/log"
	"sync"
	"testing"
	"time"
)

func TestRefreshService(t *testing.T) {
	var wg sync.WaitGroup;
	wg.Add(1)

	scanWorker := time.NewTicker( time.Second * 3 )

	//var urlStrList = []string{"TCP://pfinder-jmtp.jd.local:20560"}
	var urlStrList = []string{"TCP://mock-test.jsf.jd.com:20560"}

	var client *Client = &Client{
		urlStrList:    urlStrList,
		domainMapUrl:	convertToDomainURLSlice(urlStrList),
		refreshPeriod: 10,
		services:      make(map[string]*Service),
	}
	client.refreshServices()
	go func() {
		for {
			select {
			case <- scanWorker.C:
				if len(client.services) > 0 {
					wg.Done()
				}
			}
		}
	}()
	wg.Wait()
	if( len(client.services["pfinder-jmtp.jd.local"].instances)<5 ) {
		t.Errorf("updateServiceNameList after service pfinder-jmtp.jd.local instance num = %d, expect service num > 5!", len(client.services["pfinder-jmtp.jd.local"].instances))
	}
}

func TestConfigMapGetDomainUrl(t *testing.T){

	k8sClient, err := kubelib.CreateClientset("","");
	if err != nil {
		t.Errorf("CreateClientset('','') error : %s ,expect no error!", err)
	}

	var wg sync.WaitGroup;
	wg.Add(1)

	scanWorker := time.NewTicker( time.Second * 3 )
	stop := make(chan struct{})

	var urlStrList = []string{}
	var options = kube.ControllerOptions{
		WatchedNamespace: "bigmesh-system",
		ResyncPeriod: time.Second * 5,
	}

	var client = NewClient(urlStrList, 5, k8sClient, options);
	go func() {
		client.informer.Run(stop)
	}()
	go func() {
		for {
			select {
			case <- scanWorker.C:
				client.updateServiceNameList()
				if len(client.urlStrList) > 0 {
					close(stop)
					wg.Done()
				}
			}
		}
	}()
	wg.Wait()
	if len(client.urlStrList) <= 0 {
		t.Errorf("updateServiceNameList urlStrList num = %d, expect urlStrList num > 1!", len(client.urlStrList))
	}
}

func createClient(stop <-chan struct{}, t *testing.T) (clinet *Client) {
	k8sClient, err := kubelib.CreateClientset("","");
	if err != nil {
		t.Errorf("CreateClientset('','') error : %s ,expect no error!", err)
	}
	var urlStrList = []string{}
	var options = kube.ControllerOptions{
		WatchedNamespace: "bigmesh-system",
		ResyncPeriod: time.Second * 6,
	}
	clinet = NewClient(urlStrList, 6, k8sClient, options);
	clinet.Start(stop);
	return
}

func TestCreateClient(t *testing.T){
	var stop = make(chan struct{})
	createClient(stop, t);
	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()
}

func TestGetService(t *testing.T) {
	var stop = make(chan struct{})
	client := createClient(stop, t);
	ticker := time.NewTicker(4 * time.Second)
	if client != nil {
		defer func() {
			close(stop)
			ticker.Stop()
		}()
	}

	wg := sync.WaitGroup{}
	wg.Add(6)
	go func() {
		for {
			select {
			case <-ticker.C:
				services := client.Services()
				if len(services) > 0 {
					log.Infof("current services num : %d, content: %v", len(services),services)
				} else {
					log.Infof("current services is empty!!")
				}
			}
			wg.Done()
		}
	}()
	wg.Wait()
	if len(client.Services()) <= 0 {
		t.Errorf("client.Services() = empty, expect:>1! ")
	}
}


func TestGetServiceAfterSleep(t *testing.T) {
	var stop = make(chan struct{})
	client := createClient(stop, t);
	if client != nil {
		defer func() {
			close(stop)
		}()
	}

	//wg := sync.WaitGroup{}
	//wg.Add(1)
	//wg.Wait()
	time.Sleep(8 * time.Second)
	svcs := client.Services()
	log.Infof("client.Services():%d", len(svcs))

	if len(svcs) <= 0 {
		t.Errorf("client.Services() = empty, expect:>1! ")
	}
}
