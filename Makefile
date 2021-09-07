all: amavis-watch.go
	CGO_ENABLED=0 go build -ldflags="-s -w" amavis-watch.go


