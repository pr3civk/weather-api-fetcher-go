IMAGE        ?= zad-01-weather
TAG          ?= 1.0
DOCKERHUB    ?= pr3civk
PORT         ?= 6767
NAME         ?= weather

.PHONY: help build run logs stop clean rebuild dev push tag size layers inspect health

help:
	@echo "Targety:"
	@echo "  make dev      - go run lokalnie (bez Dockera)"
	@echo "  make build    - docker build"
	@echo "  make run      - docker run -d -p $(PORT):6767"
	@echo "  make logs     - docker logs (data, autor, port)"
	@echo "  make stop     - stop + rm kontenera"
	@echo "  make clean    - stop + rmi obrazu"
	@echo "  make rebuild  - clean && build && run"
	@echo "  make size     - rozmiar obrazu"
	@echo "  make layers   - liczba warstw + historia"
	@echo "  make inspect  - inspect obrazu (labels OCI)"
	@echo "  make health   - status healthcheck kontenera"
	@echo "  make tag      - tag pod DockerHub ($(DOCKERHUB)/$(IMAGE):$(TAG))"
	@echo "  make push     - docker login + tag + push"

dev:
	go run .

build:
	docker build -t $(IMAGE):$(TAG) .

run: stop
	docker run -d --name $(NAME) -p $(PORT):6767 $(IMAGE):$(TAG)

logs:
	docker logs $(NAME)

stop:
	-docker rm -f $(NAME) 2>/dev/null

clean: stop
	-docker rmi $(IMAGE):$(TAG) 2>/dev/null

rebuild: clean build run

size:
	@docker images $(IMAGE):$(TAG)
	@echo "---"
	@docker image inspect $(IMAGE):$(TAG) --format 'rozmiar: {{.Size}} B'

layers:
	@docker history $(IMAGE):$(TAG)
	@echo "---"
	@docker inspect $(IMAGE):$(TAG) --format 'warstwy fs: {{len .RootFS.Layers}}'

inspect:
	docker image inspect $(IMAGE):$(TAG) --format '{{json .Config.Labels}}' | sed 's/,/\n/g'

health:
	docker inspect --format='{{.State.Health.Status}}' $(NAME)

tag:
	docker tag $(IMAGE):$(TAG) $(DOCKERHUB)/$(IMAGE):$(TAG)
	docker tag $(IMAGE):$(TAG) $(DOCKERHUB)/$(IMAGE):latest

push: tag
	docker login
	docker push $(DOCKERHUB)/$(IMAGE):$(TAG)
	docker push $(DOCKERHUB)/$(IMAGE):latest
