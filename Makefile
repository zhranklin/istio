## Copyright 2017 Istio Authors
##
## Licensed under the Apache License, Version 2.0 (the "License");
## you may not use this file except in compliance with the License.
## You may obtain a copy of the License at
##
##     http://www.apache.org/licenses/LICENSE-2.0
##
## Unless required by applicable law or agreed to in writing, software
## distributed under the License is distributed on an "AS IS" BASIS,
## WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
## See the License for the specific language governing permissions and
## limitations under the License.

#-----------------------------------------------------------------------------
# Global Variables
#-----------------------------------------------------------------------------
ISTIO_GO := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
export ISTIO_GO
SHELL := /bin/bash

# Current version, updated after a release.
VERSION ?= 1.0-dev

# locations where artifacts are stored
ISTIO_DOCKER_HUB ?= docker.io/istio
export ISTIO_DOCKER_HUB
ISTIO_GCS ?= istio-release/releases/$(VERSION)
ISTIO_URL ?= https://storage.googleapis.com/$(ISTIO_GCS)
ISTIO_CNI_HUB ?= gcr.io/istio-release
export ISTIO_CNI_HUB
ISTIO_CNI_TAG ?= master-latest-daily
export ISTIO_CNI_TAG

# cumulatively track the directories/files to delete after a clean
DIRS_TO_CLEAN:=
FILES_TO_CLEAN:=

# If GOPATH is not set by the env, set it to a sane value
GOPATH ?= $(shell cd ${ISTIO_GO}/../../..; pwd)
export GOPATH

# If GOPATH is made up of several paths, use the first one for our targets in this Makefile
GO_TOP := $(shell echo ${GOPATH} | cut -d ':' -f1)
export GO_TOP

# Note that disabling cgo here adversely affects go get.  Instead we'll rely on this
# to be handled in bin/gobuild.sh
# export CGO_ENABLED=0

# It's more concise to use GO?=$(shell which go)
# but the following approach uses a more efficient "simply expanded" :=
# variable instead of a "recursively expanded" =
ifeq ($(origin GO), undefined)
  GO:=$(shell which go)
endif
ifeq ($(GO),)
  $(error Could not find 'go' in path.  Please install go, or if already installed either add it to your path or set GO to point to its directory)
endif

LOCAL_ARCH := $(shell uname -m)
ifeq ($(LOCAL_ARCH),x86_64)
GOARCH_LOCAL := amd64
else
GOARCH_LOCAL := $(LOCAL_ARCH)
endif
export GOARCH ?= $(GOARCH_LOCAL)

LOCAL_OS := $(shell uname)
ifeq ($(LOCAL_OS),Linux)
   export GOOS_LOCAL = linux
else ifeq ($(LOCAL_OS),Darwin)
   export GOOS_LOCAL = darwin
else
   $(error "This system's OS $(LOCAL_OS) isn't recognized/supported")
   # export GOOS_LOCAL ?= windows
endif

export GOOS ?= $(GOOS_LOCAL)

export ENABLE_COREDUMP ?= false

# Enable Istio CNI in helm template commands
export ENABLE_ISTIO_CNI ?= false

# NOTE: env var EXTRA_HELM_SETTINGS can contain helm chart override settings, example:
# EXTRA_HELM_SETTINGS="--set istio-cni.excludeNamespaces={} --set istio-cni.tag=v0.1-dev-foo"

#-----------------------------------------------------------------------------
# Output control
#-----------------------------------------------------------------------------
# Invoke make VERBOSE=1 to enable echoing of the command being executed
export VERBOSE ?= 0
# Place the variable Q in front of a command to control echoing of the command being executed.
Q = $(if $(filter 1,$VERBOSE),,@)
# Use the variable H to add a header (equivalent to =>) to informational output
H = $(shell printf "\033[34;1m=>\033[0m")

# To build Pilot, Mixer and CA with debugger information, use DEBUG=1 when invoking make
goVerStr := $(shell $(GO) version | awk '{split($$0,a," ")}; {print a[3]}')
goVerNum := $(shell echo $(goVerStr) | awk '{split($$0,a,"go")}; {print a[2]}')
goVerMajor := $(shell echo $(goVerNum) | awk '{split($$0, a, ".")}; {print a[1]}')
goVerMinor := $(shell echo $(goVerNum) | awk '{split($$0, a, ".")}; {print a[2]}' | sed -e 's/\([0-9]\+\).*/\1/')
gcflagsPattern := $(shell ( [ $(goVerMajor) -ge 1 ] && [ ${goVerMinor} -ge 10 ] ) && echo 'all=' || echo '')

ifeq ($(origin DEBUG), undefined)
  BUILDTYPE_DIR:=release
else ifeq ($(DEBUG),0)
  BUILDTYPE_DIR:=release
else
  BUILDTYPE_DIR:=debug
  export GCFLAGS:=$(gcflagsPattern)-N -l
  $(info $(H) Build with debugger information)
endif

# Optional file including user-specific settings (HUB, TAG, etc)
-include .istiorc.mk

