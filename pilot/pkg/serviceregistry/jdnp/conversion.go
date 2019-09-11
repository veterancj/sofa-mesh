package jdnp

import (
	"istio.io/istio/pkg/log"
	"net/url"
)

//将JSF注册中心open api服务返回的数据解析成Service对象
func convertToService( jdnpdata *JdNpDNSJsonObj, curUrls []*url.URL ) map[string]*Service {
	serviceMap := make(map[string]*Service)
	if jdnpdata != nil && jdnpdata.ResStatus == 200 && len(jdnpdata.Data) > 0 && len(curUrls) > 0 {
		for _, insJsonObj := range jdnpdata.Data {
			for _, curUrl := range curUrls {
				instance := convertToInstance( insJsonObj, curUrl )
				if instance != nil {
					hostname := curUrl.Hostname()
					s, ok := serviceMap[hostname]
					if(!ok){
						s = &Service{
							name:hostname,
							ports:make([]*Port,0),
							instances:make([]*Instance,0),
						}
						serviceMap[hostname] = s
					}
					instance.Service = s
					s.instances = append(s.instances, instance)
					s.AddPort(instance.Port)
				}
			}
		}
	}
	return serviceMap;
}

func convertToInstance( ip string, curUrl *url.URL ) *Instance {
	//判断获取的实例必须是存活，并且上线状态
	if ip != "" && curUrl != nil {
		instance := &Instance{
			Host:ip,
			Port:&Port{
				Protocol:curUrl.Scheme,
				Port:curUrl.Port(),
			},
			Labels:make(map[string]string),
		}
		if len(curUrl.Path) > 0 { instance.Labels["path"] = curUrl.Path }
		//如果Url有参数列表，进行解析
		for key, values := range curUrl.Query() {
			instance.Labels[key] = values[0];
		}
		return instance
	} else {
		return nil
	}
}

func convertToDomainURL( urlstr string ) (string, *url.URL) {
	var domian string
	var url *url.URL
	if(len(urlstr) > 0){
		tempUrl, err := url.Parse(urlstr)
		if( err != nil){
			log.Errorf("convertToURL have an error: %s",err)
		} else {
			url = tempUrl
			domian = url.Hostname()
		}
	}
	return domian,url
}

func convertToDomainURLSlice( urlStrSlice []string) map[string][]*url.URL {
	if( len(urlStrSlice) > 0 ){
		urlMap := make(map[string][]*url.URL)
		for _, urlStr := range urlStrSlice {
			if curDomain, curURL := convertToDomainURL(urlStr); curURL != nil {
				if _,exist := urlMap[curDomain] ; !exist {
					urlMap[curDomain] = make([]*url.URL,0)
				}
				urlMap[curDomain] = append(urlMap[curDomain], curURL)
			}
		}
		return urlMap
	}
	return nil
}

