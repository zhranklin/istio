package bootstrap

import (
	"istio.io/istio/pilot/pkg/model"
	"istio.io/pkg/log"
	"strconv"
	"strings"
)

func (s *Server) initNsfEnviroment(args *PilotArgs, environment *model.Environment) {
	kvs := strings.Split(args.PortMappingManager, ",")
	for _, value := range kvs {
		protocol, src, dst := parsePortMapping(value)
		environment.PortManagerMap[protocol] = [2]int{src, dst}
	}
	environment.NsfHostSuffix = args.NsfHostSuffix
}

func parsePortMapping(str string) (string, int, int) {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("Invalid port input: %v ,so skip", err)
		}
	}()
	per := strings.Split(str, "|")
	sd := strings.Split(per[1], ":")
	srcport, err := strconv.Atoi(sd[0])
	if err != nil {
		panic(err)
	}
	dstport, err := strconv.Atoi(sd[1])
	if err != nil {
		panic(err)
	}
	return per[0], srcport, dstport
}