# @todo allow user to run for a single $PKG only?
PACKAGES_CMD := GOPATH=$(GOPATH) $(GO) list ./...
GO_EXCLUDE := /vendor/|.pb.go|.gen.go
GO_FILES_CMD := find . -name '*.go' | grep -v -E '$(GO_EXCLUDE)'

# Environment for tests, the directory containing istio and deps binaries.
# Typically same as GOPATH/bin, so tests work seemlessly with IDEs.

export ISTIO_BIN=$(GO_TOP)/bin
# Using same package structure as pkg/
export OUT_DIR=$(GO_TOP)/out
export ISTIO_OUT:=$(OUT_DIR)/$(GOOS)_$(GOARCH)/$(BUILDTYPE_DIR)
export ISTIO_OUT_LINUX:=$(OUT_DIR)/linux_amd64/$(BUILDTYPE_DIR)
export HELM=$(ISTIO_OUT)/helm

# scratch dir: this shouldn't be simply 'docker' since that's used for docker.save to store tar.gz files
ISTIO_DOCKER:=${ISTIO_OUT_LINUX}/docker_temp
# Config file used for building istio:proxy container.
DOCKER_PROXY_CFG?=Dockerfile.proxy

# scratch dir for building isolated images. Please don't remove it again - using
# ISTIO_DOCKER results in slowdown, all files (including multiple copies of envoy) will be
# copied to the docker temp container - even if you add only a tiny file, >1G of data will
# be copied, for each docker image.
DOCKER_BUILD_TOP:=${ISTIO_OUT_LINUX}/docker_build

# dir where tar.gz files from docker.save are stored
ISTIO_DOCKER_TAR:=${ISTIO_OUT_LINUX}/docker

# Populate the git version for istio/proxy (i.e. Envoy)
ifeq ($(PROXY_REPO_SHA),)
  export PROXY_REPO_SHA:=$(shell grep PROXY_REPO_SHA istio.deps  -A 4 | grep lastStableSHA | cut -f 4 -d '"')
endif

# Envoy binary variables Keep the default URLs up-to-date with the latest push from istio/proxy.

# OS-neutral vars. These currently only work for linux.
export ISTIO_ENVOY_VERSION ?= ${PROXY_REPO_SHA}
export ISTIO_ENVOY_DEBUG_URL ?= https://storage.googleapis.com/istio-build/proxy/envoy-debug-$(ISTIO_ENVOY_VERSION).tar.gz
export ISTIO_ENVOY_RELEASE_URL ?= https://storage.googleapis.com/istio-build/proxy/envoy-alpha-$(ISTIO_ENVOY_VERSION).tar.gz

# Envoy Linux vars.
export ISTIO_ENVOY_LINUX_VERSION ?= ${ISTIO_ENVOY_VERSION}
export ISTIO_ENVOY_LINUX_DEBUG_URL ?= ${ISTIO_ENVOY_DEBUG_URL}
export ISTIO_ENVOY_LINUX_RELEASE_URL ?= ${ISTIO_ENVOY_RELEASE_URL}
# Variables for the extracted debug/release Envoy artifacts.
export ISTIO_ENVOY_LINUX_DEBUG_DIR ?= ${OUT_DIR}/linux_amd64/debug
export ISTIO_ENVOY_LINUX_DEBUG_NAME ?= envoy-debug-${ISTIO_ENVOY_LINUX_VERSION}
export ISTIO_ENVOY_LINUX_DEBUG_PATH ?= ${ISTIO_ENVOY_LINUX_DEBUG_DIR}/${ISTIO_ENVOY_LINUX_DEBUG_NAME}
export ISTIO_ENVOY_LINUX_RELEASE_DIR ?= ${OUT_DIR}/linux_amd64/release
export ISTIO_ENVOY_LINUX_RELEASE_NAME ?= envoy-${ISTIO_ENVOY_VERSION}
export ISTIO_ENVOY_LINUX_RELEASE_PATH ?= ${ISTIO_ENVOY_LINUX_RELEASE_DIR}/${ISTIO_ENVOY_LINUX_RELEASE_NAME}

# Envoy macOS vars.
# TODO Change url when official envoy release for macOS is available
export ISTIO_ENVOY_MACOS_VERSION ?= 1.0.2
export ISTIO_ENVOY_MACOS_RELEASE_URL ?= https://github.com/istio/proxy/releases/download/${ISTIO_ENVOY_MACOS_VERSION}/istio-proxy-${ISTIO_ENVOY_MACOS_VERSION}-macos.tar.gz
# Variables for the extracted debug/release Envoy artifacts.
export ISTIO_ENVOY_MACOS_RELEASE_DIR ?= ${OUT_DIR}/darwin_amd64/release
export ISTIO_ENVOY_MACOS_RELEASE_NAME ?= envoy-${ISTIO_ENVOY_MACOS_VERSION}
export ISTIO_ENVOY_MACOS_RELEASE_PATH ?= ${ISTIO_ENVOY_MACOS_RELEASE_DIR}/${ISTIO_ENVOY_MACOS_RELEASE_NAME}

