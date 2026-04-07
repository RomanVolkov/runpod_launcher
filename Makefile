install:
	go build -o runpod-launcher ./cmd/runpod-launcher
	sudo mv runpod-launcher /usr/local/bin/
