init: 
	rm -rf myblog
	go run *.go init myblog

build:
	rm -rf here
	fmap -var rawFiles src/ | gofmt > file.go
	go build
	./styx init here
	cd here
	./styx build

new:
	rm -rf here
	go build
	./styx init here
	./styx -workdir here new -title hello -draft