# Allow user-override for a local Envoy build.
export USE_LOCAL_PROXY ?= 0
ifeq ($(USE_LOCAL_PROXY),1)
  export ISTIO_ENVOY_LOCAL ?= $(realpath ${ISTIO_GO}/../proxy/bazel-bin/src/envoy/envoy)
  # Point the native paths to the local envoy build.
  ifeq ($(GOOS_LOCAL), Darwin)
    export ISTIO_ENVOY_MACOS_RELEASE_DIR = $(dirname ${ISTIO_ENVOY_LOCAL})
    export ISTIO_ENVOY_MACOS_RELEASE_PATH = ${ISTIO_ENVOY_LOCAL}
  else
    export ISTIO_ENVOY_LINUX_DEBUG_DIR = $(dirname ${ISTIO_ENVOY_LOCAL})
    export ISTIO_ENVOY_LINUX_RELEASE_DIR = $(dirname ${ISTIO_ENVOY_LOCAL})
    export ISTIO_ENVOY_LINUX_DEBUG_PATH = ${ISTIO_ENVOY_LOCAL}
    export ISTIO_ENVOY_LINUX_RELEASE_PATH = ${ISTIO_ENVOY_LOCAL}
  endif
endif

GO_VERSION_REQUIRED:=1.10

HUB?=istio
ifeq ($(HUB),)
  $(error "HUB cannot be empty")
endif

# If tag not explicitly set in users' .istiorc.mk or command line, default to the git sha.
TAG ?= $(shell git rev-parse --verify HEAD)
ifeq ($(TAG),)
  $(error "TAG cannot be empty")
endif

VARIANT :=
ifeq ($(VARIANT),)
  TAG_VARIANT:=${TAG}
else
  TAG_VARIANT:=${TAG}-${VARIANT}
endif

PULL_POLICY ?= IfNotPresent
ifeq ($(TAG),latest)
  PULL_POLICY = Always
endif
ifeq ($(PULL_POLICY),)
  $(error "PULL_POLICY cannot be empty")
endif

GEN_CERT := ${ISTIO_BIN}/generate_cert

# Set Google Storage bucket if not set
GS_BUCKET ?= istio-artifacts

.PHONY: default
default: depend build test

# The point of these is to allow scripts to query where artifacts
# are stored so that tests and other consumers of the build don't
# need to be updated to follow the changes in the Makefiles.
# Note that the query needs to pass the same types of parameters
# (e.g., DEBUG=0, GOOS=linux) as the actual build for the query
# to provide an accurate result.
.PHONY: where-is-out where-is-docker-temp where-is-docker-tar
where-is-out:
	@echo ${ISTIO_OUT}
where-is-docker-temp:
	@echo ${ISTIO_DOCKER}
where-is-docker-tar:
	@echo ${ISTIO_DOCKER_TAR}

#-----------------------------------------------------------------------------
# Target: depend
#-----------------------------------------------------------------------------
.PHONY: depend init

# Parse out the x.y or x.y.z version and output a single value x*10000+y*100+z (e.g., 1.9 is 10900)
# that allows the three components to be checked in a single comparison.
VER_TO_INT:=awk '{split(substr($$0, match ($$0, /[0-9\.]+/)), a, "."); print a[1]*10000+a[2]*100+a[3]}'

# using a sentinel file so this check is only performed once per version.  Performance is
# being favored over the unlikely situation that go gets downgraded to an older version
check-go-version: | $(ISTIO_BIN) ${ISTIO_BIN}/have_go_$(GO_VERSION_REQUIRED)
${ISTIO_BIN}/have_go_$(GO_VERSION_REQUIRED):
	@if test $(shell $(GO) version | $(VER_TO_INT) ) -lt \
                 $(shell echo "$(GO_VERSION_REQUIRED)" | $(VER_TO_INT) ); \
                 then printf "go version $(GO_VERSION_REQUIRED)+ required, found: "; $(GO) version; exit 1; fi
	@touch ${ISTIO_BIN}/have_go_$(GO_VERSION_REQUIRED)


# Downloads envoy, based on the SHA defined in the base pilot Dockerfile
init: check-go-version $(ISTIO_OUT)/istio_is_init
	mkdir -p ${OUT_DIR}/logs

# Sync is the same as init in release branch. In master this pulls from master.
sync: init

# I tried to make this dependent on what I thought was the appropriate
# lock file, but it caused the rule for that file to get run (which
# seems to be about obtaining a new version of the 3rd party libraries).
$(ISTIO_OUT)/istio_is_init: bin/init.sh istio.deps | $(ISTIO_OUT)
	ISTIO_OUT=$(ISTIO_OUT) bin/init.sh
	touch $(ISTIO_OUT)/istio_is_init

# init.sh downloads envoy
${ISTIO_OUT}/envoy: init
${ISTIO_ENVOY_LINUX_DEBUG_PATH}: init
${ISTIO_ENVOY_LINUX_RELEASE_PATH}: init
${ISTIO_ENVOY_MACOS_RELEASE_PATH}: init

