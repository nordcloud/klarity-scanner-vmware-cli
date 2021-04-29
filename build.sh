GOOS=windows GOARCH=amd64 go build -o KlarityVMwareScanner.exe main.go
GOOS=linux GOARCH=amd64 go build -o KlarityVMwareScanner-linux main.go
GOOS=darwin GOARCH=amd64 go build -o KlarityVMwareScanner-macos main.go
