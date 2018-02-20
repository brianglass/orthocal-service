docker:
	docker build -t orthocal-service .
	docker tag orthocal-service:latest brianglass/orthocal-service:latest
	docker push brianglass/orthocal-service:latest

docker-run:
	docker run -it -p8080:8080 orthocal-service:latest