# Pull dependencies, based on the checked in Gopkg.lock file.
# Developers must manually run `dep ensure` if adding new deps
depend: init | $(ISTIO_OUT)

OUTPUT_DIRS = $(ISTIO_OUT) $(ISTIO_BIN)
DIRS_TO_CLEAN+=${ISTIO_OUT}
ifneq ($(ISTIO_OUT),$(ISTIO_OUT_LINUX))
  OUTPUT_DIRS += $(ISTIO_OUT_LINUX)
  DIRS_TO_CLEAN += $(ISTIO_OUT_LINUX)
endif

$(OUTPUT_DIRS):
	@mkdir -p $@

# Used by CI for automatic go code generation and generates a git diff of the generated files against HEAD.
go.generate.diff: $(ISTIO_OUT)
	git diff HEAD > $(ISTIO_OUT)/before_go_generate.diff
	-go generate ./...
	git diff HEAD > $(ISTIO_OUT)/after_go_generate.diff
	diff $(ISTIO_OUT)/before_go_generate.diff $(ISTIO_OUT)/after_go_generate.diff

.PHONY: ${GEN_CERT}
${GEN_CERT}:
	GOOS=$(GOOS_LOCAL) && GOARCH=$(GOARCH_LOCAL) && CGO_ENABLED=1 bin/gobuild.sh $@ ./security/tools/generate_cert

#-----------------------------------------------------------------------------
# Target: precommit
#-----------------------------------------------------------------------------
.PHONY: precommit format format.gofmt format.goimports lint buildcache

# Target run by the pre-commit script, to automate formatting and lint
# If pre-commit script is not used, please run this manually.
precommit: format lint

format:
	scripts/run_gofmt.sh

fmt:
	scripts/run_gofmt.sh

# Build with -i to store the build caches into $GOPATH/pkg
buildcache:
	GOBUILDFLAGS=-i $(MAKE) build

JUNIT_LINT_TEST_XML ?= $(ISTIO_OUT)/junit_lint-tests.xml
# Existence of build cache .a files actually affects the results of
# some linters; they need to exist.
lint: $(JUNIT_REPORT) buildcache
	mkdir -p $(dir $(JUNIT_LINT_TEST_XML))
	set -o pipefail; \
	SKIP_INIT=1 bin/linters.sh \
	2>&1 | tee >($(JUNIT_REPORT) > $(JUNIT_LINT_TEST_XML))


shellcheck:
	bin/check_shell_scripts.sh

# @todo gometalinter targets?

#-----------------------------------------------------------------------------
# Target: go build
#-----------------------------------------------------------------------------

# gobuild script uses custom linker flag to set the variables.
# Params: OUT VERSION_PKG SRC

RELEASE_LDFLAGS='-extldflags -static -s -w'
DEBUG_LDFLAGS='-extldflags "-static"'

# Generates build both native and linux (needed by Docker) targets for an application
# Params:
# $(1): The base name for the generated target. If the name specified is "app", then targets will be generated for:
#   app, $(ISTIO_OUT)/app and additionally $(ISTIO_OUT_LINUX)/app if GOOS != linux.
# $(2): The path to the source directory for the application
# $(3): The value for LDFLAGS
define genTargetsForNativeAndDocker
$(ISTIO_OUT)/$(1):
	STATIC=0 GOOS=$(GOOS) GOARCH=amd64 LDFLAGS=$(3) bin/gobuild.sh $(ISTIO_OUT)/$(1) $(2)

.PHONY: $(1)
$(1):
	STATIC=0 GOOS=$(GOOS) GOARCH=amd64 LDFLAGS=$(3) bin/gobuild.sh $(ISTIO_OUT)/$(1) $(2)

ifneq ($(ISTIO_OUT),$(ISTIO_OUT_LINUX))
$(ISTIO_OUT_LINUX)/$(1):
	STATIC=0 GOOS=linux GOARCH=amd64 LDFLAGS=$(3) bin/gobuild.sh $(ISTIO_OUT_LINUX)/$(1) $(2)
endif

endef

# Build targets for istioctl
ISTIOCTL_BINS:=istioctl
$(foreach ITEM,$(ISTIOCTL_BINS),$(eval $(call genTargetsForNativeAndDocker,$(ITEM),./istioctl/cmd/$(ITEM),$(RELEASE_LDFLAGS))))

# Non-static istioctl targets. These are typically a build artifact.
${ISTIO_OUT}/istioctl-linux: depend
	STATIC=0 GOOS=linux LDFLAGS=$(RELEASE_LDFLAGS) bin/gobuild.sh $@ ./istioctl/cmd/istioctl
${ISTIO_OUT}/istioctl-osx: depend
	STATIC=0 GOOS=darwin LDFLAGS=$(RELEASE_LDFLAGS) bin/gobuild.sh $@ ./istioctl/cmd/istioctl
${ISTIO_OUT}/istioctl-win.exe: depend
	STATIC=0 GOOS=windows LDFLAGS=$(RELEASE_LDFLAGS) bin/gobuild.sh $@ ./istioctl/cmd/istioctl

