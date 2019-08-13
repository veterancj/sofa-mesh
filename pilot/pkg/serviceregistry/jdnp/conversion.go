package jdnp

//将JSF注册中心open api服务返回的数据解析成Service对象
func convertToService( jdnpdata *JdNpDNSJsonObj ) map[string]*Service {
	serviceMap := make(map[string]*Service)
	if jdnpdata != nil && jdnpdata.ResStatus == 200 && len(jdnpdata.Data) > 0 {
		for _, insJsonObj := range jdnpdata.Data {
			instance := convertToInstance( insJsonObj )
			if instance != nil {
				hostname := jdnpdata.Domain
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
	return serviceMap;
}

func convertToInstance( ip string ) *Instance {
	//判断获取的实例必须是存活，并且上线状态
	if ip != "" {
		instance := &Instance{
			Host:ip,
			Port:&Port{
				Protocol:"HTTP",
				Port:"80",
			},
			Labels:make(map[string]string),
		}
		return instance
	} else {
		return nil
	}
}
