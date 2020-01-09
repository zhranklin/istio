package version

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"istio.io/api/networking/v1alpha3"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/pkg/log"
)

type handler func(w http.ResponseWriter, r *http.Request)

type route struct {
	regexPattern *regexp.Regexp
	handlerFunc  handler
}

type RegexpHanlder struct {
	routes []*route
}

func (s *Server) initHttpHandler(args *Args) error {
	//pattern: /{namespace}/version
	s.HandlerFunc(regexp.MustCompile(`/(.*)/version`), s.defaultVersion)

	return nil
}

func (s *Server) HandlerFunc(pattern *regexp.Regexp, f func(w http.ResponseWriter, r *http.Request)) {
	r := route{
		regexPattern: pattern,
		handlerFunc:  f,
	}
	s.handler.routes = append(s.handler.routes, &r)
}

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	log.Infof("receive request uri:[%v], header:[%v]", r.RequestURI, r.Header)
	for _, route := range s.handler.routes {
		if route.regexPattern.MatchString(path) {
			route.handlerFunc(w, r)
			return
		}
	}
	log.Warnf("there is not handler found for request:[%v]", path)
}

func (s *Server) ToErrorResponse(w http.ResponseWriter, err error) {
	var defaultMessage = `{"err":"unknown error occurs"}`
	const defaultCode string = "500"

	w.Header().Set("status", defaultCode)
	if err != nil {
		defaultMessage = fmt.Sprint(err)
	}
	message := struct{ err string }{
		err: defaultMessage,
	}

	bytes, err := json.Marshal(message)
	_, err = w.Write(bytes)
	if err != nil {
		log.Warna(err)
	}
	return
}

func (s *Server) defaultVersion(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	patterns := regexp.MustCompile(`/(.*)/version`).FindAllStringSubmatch(path, 1)
	vmCrd, err := s.svmClient.Get(patterns[0][1])
	if err != nil {
		log.Warna(err)
		s.ToErrorResponse(w, err)
		return
	}
	proto, err := model.VersionManager.FromJSONMap(vmCrd.Spec)
	if err != nil {
		log.Warna(err)
		s.ToErrorResponse(w, err)
		return
	}
	vm, ok := proto.(*v1alpha3.VersionManager)
	if !ok {
		errMsg := "type assertion failed. original type:[proto.Message], assert type:[v1alpha3.VersionManager]"
		log.Warn(errMsg)
		s.ToErrorResponse(w, errors.New(errMsg))
		return
	}
	result := struct {
		Version string
	}{Version: vm.DefaultVersion}
	w.Header().Set("status", "200")
	w.Header().Set("context-type", "application/json")
	bytes, err := json.Marshal(result)
	if err != nil {
		log.Warna(err)
		s.ToErrorResponse(w, err)
		return
	}
	_, err = w.Write(bytes)
	if err != nil {
		log.Warna(err)
	}
}