# generate the istioctl completion files
${ISTIO_OUT}/istioctl.bash: istioctl
	${ISTIO_OUT}/istioctl collateral --bash && \
	mv istioctl.bash ${ISTIO_OUT}/istioctl.bash

${ISTIO_OUT}/_istioctl: istioctl
	${ISTIO_OUT}/istioctl collateral --zsh && \
	mv _istioctl ${ISTIO_OUT}/_istioctl

# Build targets for apps under ./pilot/cmd
PILOT_BINS:=pilot-discovery pilot-agent sidecar-injector
$(foreach ITEM,$(PILOT_BINS),$(eval $(call genTargetsForNativeAndDocker,$(ITEM),./pilot/cmd/$(ITEM),$(RELEASE_LDFLAGS))))

# Build targets for apps under ./mixer/cmd
MIXER_BINS:=mixs mixc
$(foreach ITEM,$(MIXER_BINS),$(eval $(call genTargetsForNativeAndDocker,$(ITEM),./mixer/cmd/$(ITEM),$(RELEASE_LDFLAGS))))

# Build targets for apps under ./mixer/tools
MIXER_TOOLS_BINS:=mixgen
$(foreach ITEM,$(MIXER_TOOLS_BINS),$(eval $(call genTargetsForNativeAndDocker,$(ITEM),./mixer/tools/$(ITEM),$(RELEASE_LDFLAGS))))

# Build targets for apps under ./galley/cmd
GALLEY_BINS:=galley
$(foreach ITEM,$(GALLEY_BINS),$(eval $(call genTargetsForNativeAndDocker,$(ITEM),./galley/cmd/$(ITEM),$(RELEASE_LDFLAGS))))

# Build targets for apps under ./security/cmd
SECURITY_BINS:=node_agent node_agent_k8s istio_ca
$(foreach ITEM,$(SECURITY_BINS),$(eval $(call genTargetsForNativeAndDocker,$(ITEM),./security/cmd/$(ITEM),$(RELEASE_LDFLAGS))))

# Build targets for apps under ./security/tools
SECURITY_TOOLS_BINS:=sdsclient
$(foreach ITEM,$(SECURITY_TOOLS_BINS),$(eval $(call genTargetsForNativeAndDocker,$(ITEM),./security/tools/$(ITEM),$(RELEASE_LDFLAGS))))

# Build targets for apps under ./tools
ISTIO_TOOLS_BINS:=hyperistio istio-iptables
$(foreach ITEM,$(ISTIO_TOOLS_BINS),$(eval $(call genTargetsForNativeAndDocker,$(ITEM),./tools/$(ITEM),$(DEBUG_LDFLAGS))))

BUILD_BINS:=$(PILOT_BINS) mixc mixs mixgen node_agent node_agent_k8s istio_ca istioctl galley sdsclient
LINUX_BUILD_BINS:=$(foreach buildBin,$(BUILD_BINS),$(ISTIO_OUT_LINUX)/$(buildBin))

.PHONY: build
# Build will rebuild the go binaries.
build: depend $(BUILD_BINS)

.PHONY: build-linux
build-linux: depend $(LINUX_BUILD_BINS)

.PHONY: version-test
# Do not run istioctl since is different (connects to kubernetes)
version-test:
	go test ./tests/version/... -v --base-dir ${ISTIO_OUT} --binaries="$(PILOT_BINS) mixc mixs mixgen node_agent node_agent_k8s istio_ca galley sdsclient"

# The following are convenience aliases for most of the go targets
# The first block is for aliases that are the same as the actual binary,
# while the ones that follow need slight adjustments to their names.
#
# This is intended for developer use - will rebuild the package.

.PHONY: citadel
citadel: istio_ca

.PHONY: pilot
pilot: pilot-discovery

# istioctl-all makes all of the non-static istioctl executables for each supported OS
.PHONY: istioctl-all
istioctl-all: ${ISTIO_OUT}/istioctl-linux ${ISTIO_OUT}/istioctl-osx ${ISTIO_OUT}/istioctl-win.exe

.PHONY: istioctl.completion
istioctl.completion: ${ISTIO_OUT}/istioctl.bash ${ISTIO_OUT}/_istioctl

.PHONY: istio-archive

istio-archive: ${ISTIO_OUT}/archive

# TBD: how to capture VERSION, ISTIO_DOCKER_HUB, ISTIO_URL as dependencies
${ISTIO_OUT}/archive: istioctl-all istioctl.completion LICENSE README.md install/updateVersion.sh release/create_release_archives.sh
	rm -rf ${ISTIO_OUT}/archive
	mkdir -p ${ISTIO_OUT}/archive/istioctl
	cp ${ISTIO_OUT}/istioctl-* ${ISTIO_OUT}/archive/istioctl/
	cp LICENSE ${ISTIO_OUT}/archive
	cp README.md ${ISTIO_OUT}/archive
	cp -r tools ${ISTIO_OUT}/archive
	cp ${ISTIO_OUT}/istioctl.bash ${ISTIO_OUT}/archive/tools/
	cp ${ISTIO_OUT}/_istioctl ${ISTIO_OUT}/archive/tools/
	ISTIO_RELEASE=1 install/updateVersion.sh -a "$(ISTIO_DOCKER_HUB),$(VERSION)" \
		-P "$(ISTIO_URL)/deb" \
		-d "${ISTIO_OUT}/archive"
	release/create_release_archives.sh -v "$(VERSION)" -o "${ISTIO_OUT}/archive"

