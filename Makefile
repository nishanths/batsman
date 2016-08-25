init: 
	rm -rf myblog
	go run *.go init myblog

build:
	rm -rf here
	go build
	./styx init here
	./styx -workdir here build

new:
	rm -rf here
	go build
	./styx init here
	./styx -workdir here new -title hello -draft
