
# Image URL to use all building/pushing image targets
IMG ?= ghcr.io/skai-x/elastic-jupyter-operator:latest
REGISTRY_IMG ?= ghcr.io/skai-x/enterprise-gateway:latest
REGISTRY_K8S_IMG ?= ghcr.io/skai-x/enterprise-gateway-with-kernel-spec:latest
KERNEL_PY_IMG ?= ghcr.io/skai-x/jupyter-kernel-py:2.6.0
KERNEL_R_IMG ?= ghcr.io/skai-x/jupyter-kernel-r:2.6.0
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/elastic-jupyter-operator main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen api-reference
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."

api-reference: install-tools ## Generate API reference documentation
	$(GOBIN)/crd-ref-docs \
		--source-path ./api/v1alpha1 \
		--config ./docs/api/autogen/config.yaml \
		--templates-dir ./docs/api/autogen/templates \
		--output-path ./docs/api/generated.asciidoc \
		--max-depth 30

# Build the docker image
docker-build: test
	docker build . -t ${IMG}

# Push the docker image
docker-push:
	docker buildx build --push --platform linux/amd64,linux/arm64 --tag ${IMG} .

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.18.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif

install-tools:
	go get github.com/elastic/crd-ref-docs

enterprise-gateway:
	cd enterprise_gateway && python setup.py bdist_wheel \
		&& rm -rf *.egg-info && cd - && \
		docker buildx build --push --platform linux/amd64,linux/arm64 --tag ${REGISTRY_IMG} -f enterprise_gateway/etc/docker/enterprise-gateway/Dockerfile .
	
enterprise-gateway-k8s:
	cd enterprise_gateway && make kernelspecs && \
		python setup.py bdist_wheel \
		&& rm -rf *.egg-info && cd - && \
		docker buildx build --push --platform linux/amd64,linux/arm64 --tag ${REGISTRY_K8S_IMG} -f enterprise_gateway/etc/docker/enterprise-gateway-k8s/Dockerfile .

kernel: kernel-py kernel-r

kernel-py:
	docker buildx build --push --platform linux/amd64,linux/arm64 --tag ${KERNEL_PY_IMG} -f enterprise_gateway/etc/docker/kernel-py/Dockerfile .

kernel-r:
	docker buildx build --push --platform linux/amd64,linux/arm64 --tag ${KERNEL_R_IMG} -f enterprise_gateway/etc/docker/kernel-r/Dockerfile .