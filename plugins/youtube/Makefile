.PHONY: plugin
plugin: youtube.go
	go build -buildmode=plugin --ldflags "-w -s" -o youtube.plugin youtube.go