# istioctl-install builds then installs istioctl into $GOPATH/BIN
# Used for debugging istioctl during dev work
.PHONY: istioctl-install
istioctl-install:
	go install istio.io/istio/istioctl/cmd/istioctl

#-----------------------------------------------------------------------------
# Target: test
#-----------------------------------------------------------------------------

.PHONY: test localTestEnv test-bins

JUNIT_REPORT := $(shell which go-junit-report 2> /dev/null || echo "${ISTIO_BIN}/go-junit-report")

${ISTIO_BIN}/go-junit-report:
	@echo "go-junit-report not found. Installing it now..."
	unset GOOS && unset GOARCH && CGO_ENABLED=1 go get -u github.com/jstemmer/go-junit-report

# Run coverage tests
JUNIT_UNIT_TEST_XML ?= $(ISTIO_OUT)/junit_unit-tests.xml
ifeq ($(WHAT),)
       TEST_OBJ = common-test pilot-test mixer-test security-test galley-test istioctl-test
else
       TEST_OBJ = selected-pkg-test
endif
test: | $(JUNIT_REPORT)
	mkdir -p $(dir $(JUNIT_UNIT_TEST_XML))
	set -o pipefail; \
	KUBECONFIG="$${KUBECONFIG:-$${GO_TOP}/src/istio.io/istio/.circleci/config}" \
	$(MAKE) --keep-going $(TEST_OBJ) \
	2>&1 | tee >($(JUNIT_REPORT) > $(JUNIT_UNIT_TEST_XML))

GOTEST_PARALLEL ?= '-test.parallel=1'
# This is passed to mixer and other tests to limit how many builds are used.
# In CircleCI, set in "Project Settings" -> "Environment variables" as "-p 2" if you don't have xlarge machines
GOTEST_P ?=

TEST_APP_BINS:=server client
$(foreach ITEM,$(TEST_APP_BINS),$(eval $(call genTargetsForNativeAndDocker,pkg-test-echo-cmd-$(ITEM),./pkg/test/echo/cmd/$(ITEM),$(DEBUG_LDFLAGS))))

MIXER_TEST_BINS:=policybackend
$(foreach ITEM,$(MIXER_TEST_BINS),$(eval $(call genTargetsForNativeAndDocker,mixer-test-$(ITEM),./mixer/test/$(ITEM),$(DEBUG_LDFLAGS))))

TEST_BINS:=$(foreach ITEM,$(TEST_APP_BINS),$(ISTIO_OUT)/pkg-test-echo-cmd-$(ITEM)) $(foreach ITEM,$(MIXER_TEST_BINS),$(ISTIO_OUT)/mixer-test-$(ITEM))
LINUX_TEST_BINS:=$(foreach ITEM,$(TEST_APP_BINS),$(ISTIO_OUT_LINUX)/pkg-test-echo-cmd-$(ITEM)) $(foreach ITEM,$(MIXER_TEST_BINS),$(ISTIO_OUT_LINUX)/mixer-test-$(ITEM))

test-bins: $(TEST_BINS)

test-bins-linux: $(LINUX_TEST_BINS)

localTestEnv: test-bins
	bin/testEnvLocalK8S.sh ensure

localTestEnvCleanup: test-bins
	bin/testEnvLocalK8S.sh stop

# Temp. disable parallel test - flaky consul test.
# https://github.com/istio/istio/issues/2318
.PHONY: pilot-test
pilot-test: pilot-agent
	go test -p 1 ${T} ./pilot/...

.PHONY: istioctl-test
istioctl-test: istioctl
	go test -p 1 ${T} ./istioctl/...

.PHONY: mixer-test
MIXER_TEST_T ?= ${T} ${GOTEST_PARALLEL}
mixer-test: mixs
	# Some tests use relative path "testdata", must be run from mixer dir
	(cd mixer; go test -p 1 ${MIXER_TEST_T} ./...)

.PHONY: galley-test
galley-test: depend
	go test ${T} ./galley/...

.PHONY: security-test
security-test:
	go test ${T} ./security/pkg/...
	go test ${T} ./security/cmd/...

.PHONY: common-test
common-test: istio-iptables
	go test ${T} ./pkg/...
	go test ${T} ./tests/common/...
	# Execute bash shell unit tests scripts
	./tests/scripts/scripts_test.sh
	./tests/scripts/istio-iptables-test.sh

