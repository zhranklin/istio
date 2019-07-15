// Copyright 2019 Istio Authors
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

package mock

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"math/rand"
	"sync/atomic"
	"time"
)

type CAClient struct {
	signInvokeCount     uint64
	mockCertChain1st    []string
	mockCertChainRemain []string
}

func NewMockCAClient(mockCertChain1st, mockCertChainRemain []string) *CAClient {
	cl := CAClient{
		mockCertChain1st:    mockCertChain1st,
		mockCertChainRemain: mockCertChainRemain,
	}
	atomic.StoreUint64(&cl.signInvokeCount, 0)
	return &cl
}

func (c *CAClient) CSRSign(ctx context.Context, csrPEM []byte, exchangedToken string,
	certValidTTLInSec int64) ([]string /*PEM-encoded certificate chain*/, error) {
	// Mock CSRSign failure errors to force Citadel agent to retry.
	// 50% chance of failure.
	if rand.Intn(2) != 0 {
		return nil, status.Error(codes.Unavailable, "CA is unavailable")
	}

	if atomic.LoadUint64(&c.signInvokeCount) == 0 {
		atomic.AddUint64(&c.signInvokeCount, 1)
		return nil, status.Error(codes.Internal, "some internal error")
	}

	if atomic.LoadUint64(&c.signInvokeCount) == 1 {
		atomic.AddUint64(&c.signInvokeCount, 1)
		return c.mockCertChain1st, nil
	}

	return c.mockCertChainRemain, nil
}

type TokenExchangeServer struct {
}

func NewMockTokenExchangeServer() *TokenExchangeServer {
	return &TokenExchangeServer{}
}

func (s *TokenExchangeServer) ExchangeToken(context.Context, string, string) (string, time.Time, int, error) {
	// Mock ExchangeToken failure errors to force Citadel agent to retry.
	// 50% chance of failure.
	if rand.Intn(2) != 0 {
		return "", time.Time{}, 503, fmt.Errorf("service unavailable")
	}
	// Since the secret cache uses the k8s token in the stored secret, we can just return anything here.
	return "some-token", time.Now(), 200, nil
}
