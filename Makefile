all:
	export GOPATH=`pwd` && go build -o ipquery src/qrcode/main.go
