.PHONY: plugin
plugin: csn.go
	go build -buildmode=plugin --ldflags "-w -s" -o csn.plugin csn.go