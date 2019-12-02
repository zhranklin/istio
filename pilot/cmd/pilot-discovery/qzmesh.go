package main

func processQingZhouMeshExtensionFlags() {
	discoveryCmd.PersistentFlags().StringVar(&serverArgs.PortMappingManager, "portMapping", "http|8550:80",
		"Comma separated list of protocol default port")
	discoveryCmd.PersistentFlags().StringVar(&serverArgs.NsfHostSuffix, "nsfHostSuffix", "",
		"Suffix for exspansion domain")
}