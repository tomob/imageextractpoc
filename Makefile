.PHONY: all help build clean vendor binary-static binary-dynamic

#CONTAINER_RUNTIME := $(shell command -v podman 2> /dev/null || echo docker)
CONTAINER_RUNTIME := docker
GO ?= go
GPGME_ENV := CGO_CFLAGS="$(shell gpgme-config --cflags 2>/dev/null)" CGO_LDFLAGS="$(shell gpgme-config --libs 2>/dev/null)"
GIT_COMMIT := $(shell git rev-parse HEAD 2> /dev/null || true)

ifeq ($(DEBUG), 1)
  override GOGCFLAGS += -N -l
endif

# Go module support: set `-mod=vendor` to use the vendored sources.
# See also hack/make.sh.
ifeq ($(shell go help mod >/dev/null 2>&1 && echo true), true)
  GO:=GO111MODULE=on $(GO)
  MOD_VENDOR=-mod=vendor
endif

BTRFS_BUILD_TAG = $(shell hack/btrfs_tag.sh) $(shell hack/btrfs_installed_tag.sh)
LIBDM_BUILD_TAG = $(shell hack/libdm_tag.sh)
LOCAL_BUILD_TAGS = $(BTRFS_BUILD_TAG) $(LIBDM_BUILD_TAG)
BUILDTAGS += $(LOCAL_BUILD_TAGS)

ifeq ($(DISABLE_CGO), 1)
	override BUILDTAGS = exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp
endif

all: binary

help:
	@echo "Usage: make <target>"
	@echo
	@echo " * 'build-container' - Build Docker build image"
	@echo " * 'binary' - Build image-extract-poc with a container"
	@echo " * 'clean' - Clean artifacts"

binary: cmd/image-extract-poc/*.go build-container
	${CONTAINER_RUNTIME} run --rm --security-opt label=disable -v $$(pwd):/src/github.com/tomo/imageextractpoc \
		image-extract-poc-image make binary-dynamic $(if $(DEBUG),DEBUG=$(DEBUG)) BUILDTAGS='$(BUILDTAGS)'

vendor:
	${CONTAINER_RUNTIME} run --rm --security-opt label=disable -v $$(pwd):/src/github.com/tomo/imageextractpoc image-extract-poc-image make go.nod

binary-static:
	$(GPGME_ENV) $(GO) build $(MOD_VENDOR) \
		-ldflags "-extldflags \"-static\" -X main.gitCommit=${GIT_COMMIT}" \
		-gcflags "$(GOGCFLAGS)" \
		-tags "$(BUILDTAGS)" \
		-o image-extract-poc \
		./cmd/image-extract-poc

binary-dynamic:
	$(GPGME_ENV) $(GO) build $(MOD_VENDOR) \
		-ldflags "-X main.gitCommit=${GIT_COMMIT}" \
		-gcflags "$(GOGCFLAGS)" \
		-tags "$(BUILDTAGS)" \
		-o image-extract-poc \
		./cmd/image-extract-poc

build-container: Dockerfile.build
	@${CONTAINER_RUNTIME} build ${BUILD_ARGS} -f Dockerfile.build -t image-extract-poc-image .

go.mod: cmd/image-extract-poc/*.go
	# $(GO) mod tidy
	$(GO) mod vendor
	$(GO) mod verify

clean:
	rm -f image-extract-poc
	${CONTAINER_RUNTIME} rmi image-extract-poc-image