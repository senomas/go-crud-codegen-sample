SHELL := /bin/bash
.SHELLFLAGS := -e -o pipefail -c
.PHONY: FORCE
.ONESHELL:

$(shell test -f .local.env || touch .local.env)

include .env
-include build.properties
-include .local.env

include ./docker.mk
BUILD_ENV ?= dev
RUN_TARGET ?= test
TRACKED_FILES=build.properties model/*.yml Dockerfile cmd handler migrations model util go.* *.go

test: init FORCE
	@$(MAKE) base-build
	@$(MAKE) model/.gen
	@$(MAKE) docs/.gen
	@$(MAKE) --no-print-directory init_test
	@$(MAKE) --no-print-directory model_test
	@$(MAKE) --no-print-directory handler_test

init_test: init FORCE
	@rm -rf test.log
	@$(DC) rm -f app-test
	@bargs=$$($(call build-args)); \
	echo docker build $$bargs --target=builder -t local/app-test:dev .; \
	docker build $$bargs --target=builder -t local/app-test:dev .
	@$(call start-service,"test-db")

migrate-test: FORCE
	@# only run pattern 00* for tests (no actual data)
	@${DC} run --rm app-test /app/migrate \
		--db db --prefix DB --drop --pattern "^00" /app/migrations || \
		(tail -n 100 test.log; exit 1)

migrate: FORCE
	@${DC} run --rm app /app/migrate \
		--db db --prefix DB --drop /app/migrations || \
		(tail -n 100 test.log; exit 1)

model_test: FORCE
	@$(MAKE) --no-print-directory migrate-test
	@${DC} run --rm app-test go test -v --failfast ./model/... || \
		(tail -n 100 test.log; exit 1)

handler_test: FORCE
	@$(MAKE) --no-print-directory migrate-test
	@${DC} run --rm app-test go test -v --failfast ./handler/ -v 10_ldap_test.go || \
		(tail -n 100 test.log; exit 1)

clean: FORCE
	@if [ ! -f .secret.env ]; then touch .secret.env; fi
	@${DC} down -v --remove-orphans
	@$(MAKE) clean-images
	@rm -rf .secret.env
	@rm -rf .*.build
	@rm -rf model/gen__* handler/gen__*
	@rm -rf model/.gen handler/.gen
	@rm -rf docs markdown

clean-images: FORCE
	@$(call clean-images,${DOCKER_REGISTRY}/app)
	@$(call clean-images,${DOCKER_REGISTRY}/bookworm-db)
	@$(call clean-images,${DOCKER_REGISTRY}/crudgen)
	@docker images -f "dangling=true" -q | xargs -r docker rmi 2>/dev/null || true

build-util: FORCE
	$(call docker-build,build-util,BUILD_UTIL,app-build-util)

base-build: FORCE
	$(call docker-build,base,DEBIAN_BASE,bookworm-db)

deploy: FORCE
	@$(MAKE) BUILD_ENV=prod build

build: init FORCE
	@mkdir -p model migrations handler util docs
	@$(MAKE) base-build
	@$(MAKE) model/.gen
	@$(MAKE) docs/.gen
	@$(MAKE) clean-images

model/.gen: build.properties model/*.yml
	@mkdir -p .cache/golang/pkg .cache/golang/cache
	@rm -rf model/gen__* handler/gen__*
	@build_args=$$($(call envs)); \
	if [ "${CODEGEN_PATH}" != "" ]; then \
		build_args="$$build_args -v $(shell pwd)/${CODEGEN_PATH}:/work/codegen"; \
	fi; \
	echo "Generating using ${DOCKER_REGISTRY}/crudgen:${CRUD_GEN_VER}: $$build_args"; \
	docker run --rm -it \
		-e LOCAL_USER_ID=$$(id -u) \
		-e LOCAL_GROUP_ID=$$(id -g) \
		-v $(shell pwd):/work/app \
		-v $(shell pwd)/.cache/golang/pkg:/work/.go/pkg \
		-v $(shell pwd)/.cache/golang/cache:/work/.go/cache \
		$$build_args  \
		-w /work/app \
		${DOCKER_REGISTRY}/crudgen:${CRUD_GEN_VER} \
		codegen postgresql example.com/app-api && \
	docker run --rm \
		-e LOCAL_USER_ID=$$(id -u) \
		-e LOCAL_GROUP_ID=$$(id -g) \
		-v $(shell pwd):/work/app \
		-v $(shell pwd)/.cache/golang/pkg:/work/.go/pkg \
		-v $(shell pwd)/.cache/golang/cache:/work/.go/cache \
		$$build_args  \
		-w /work/app \
		${DOCKER_REGISTRY}/crudgen:${CRUD_GEN_VER} \
		go vet ./...
	touch model/.gen

docs/.gen: build.properties *.go model/*.go handler/*.go
	@mkdir -p .cache/golang/pkg .cache/golang/cache
	@build_args=$$($(call envs)); \
	if [ "${CODEGEN_PATH}" != "" ]; then \
		build_args="$$build_args -v $(shell pwd)/${CODEGEN_PATH}:/work/codegen"; \
	fi; \
	rm -rf docs markdown && \
	docker run --rm \
		-e LOCAL_USER_ID=$$(id -u) \
		-e LOCAL_GROUP_ID=$$(id -g) \
		-v $(shell pwd):/work/app \
		-v $(shell pwd)/.cache/golang/pkg:/work/.go/pkg \
		-v $(shell pwd)/.cache/golang/cache:/work/.go/cache \
		$$build_args  \
		-w /work/app \
		${DOCKER_REGISTRY}/crudgen:${CRUD_GEN_VER} \
		swag init main.go -q -d ./swag --output docs | grep -v 'Override detected'
	@touch docs/.gen

init: FORCE
	@if [ ! -f .secret.env ]; then \
		DB_PASSWORD=$$(openssl rand -hex 16); \
		JWT_SECRET=$$(openssl rand -hex 16); \
		echo "DB_PASSWORD=$$DB_PASSWORD"        >  .secret.env; \
		echo "POSTGRES_PASSWORD=$$DB_PASSWORD"  >> .secret.env; \
		echo "JWT_SECRET=$$JWT_SECRET"          >> .secret.env; \
		echo "✅ Generated .secret.env"; \
	fi