.PHONY: selected-pkg-test
selected-pkg-test:
	find ${WHAT} -name "*_test.go" | xargs -I {} dirname {} | uniq | xargs -I {} go test ${T} ./{}

#-----------------------------------------------------------------------------
# Target: coverage
#-----------------------------------------------------------------------------

.PHONY: coverage

# Run coverage tests
coverage: pilot-coverage mixer-coverage security-coverage galley-coverage common-coverage istioctl-coverage

.PHONY: pilot-coverage
pilot-coverage:
	bin/codecov.sh pilot

.PHONY: istioctl-coverage
istioctl-coverage:
	bin/codecov.sh istioctl

.PHONY: mixer-coverage
mixer-coverage:
	bin/codecov.sh mixer

.PHONY: galley-coverage
galley-coverage:
	bin/codecov.sh galley

.PHONY: security-coverage
security-coverage:
	bin/codecov.sh security/pkg
	bin/codecov.sh security/cmd

.PHONY: common-coverage
common-coverage:
	bin/codecov.sh pkg

#-----------------------------------------------------------------------------
# Target: go test -race
#-----------------------------------------------------------------------------

.PHONY: racetest

JUNIT_RACE_TEST_XML ?= $(ISTIO_OUT)/junit_race-tests.xml
RACE_TESTS ?= pilot-racetest mixer-racetest security-racetest galley-test common-racetest istioctl-racetest
racetest: $(JUNIT_REPORT)
	mkdir -p $(dir $(JUNIT_RACE_TEST_XML))
	set -o pipefail; \
	$(MAKE) --keep-going $(RACE_TESTS) \
	2>&1 | tee >($(JUNIT_REPORT) > $(JUNIT_RACE_TEST_XML))

.PHONY: pilot-racetest
pilot-racetest: pilot-agent
	RACE_TEST=true go test -p 1 ${T} -race ./pilot/...

.PHONY: istioctl-racetest
istioctl-racetest: istioctl
	RACE_TEST=true go test -p 1 ${T} -race ./istioctl/...

.PHONY: mixer-racetest
mixer-racetest: mixs
	# Some tests use relative path "testdata", must be run from mixer dir
	(cd mixer; RACE_TEST=true go test -p 1 ${T} -race ./...)

.PHONY: galley-racetest
galley-racetest: depend
	RACE_TEST=true go test ${T} -race ./galley/...

.PHONY: security-racetest
security-racetest:
	RACE_TEST=true go test ${T} -race ./security/pkg/... ./security/cmd/...

.PHONY: common-racetest
common-racetest:
	RACE_TEST=true go test ${T} -race ./pkg/...

#-----------------------------------------------------------------------------
# Target: clean
#-----------------------------------------------------------------------------
.PHONY: clean clean.go

clean: clean.go
	rm -rf $(DIRS_TO_CLEAN)
	rm -f $(FILES_TO_CLEAN)

clean.go: ; $(info $(H) cleaning...)
	$(eval GO_CLEAN_FLAGS := -i -r)
	$(Q) $(GO) clean $(GO_CLEAN_FLAGS)

#-----------------------------------------------------------------------------
# Target: docker
#-----------------------------------------------------------------------------
.PHONY: artifacts gcs.push.istioctl-all artifacts installgen

# for now docker is limited to Linux compiles - why ?
include tools/istio-docker.mk

# if first part of URL (i.e., hostname) is gcr.io then upload istioctl and deb
$(if $(findstring gcr.io,$(firstword $(subst /, ,$(HUB)))),$(eval push: gcs.push.istioctl-all gcs.push.deb),)

push: docker.push installgen

gcs.push.istioctl-all: istioctl-all
	gsutil -m cp -r "${ISTIO_OUT}"/istioctl-* "gs://${GS_BUCKET}/pilot/${TAG}/artifacts/istioctl"

