package model

import "github.com/gogo/protobuf/proto"

func (ps *PushContext) initSharedConfigs(env *Environment) error {
	return nil
}

// ValidateSharedConfig checks that a v1alpha3 shared config is well-formed.
func ValidateSharedConfig(name, namespace string, msg proto.Message) (errs error) {
	//sharedConfig, ok := msg.(*networking.SharedConfig)
	//if !ok {
	//	return errors.New("cannot cast to shared config")
	//}
	//
	//appliesToMesh := false
	//if len(sharedConfig.RateLimitConfigs) == 0 {
	//	appliesToMesh = true
	//}
	//
	//errs = appendErrors(errs, validateGatewayNames(virtualService.Gateways))
	//for _, gateway := range virtualService.Gateways {
	//	if gateway == IstioMeshGateway {
	//		appliesToMesh = true
	//		break
	//	}
	//}
	//
	//if len(virtualService.Hosts) == 0 {
	//	errs = appendErrors(errs, fmt.Errorf("virtual service must have at least one host"))
	//}
	//
	//allHostsValid := true
	//for _, host := range virtualService.Hosts {
	//	if err := ValidateWildcardDomain(host); err != nil {
	//		ipAddr := net.ParseIP(host) // Could also be an IP
	//		if ipAddr == nil {
	//			errs = appendErrors(errs, err)
	//			allHostsValid = false
	//		}
	//	} else if appliesToMesh && host == "*" {
	//		errs = appendErrors(errs, fmt.Errorf("wildcard host * is not allowed for virtual services bound to the mesh gateway"))
	//		allHostsValid = false
	//	}
	//}
	//
	//// Check for duplicate hosts
	//// Duplicates include literal duplicates as well as wildcard duplicates
	//// E.g., *.foo.com, and *.com are duplicates in the same virtual service
	//if allHostsValid {
	//	for i := 0; i < len(virtualService.Hosts); i++ {
	//		hostI := Hostname(virtualService.Hosts[i])
	//		for j := i + 1; j < len(virtualService.Hosts); j++ {
	//			hostJ := Hostname(virtualService.Hosts[j])
	//			if hostI.Matches(hostJ) {
	//				errs = appendErrors(errs, fmt.Errorf("duplicate hosts in virtual service: %s & %s", hostI, hostJ))
	//			}
	//		}
	//	}
	//}
	//
	//if len(virtualService.Http) == 0 && len(virtualService.Tcp) == 0 && len(virtualService.Tls) == 0 {
	//	errs = appendErrors(errs, errors.New("http, tcp or tls must be provided in virtual service"))
	//}
	//for _, httpRoute := range virtualService.Http {
	//	errs = appendErrors(errs, validateHTTPRoute(httpRoute))
	//}
	//for _, tlsRoute := range virtualService.Tls {
	//	errs = appendErrors(errs, validateTLSRoute(tlsRoute, virtualService))
	//}
	//for _, tcpRoute := range virtualService.Tcp {
	//	errs = appendErrors(errs, validateTCPRoute(tcpRoute))
	//}
	//
	//errs = appendErrors(errs, validateExportTo(SharedConfig.ExportTo))
	return
}
