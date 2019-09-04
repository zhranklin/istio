package bootstrap

import (
	"github.com/hashicorp/go-multierror"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/log"
	rlslib "istio.io/istio/pkg/rls"
	"strconv"
	"strings"
)

// initRlsClient creates the RLS client if running with rate limiter service enable.
func (s *Server) initRlsClient(args *PilotArgs) error {
	if args.RLSServerAddrs != nil && len(args.RLSServerAddrs) > 0 {
		client, rlserr := rlslib.CreateClientSet(args.RLSServerAddrs)
		if rlserr != nil {
			return multierror.Prefix(rlserr, "failed to connect to RLS server ")
		}
		s.rlsClientSet = client
	}

	return nil
}

func (s *Server) initNsfEnviroment(args *PilotArgs, environment *model.Environment) {
	kvs := strings.Split(args.PortMappingManager, ",")
	for _, value := range kvs {
		protocol, src, dst := parsePortMapping(value)
		environment.PortManagerMap[protocol] = [2]int{src, dst}
	}
	vs := strings.Split(args.NsfUrlPrefix, ",")
	environment.NsfUrlPrefix = vs
	environment.NsfHostSuffix = args.NsfHostSuffix
	environment.EgressDomain = args.EgressDomain
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
