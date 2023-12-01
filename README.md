# go-traceurl-cli

Implements the URL tracing (and partial cleaning, more work to do there) of my [go-traceurl](https://github.com/jdmartin/go-traceurl) tool, but on the cli.


(N.B. I'm the world's okayest Go programmer (just learning). There are better tools in the world for this, probably.)

### Build
I build it like this on my Apple Silicon Mac. You'll want to change the GOOS and GOARCH to match your needs.

`env GOOS=darwin GOARCH=arm64 go build -o go-trace -ldflags="-w -s" -tags netgo .`

### Usage
go-trace [options] URL

Options:<br>
\-h: prints help message<br>
\-j: output as JSON<br>
\-s: short output. Just the Final/Clean URL<br>
\-v: verbose output (shows all hops)<br>
\-w: int, width of URL tab

Defaults:<br>
\-j: Off<br>
\-v: Off (Final/Clean URL only)<br>
\-w: 120

### Global Config:<br>

The program does support a config file. It will look in [$XDG_CONFIG_HOME](https://xdgbasedirectoryspecification.com/) to find go-trace.toml, or else it will check ~/.config/go-trace.toml.  You can use this file to create global defaults (maybe you always want JSON, or maybe you always want terse/verbose output, or maybe you want the width to be 80 chars like ~~God~~ IBM intended...)

Anyway, for available options, see the go-trace.toml.template file!
