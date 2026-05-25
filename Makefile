ENVS := CGO_ENABLED=0
BUILD_DIR := build
BUILD_FLAGS := -ldflags="-s -w"

.PHONY: build yd

build: yd

yd:
	mkdir -p $(BUILD_DIR)
	${ENVS} go build ${BUILD_FLAGS} -o $(BUILD_DIR)
