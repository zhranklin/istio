// Copyright 2018 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// how to generate proto client code
// https://grpc.io/docs/quickstart/go/

package rlsclient

import (
	"context"
	"google.golang.org/grpc"
	networking "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/log"
	pb "istio.io/istio/pkg/rls/config"
	"net/url"
	"time"
)

type ClientSetInterface interface {
	DoSharedConfigSync(config model.Config, event model.Event)
	AddRatelimitConfig(*networking.SharedConfig) error
	UpdateRatelimitConfig(*networking.SharedConfig) error
	DeleteRatelimitConfig(*networking.SharedConfig) error
	GetRatelimitConfig() error
	ListRatelimitConfigs() error
}

type ClientSet struct {
	conns   []*grpc.ClientConn
	clients []*pb.RatelimitConfigServiceClient
}

func (r ClientSet) DoSharedConfigSync(config model.Config, event model.Event) {
	if config.Spec == nil {
		return
	}
	shareConfig := config.Spec.(*networking.SharedConfig)

	switch event {
	case model.EventAdd:
		_ = r.AddRatelimitConfig(shareConfig)
		break
	case model.EventUpdate:
		_ = r.UpdateRatelimitConfig(shareConfig)
		break
	case model.EventDelete:
		_ = r.DeleteRatelimitConfig(shareConfig)
		break
	}
}

func transformRateLimiterConfig(sharedConfig *networking.SharedConfig) []*pb.RateLimitConf {
	rateLimitConfigs := make([]*pb.RateLimitConf, len(sharedConfig.RateLimitConfigs))
	for i, c := range sharedConfig.RateLimitConfigs {
		rateLimitConfigs[i] = &pb.RateLimitConf{}
		rateLimitConfigs[i].Domain = c.Domain
		rateLimitConfigs[i].Descriptors = make([]*pb.RateLimitDescriptorConfig, len(c.Descriptors))
		RecurCopDescriptors(&rateLimitConfigs[i].Descriptors, c.Descriptors)
	}
	return rateLimitConfigs
}

func RecurCopDescriptors(ppDescriptors *[]*pb.RateLimitDescriptorConfig, srcDescriptors []*networking.RateLimitDescriptor) {
	if srcDescriptors == nil {
		return
	}
	*ppDescriptors = make([]*pb.RateLimitDescriptorConfig, len(srcDescriptors))
	for i, d := range srcDescriptors {
		(*ppDescriptors)[i] = &pb.RateLimitDescriptorConfig{}
		(*ppDescriptors)[i].Key = d.Key
		(*ppDescriptors)[i].Value = d.Value
		if d.RateLimit != nil {
			t := pb.RateConfig_UnitType(d.RateLimit.Unit)
			(*ppDescriptors)[i].RateLimit = &pb.RateConfig{
				Unit:            t,
				RequestsPerUnit: d.RateLimit.RequestsPerUnit,
			}
		}
		RecurCopDescriptors(&(*ppDescriptors)[i].Descriptors, d.Descriptors)
	}
}

func (r ClientSet) AddRatelimitConfig(config *networking.SharedConfig) error {
	rlsConfigs := transformRateLimiterConfig(config)
	for _, config := range rlsConfigs {
		req := pb.AddRatelimitConfigReq{
			Config: config,
		}
		for _, client := range r.clients {
			ctx, _ := context.WithTimeout(context.Background(), time.Second)
			_, err := (*client).AddRatelimitConfig(ctx, &req)
			if err != nil {
				log.Errorf("AddRatelimitConfig failed: %v", err)
				return err
			}
		}
	}
	log.Info("AddRatelimitConfig successfully")
	return nil
}

func (r ClientSet) UpdateRatelimitConfig(config *networking.SharedConfig) error {
	rlsConfigs := transformRateLimiterConfig(config)
	for _, config := range rlsConfigs {
		req := pb.UpdateRatelimitConfigReq{
			Config: config,
		}
		for _, client := range r.clients {
			ctx, _ := context.WithTimeout(context.Background(), time.Second)
			_, err := (*client).UpdateRatelimitConfig(ctx, &req)
			if err != nil {
				log.Errorf("UpdateRatelimitConfig failed: %v", err)
				return err
			}
		}
	}
	log.Info("UpdateRatelimitConfig successfully")
	return nil
}

func (r ClientSet) DeleteRatelimitConfig(config *networking.SharedConfig) error {
	rlsConfigs := transformRateLimiterConfig(config)
	for _, config := range rlsConfigs {
		req := pb.DeleteRatelimitConfigReq{
			Domain: config.Domain,
		}
		for _, client := range r.clients {
			ctx, _ := context.WithTimeout(context.Background(), time.Second)
			_, err := (*client).DeleteRatelimitConfig(ctx, &req)
			if err != nil {
				log.Errorf("DeleteRatelimitConfig failed: %v", err)
				return err
			}
		}
	}
	log.Info("DeleteRatelimitConfig successfully")
	return nil
}

func (r ClientSet) GetRatelimitConfig() error {
	panic("implement me")
}

func (r ClientSet) ListRatelimitConfigs() error {
	panic("implement me")
}

func CreateClientSet(rlsServerAddrs []string) (ClientSetInterface, error) {
	var rlsClientSet ClientSetInterface
	clientSet := ClientSet{}
	clientSet.conns = make([]*grpc.ClientConn, len(rlsServerAddrs))
	clientSet.clients = make([]*pb.RatelimitConfigServiceClient, len(rlsServerAddrs))
	for i, addr := range rlsServerAddrs {
		u, err := url.Parse(addr)
		if err != nil {
			return nil, err
		}
		conn, err := grpc.Dial(u.Host, grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		//defer conn.Close()
		client := pb.NewRatelimitConfigServiceClient(conn)
		clientSet.conns[i] = conn
		clientSet.clients[i] = &client
	}
	rlsClientSet = clientSet
	return rlsClientSet, nil
}
