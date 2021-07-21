GO=go

klarity-scanner-vmware-cli-linux-amd64: main.go go.mod go.sum
	mkdir -p build
	GOOS=linux GOARCH=amd64 go build -o build/klarity-scanner-vmware-cli-linux-amd64 main.go

klarity-scanner-vmware-cli-win-amd64.exe: main.go go.mod go.sum
	mkdir -p build
	GOOS=windows GOARCH=amd64 go build -o build/klarity-scanner-vmware-cli-win-amd64.exe main.go

.PHONY: build dist
build: klarity-scanner-vmware-cli-linux-amd64 klarity-scanner-vmware-cli-win-amd64.exe
dist: build
	cp config.json build/
	
	cd build && zip klarity-scanner-vmware-cli-windows-amd64.zip klarity-scanner-vmware-cli-win-amd64.exe config.json
	cd build && zip klarity-scanner-vmware-cli-linux-amd64.zip klarity-scanner-vmware-cli-linux-amd64 config.json
	rm build/config.json