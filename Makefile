NAMESPACE=orthocal

.PHONY: builder run deploy

docker: *.go templates/*
	docker build -t orthocal-service .
	touch docker

builder:
	docker build -t golang-sqlite builder
	docker tag golang-sqlite:latest brianglass/golang-sqlite:latest
	docker push brianglass/golang-sqlite:latest

run: docker
	docker run -it -p8080:8080 orthocal-service

deploy: docker
	docker tag orthocal-service:latest brianglass/orthocal-service:latest
	docker push brianglass/orthocal-service:latest
	kubectl delete pods -l app=orthocal --namespace=${NAMESPACE}
