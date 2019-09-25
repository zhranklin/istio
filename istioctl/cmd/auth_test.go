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

package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"istio.io/istio/istioctl/pkg/auth"

	"istio.io/istio/pilot/test/util"
)

func runCommandAndCheckGoldenFile(name, command, golden string, t *testing.T) {
	out := runCommand(name, command, t)
	util.CompareContent(out.Bytes(), golden, t)
}

func runCommandAndCheckExpectedString(name, command, expected string, t *testing.T) {
	out := runCommand(name, command, t)
	if out.String() != expected {
		t.Errorf("test %q failed. \nExpected\n%s\nGot%s\n", name, expected, out.String())
	}
}

func runCommand(name, command string, t *testing.T) bytes.Buffer {
	t.Helper()
	var out bytes.Buffer
	rootCmd := GetRootCmd(strings.Split(command, " "))
	rootCmd.SetOutput(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Errorf("%s: unexpected error: %s", name, err)
	}
	return out
}

func TestAuthCheck(t *testing.T) {
	testCases := []struct {
		name   string
		in     string
		golden string
	}{
		{
			name:   "listeners and clusters",
			in:     "testdata/auth/productpage_config_dump.json",
			golden: "testdata/auth/productpage.golden",
		},
	}

	for _, c := range testCases {
		command := fmt.Sprintf("experimental auth check -f %s", c.in)
		runCommandAndCheckGoldenFile(c.name, command, c.golden, t)
	}
}

func TestAuthValidator(t *testing.T) {
	testCases := []struct {
		name     string
		in       []string
		expected string
	}{
		{
			name:     "good policy",
			in:       []string{"testdata/auth/authz-policy.yaml"},
			expected: auth.GetPolicyValidReport(),
		},
		{
			name: "bad policy",
			in:   []string{"testdata/auth/unused-role.yaml", "testdata/auth/notfound-role-in-binding.yaml"},
			expected: fmt.Sprintf("%s%s",
				auth.GetRoleNotFoundReport("some-role", "bind-service-viewer", "default"),
				auth.GetRoleNotUsedReport("unused-role", "default")),
		},
	}
	for _, c := range testCases {
		command := fmt.Sprintf("experimental auth validate -f %s", strings.Join(c.in, ","))
		runCommandAndCheckExpectedString(c.name, command, c.expected, t)
	}
}
