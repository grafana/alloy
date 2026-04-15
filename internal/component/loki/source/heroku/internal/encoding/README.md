# logplex/encoding

This is a copy of the [github.com/heroku/x/logplex/encoding](https://github.com/heroku/x/tree/master/logplex/encoding) package.
The copy was made because the upstream repository depends on the legacy aws-sdk-go v1, which is EOL.

## What's this?

A set of libraries we use to parse messages, and to also publish these same
syslog RFC5424 messages.

## How to use?

We have 2 scanners available. If you're trying to build a logplex compatible ingress,
you can use the regular scanner.

### Scanner

```go
func handler(w http.ResponseWriter, r *http.Request) {
	s := NewScanner(r.Body)

	for s.Scan() {
		log.Printf("%+v", scanner.Message())
	}

	if s.Err() != nil {
		log.Printf("err: %v", s.Err())
	}
}
```

### DrainScanner

If the intent is to write an application which acts as a heroku drain,
then using the DrainScanner is preferrable -- primarily because it doesn't
require structured data.

```
func handler(w http.ResponseWriter, r *http.Request) {
	s := NewDrainScanner(r.Body)

	for s.Scan() {
		log.Printf("%+v", scanner.Message())
	}

	if s.Err() != nil {
		log.Printf("err: %v", s.Err())
	}
}
```
