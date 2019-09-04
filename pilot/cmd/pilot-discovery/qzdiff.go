package main

func processNsfExtensionFlags() {
	// RLS client flags
	discoveryCmd.PersistentFlags().StringSliceVar(&serverArgs.RLSServerAddrs, "rlsServerAddrs", []string{},
		"comma separated list of RLS server addresses with "+
			"rls:// (insecure) or rlss:// (secure) schema, e.g. rlss://istio-rls.istio-system.svc:9901")
	//nsf-extension
	discoveryCmd.PersistentFlags().StringVar(&serverArgs.BackUpAddress, "backupAddress", "",
		"The address of backup gateway,  When the current cluster has no healthy instance, access the external"+
			" cluster through the backup gateway.")
	discoveryCmd.PersistentFlags().StringVar(&serverArgs.PortMappingManager, "portMapping", "http|8550:80",
		"Comma separated list of protocol default port")
	discoveryCmd.PersistentFlags().StringVar(&serverArgs.NsfUrlPrefix, "nsfUrlPrefix", "/proxy",
		"Comma separated list of url prefix, which will be proccessed by PortMappingManager")
	discoveryCmd.PersistentFlags().StringVar(&serverArgs.NsfHostSuffix, "nsfHostSuffix", "",
		"Suffix for exspansion domain")

	discoveryCmd.PersistentFlags().StringVar(&serverArgs.EgressDomain, "egressDomain", "egress",
		"default cluster for mapping port ( suffix matched host ) ")

}
