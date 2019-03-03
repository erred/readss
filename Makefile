.PHONY: all build clean run test
all: build run

build:
	docker build -t readss --network host .

clean:
	go clean

run:
	docker run --network host readss

test:
	go test

.PHONY: redeploy
redeploy:
	kubectl delete po -l app=readss

.PHONY: icon
icon:
	convert -background black -density 1200 \
		logo.svg \
		\( +clone -resize 512x512 -quality 60 -write static/icon-512.png +delete \) \
		\( +clone -resize 192x192 -quality 60 -write static/icon-192.png +delete \) \
		\( +clone -resize 64x64 -quality 60 -write static/icon-64.png +delete \) \
		\( +clone -resize 32x32 -quality 60 -write static/icon-32.png +delete \) \
		\( +clone -resize 16x16 -quality 60 -write static/icon-16.png +delete \) \
		-resize 16x16 -quality 60 static/icon.ico

