
**NOTE**: This is a fork of https://github.com/grafana/tail, which is a fork of https://github.com/hpcloud/tail.
The `grafana/tail` repo is no longer mainained because the Loki team has deprecated the Promtail project.
It is easier for the Alloy team to maintain this tail package inside the Alloy repo than to have a separate repository for it.

Use outside of that context is not tested or supported.

# Go package for tail-ing files

A Go package striving to emulate the features of the BSD `tail` program.

```Go
t, err := tail.TailFile("/var/log/nginx.log", tail.Config{Follow: true})
for line := range t.Lines {
    fmt.Println(line.Text)
}
```

See [API documentation](http://godoc.org/github.com/hpcloud/tail).

## Log rotation

Tail comes with full support for truncation/move detection as it is
designed to work with log rotation tools.

## Installing

    go get github.com/hpcloud/tail/...

## Windows support

This package [needs assistance](https://github.com/hpcloud/tail/labels/Windows) for full Windows support.