gcs.push.deb: deb
	gsutil -m cp -r "${ISTIO_OUT}"/*.deb "gs://${GS_BUCKET}/pilot/${TAG}/artifacts/debs/"

artifacts: docker
	@echo 'To be added'

# generate_yaml in tests/istio.mk can build without specifying a hub & tag
installgen:
	install/updateVersion.sh -a ${HUB},${TAG}
	$(MAKE) istio.yaml

$(HELM): $(ISTIO_OUT)
	bin/init_helm.sh

$(HOME)/.helm:
	$(HELM) init --client-only

# create istio-init.yaml
istio-init.yaml: $(HELM) $(HOME)/.helm
	cat install/kubernetes/namespace.yaml > install/kubernetes/$@
	cat install/kubernetes/helm/istio-init/files/crd-* >> install/kubernetes/$@
	$(HELM) template --name=istio --namespace=istio-system \
		--set global.tag=${TAG_VARIANT} \
		--set global.hub=${HUB} \
		install/kubernetes/helm/istio-init >> install/kubernetes/$@


# This is used to @include values-istio-demo-common.yaml file
istio-demo.yaml istio-demo-auth.yaml: export EXTRA_HELM_SETTINGS+=--values install/kubernetes/helm/istio/values-istio-demo-common.yaml

# creates istio-demo.yaml istio-demo-auth.yaml istio-remote.yaml
# Ensure that values-$filename is present in install/kubernetes/helm/istio
istio-demo.yaml istio-demo-auth.yaml istio-remote.yaml istio-minimal.yaml: $(HELM) $(HOME)/.helm
	cat install/kubernetes/namespace.yaml > install/kubernetes/$@
	cat install/kubernetes/helm/istio-init/files/crd-* >> install/kubernetes/$@
	$(HELM) template \
		--name=istio \
		--namespace=istio-system \
		--set global.tag=${TAG_VARIANT} \
		--set global.hub=${HUB} \
		--set global.imagePullPolicy=$(PULL_POLICY) \
		--set global.proxy.enableCoreDump=${ENABLE_COREDUMP} \
		--set istio_cni.enabled=${ENABLE_ISTIO_CNI} \
		${EXTRA_HELM_SETTINGS} \
		--values install/kubernetes/helm/istio/values-$@ \
		install/kubernetes/helm/istio >> install/kubernetes/$@

e2e_files = istio-auth-non-mcp.yaml \
			istio-auth-sds.yaml \
			istio-non-mcp.yaml \
			istio.yaml \
			istio-auth.yaml \
			istio-auth-mcp.yaml \
			istio-auth-multicluster.yaml \
			istio-mcp.yaml \
			istio-one-namespace.yaml \
			istio-one-namespace-auth.yaml \
			istio-one-namespace-trust-domain.yaml \
			istio-multicluster.yaml \
			istio-multicluster-split-horizon.yaml \

.PHONY: generate_e2e_yaml generate_e2e_yaml_coredump
generate_e2e_yaml: $(e2e_files)

generate_e2e_yaml_coredump: export ENABLE_COREDUMP=true
generate_e2e_yaml_coredump:
	$(MAKE) generate_e2e_yaml

# Create yaml files for e2e tests. Applies values-e2e.yaml, then values-$filename.yaml
$(e2e_files): $(HELM) $(HOME)/.helm istio-init.yaml
	cat install/kubernetes/namespace.yaml > install/kubernetes/$@
	cat install/kubernetes/helm/istio-init/files/crd-* >> install/kubernetes/$@
	$(HELM) template \
		--name=istio \
		--namespace=istio-system \
		--set global.tag=${TAG_VARIANT} \
		--set global.hub=${HUB} \
		--set global.imagePullPolicy=$(PULL_POLICY) \
		--set global.proxy.enableCoreDump=${ENABLE_COREDUMP} \
		--set istio_cni.enabled=${ENABLE_ISTIO_CNI} \
		${EXTRA_HELM_SETTINGS} \
		--values install/kubernetes/helm/istio/test-values/values-e2e.yaml \
		--values install/kubernetes/helm/istio/test-values/values-$@ \
		install/kubernetes/helm/istio >> install/kubernetes/$@

# files generated by the default invocation of updateVersion.sh
FILES_TO_CLEAN+=install/consul/istio.yaml \
                install/kubernetes/istio-auth.yaml \
                install/kubernetes/istio-citadel-plugin-certs.yaml \
                install/kubernetes/istio-citadel-with-health-check.yaml \
                install/kubernetes/istio-one-namespace-auth.yaml \
                install/kubernetes/istio-one-namespace-trust-domain.yaml \
                install/kubernetes/istio-one-namespace.yaml \
                install/kubernetes/istio.yaml \
                samples/bookinfo/platform/consul/bookinfo.sidecars.yaml \

#-----------------------------------------------------------------------------
# Target: environment and tools
#-----------------------------------------------------------------------------
.PHONY: show.env show.goenv

show.env: ; $(info $(H) environment variables...)
	$(Q) printenv

show.goenv: ; $(info $(H) go environment...)
	$(Q) $(GO) version
	$(Q) $(GO) env

# tickle
# show makefile variables. Usage: make show.<variable-name>
show.%: ; $(info $* $(H) $($*))
	$(Q) true

#-----------------------------------------------------------------------------
# Target: artifacts and distribution
#-----------------------------------------------------------------------------
.PHONY: dist dist-bin

${ISTIO_OUT}/dist/Gopkg.lock:
	mkdir -p ${ISTIO_OUT}/dist
	cp Gopkg.lock ${ISTIO_OUT}/dist/

# Binary/built artifacts of the distribution
dist-bin: ${ISTIO_OUT}/dist/Gopkg.lock

dist: dist-bin

include .circleci/Makefile

# deb, rpm, etc packages
include tools/packaging/packaging.mk

#-----------------------------------------------------------------------------
# Target: e2e tests
#-----------------------------------------------------------------------------
include tests/istio.mk

#-----------------------------------------------------------------------------
# Target: integration tests
#-----------------------------------------------------------------------------
include tests/integration/tests.mk

#-----------------------------------------------------------------------------
# Target: bench check
#-----------------------------------------------------------------------------

.PHONY: benchcheck
benchcheck:
	bin/perfcheck.sh

include Makefile.common.mk
