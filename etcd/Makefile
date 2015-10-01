# ! IMPORTANT ! We are no longer using `godeps` to run builds. You should
# 	be using the GO15VENDOREXPERIMENT flag, and your dependencies should
# 	all be in $DEIS/vendor

# Set these if they are not present in the environment.
export GOARCH ?= amd64
export GOOS ?= linux
export DEISCTL_UNITS ?= "../deisctl/units"
export DEV_REGISTRY ?= "$(shell docker-machine ip deis):5000"
export DEIS_REGISTRY ?= ${DEV_REGISTRY}

# Non-optional environment variables
export GO15VENDOREXPERIMENT=1
export CGO_ENABLED=0

# Environmental details
BINDIR := rootfs/bin
VERSION := $(shell git describe --tags)
LDFLAGS := "-s -X main.version=${VERSION}"
IMAGE := ${DEIS_REGISTRY}/deis/etcd:${VERSION}
RC := "${DEISCTL_UNITS}/rcs/deis-etcd-1.json"

build:
	go build -o ${BINDIR}/boot -a -installsuffix cgo -ldflags ${LDFLAGS}  boot.go

info:
	@echo "Version:    ${VERSION}"
	@echo "Registry:   ${DEIS_REGISTRY}"
	@echo "Go flags:   GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=${CGO_ENABLED}"
	@echo "Image:      ${IMAGE}"
	@echo "Units:      ${DEISCTL_UNITS}"

clean:
	-rm rootfs/bin/boot

docker-build:
	docker build --rm -t ${IMAGE} .

docker-push:
	docker push ${IMAGE}

kube-delete:
	-kubectl delete rc deis-etcd-1
	sleep 5

kube-rc:
	@# The real pattern to match is v[0-9]+.[0-9]+.[0-9]+-[0-9]+-[0-9a-z]{8}, but
	@# we want to find broken versions, too.
	perl -pi -e "s|[a-z0-9.:]+\/deis\/etcd:[0-9a-z-.]+|${IMAGE}|g" ${RC}
	kubectl create -f ${RC}

kube-update:
	perl -pi -e "s|[a-z0-9.:]+\/deis\/etcd:[0-9a-z-.]+|${IMAGE}|g" ${RC}
	kubectl update -f ${RC}

kube-service:
	-kubectl create -f ${DEISCTL_UNITS}/services/deis-etcd-1-service.json

test:
	@echo "TODO: This should start a Docker container and run tests in it."

all: build docker-build docker-push kube-clean kube-rc test

.PHONY: build clean docker-build docker-push all kube-clean kube-rc kube-serviceinfo
