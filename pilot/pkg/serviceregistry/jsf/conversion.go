package jsf

import "strconv"

//将JSF注册中心open api服务返回的数据解析成Service对象
func convertToService( serviceJsonObj *ServiceJsonObj) map[string]*Service {
	serviceMap := make(map[string]*Service)
	if serviceJsonObj != nil && serviceJsonObj.Success && len(serviceJsonObj.Result) > 0 {
		for _, insJsonObj := range serviceJsonObj.Result {
			instance := convertToInstance( &insJsonObj )
			if instance != nil {
				hostname := makeHostname(instance)
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

func convertToInstance( insJsonObj *InstanceJsonObj ) *Instance {
	//判断获取的实例必须是存活，并且上线状态
	if insJsonObj != nil && insJsonObj.Status == 1 && insJsonObj.OptType == 1 {
		instance := &Instance{
			Host:insJsonObj.Ip,
			Port:&Port{
				Protocol:strconv.Itoa(insJsonObj.Protocol),
				Port:strconv.Itoa(insJsonObj.Port),
			},
			Labels:make(map[string]string),
		}
		instance.Labels["insKey"] = insJsonObj.InsKey
		instance.Labels["weight"] = strconv.Itoa(insJsonObj.Weight)
		instance.Labels["pid"] = strconv.Itoa(insJsonObj.Pid)
		instance.Labels["room"] = strconv.Itoa(insJsonObj.Room)
		instance.Labels["srcType"] = strconv.Itoa(insJsonObj.SrcType)
		instance.Labels["timeout"] = strconv.Itoa(insJsonObj.Timeout)
		instance.Labels["optType"] = strconv.Itoa(insJsonObj.OptType)
		instance.Labels["random"] = strconv.FormatBool(insJsonObj.Random)
		instance.Labels["uniqKey"] = insJsonObj.UniqKey
		instance.Labels["alias"] = insJsonObj.Alias
		instance.Labels["delTime"] = strconv.Itoa(insJsonObj.DelTime)
		instance.Labels["startTime"] = strconv.FormatInt(insJsonObj.StartTime,10)
		instance.Labels["interfaceName"] = insJsonObj.InterfaceName
		instance.Labels["status"] = strconv.Itoa(insJsonObj.Status)
		instance.Labels["protocol"] = strconv.Itoa(insJsonObj.Protocol)
		return instance
	} else {
		return nil
	}
}

//接口名为服务集群信息，其他的存入label标签
func makeHostname(instance *Instance) string {
	return instance.Labels["interfaceName"] //+ ":" + instance.Labels["alias"]
}