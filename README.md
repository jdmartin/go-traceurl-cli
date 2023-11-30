# go-traceurl-cli (Alpha Edition)

Implements the URL tracing (and partial cleaning, more work to do there) of my [go-traceurl](https://github.com/jdmartin/go-traceurl) tool, but on the cli.

### Build
I build it like this on my Mac. 

You'll want to change the GOOS and GOARCH to match your needs.

`env GOOS=darwin GOARCH=arm64 go build -o url-tracer -ldflags="-w -s" -gcflags "all=-N -l" -tags netgo .`

### Usage
url-tracer URL

