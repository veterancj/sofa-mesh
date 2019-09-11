package jdnp

import (
	"go.uber.org/atomic"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pilot/pkg/serviceregistry/kube"
	kubelib "istio.io/istio/pkg/kube"
	"istio.io/istio/pkg/log"
	"testing"
	"time"
)

func createController(stop <-chan struct{}, serviceF func(*model.Service, model.Event), insF func(*model.ServiceInstance, model.Event), t *testing.T) (controller *Controller) {
	k8sClient, err := kubelib.CreateClientset("","");
	if err != nil {
		t.Errorf("CreateClientset('','') error : %s ,expect no error!", err)
	}
	var options = kube.ControllerOptions{
		WatchedNamespace: "bigmesh-system",
		ResyncPeriod: time.Second * 5,
	}

	controller , err = NewController("",5, k8sClient, options);
	if serviceF != nil { controller.AppendServiceHandler(serviceF)}
	if insF != nil { controller.AppendInstanceHandler(insF)}

	go controller.Run(stop)
	return
}

func TestServices(t *testing.T) {
	stop := make(chan struct{})
	controller := createController(stop, nil, nil, t)

	time.Sleep(8 * time.Second)

	svcs, err := controller.Services()
	if err != nil {
		log.Errorf("%s",err)
	}

	if len(svcs) <= 0 {
		t.Errorf("controller.Services() num = %d , expected > 0!", len(svcs))
	}
	stop <- struct{}{}
}


func TestInstances(t *testing.T) {
	stop := make(chan struct{})
	controller := createController(stop, nil, nil,  t)

	time.Sleep(8 * time.Second)

	ins, err := controller.Instances("mock-test.jsf.jd.com", []string{"tcp"}, model.LabelsCollection{})
	if err != nil {
		log.Errorf("%s",err)
	}

	if len(ins) < 3 {
		t.Errorf("controller.Services() num = %d , expected >= 3!", len(ins))
	}
	stop <- struct{}{}
}

func TestInstancesByPort(t *testing.T) {
	stop := make(chan struct{})
	controller := createController(stop, nil, nil,  t)

	time.Sleep(8 * time.Second)

	ins, err := controller.InstancesByPort("pfinder-jmtp.jd.local", 20560, model.LabelsCollection{})
	if err != nil {
		log.Errorf("%s",err)
	}

	if len(ins) < 3 {
		t.Errorf("controller.Services() num = %d , expected >= 3!", len(ins))
	}
	stop <- struct{}{}
}

var (
	svcCount = atomic.Int32{}
	insCount = atomic.Int32{}
)

func serviceListener(svc *model.Service, event model.Event){
	log.Infof("serviceListener service name : %s, event value: %d", svc.Hostname, event)
	if event == 0 { svcCount.Inc() }
	if event == 2 { svcCount.Dec() }
}

func instanceListener(ins *model.ServiceInstance, event model.Event){
	log.Infof("instanceListener ins ip:port : %s:%d, event value: %d", ins.Endpoint.Address, ins.Endpoint.Port, event)
	if event == 0 { insCount.Inc() }
	if event == 2 { insCount.Dec() }
}

func TestAppendServiceHandler(t *testing.T) {
	stop := make(chan struct{})
	ctl := createController(stop, nil, nil,  t)
	ctl.AppendServiceHandler(serviceListener)
	time.Sleep(3 * time.Second)

	delete(ctl.client.services, "pfinder-jmtp.jd.local")

	time.Sleep(8 * time.Second)
	if svcCount.Load() < 1 {
		t.Errorf("controller.Services() num = %d , expected >= 0!", len(ctl.client.Services()))
	}
	stop <- struct{}{}
}

func TestAppendInstanceHandler(t *testing.T) {
	stop := make(chan struct{})
	ctl := createController(stop, nil, nil,  t)
	ctl.AppendInstanceHandler(instanceListener)
	time.Sleep(8 * time.Second)

	ins := ctl.client.services["pfinder-jmtp.jd.local"].instances
	ctl.client.services["pfinder-jmtp.jd.local"].instances = ins[3:]

	time.Sleep(8 * time.Second)
	if insCount.Load() < 7 {
		t.Errorf("controller.Service.instance() num = %d , expected >= 7!", insCount.Load())
	}
	t.Logf("current controller.Service.instance num:%d", insCount.Load() )
	stop <- struct{}{}
}

func TestGetServiceAttributes(t *testing.T) {

	stop := make(chan struct{})
	ctl := createController(stop, nil, nil,  t)
	time.Sleep(8 * time.Second)

	attrs,err := ctl.GetServiceAttributes("pfinder-jmtp.jd.local")
	if err != nil {
		log.Errorf("%s",err)
	}

	exportBool := attrs.ExportTo["*"]
	if !exportBool {
		log.Errorf("GetServiceAttributes(\"pfinder-jmtp.jd.local\").ExportTo[\"*\"]=%v , expected=true",exportBool )
	}
	stop <- struct{}{}
}

func TestGetProxyServiceInstances(t *testing.T) {

	stop := make(chan struct{})
	ctl := createController(stop, nil, nil,  t)
	time.Sleep(8 * time.Second)

	var proxy *model.Proxy = new(model.Proxy)
	proxy.IPAddresses = append(proxy.IPAddresses, "11.17.229.245","11.18.165.197","11.18.84.217")
	instances,err := ctl.GetProxyServiceInstances(proxy)
	if err != nil {
		log.Errorf("%s",err)
	}

	if len(instances) < 3 {
		t.Errorf("GetProxyServiceInstances(proxy) instance num = %d , expected = 3!", len(instances))
	}
	stop <- struct{}{}

}

