package source

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"golang.org/x/oauth2"
)

// Failure reasons. They are the values of the reason label on
// infinity_source_poll_failures_total.
const (
	reasonRequest     = "request"
	reasonTimeout     = "timeout"
	reasonStatus      = "status"
	reasonTooLarge    = "too_large"
	reasonParse       = "parse"
	reasonPostprocess = "postprocess"
	reasonSeriesLimit = "series_limit"
	reasonEntryLimit  = "entry_limit"
	reasonEmit        = "emit"
)

// pollError is a poll failure. Health and warn logs show msg, which holds
// only values from the config and typed values such as a status code. The
// cause can hold server text, so only debug logs show it, through detail.
type pollError struct {
	reason string
	msg    string
	err    error
}

// Error returns the safe message. The reason stays out of this text; it is
// available as a metric label and a log field through reasonOf.
func (e *pollError) Error() string { return e.msg }
func (e *pollError) Unwrap() error { return e.err }

// detail returns the full text of the cause for debug logs. Text scrubbing
// is only a backstop here, because it cannot find every secret.
func (e *pollError) detail() string {
	if e.err == nil {
		return e.msg
	}
	return scrubURLs(e.err.Error())
}

// newPollError makes a poll error whose message is the text of err. Use it
// only for errors whose text this package builds from its own values.
func newPollError(reason string, err error) error {
	return &pollError{reason: reason, msg: err.Error(), err: err}
}

// maxLibErrorLen caps the library text in a parse or post-process message.
const maxLibErrorLen = 200

// newLibError makes a poll error for a parse or post-process failure. The
// text comes from the config expressions and the library, but it can quote
// the response, so it is scrubbed and capped.
func newLibError(reason string, err error) error {
	return &pollError{reason: reason, msg: capText(scrubURLs(err.Error()), maxLibErrorLen), err: err}
}

// capText cuts s to at most n runes. It cuts after scrubbing, so the cut
// cannot hide a URL from the scrubber.
func capText(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "..."
}

// newEmitError makes a poll error for a failed send. The receivers can
// return text from a remote server, so the message holds none of it.
func newEmitError(err error) *pollError {
	return &pollError{reason: reasonEmit, msg: "could not send the result to the outputs", err: redactErr(err)}
}

// asPollError makes sure err has a safe message. An error without a reason
// is a request error.
func asPollError(err error) *pollError {
	var pe *pollError
	if errors.As(err, &pe) {
		return pe
	}
	return &pollError{reason: reasonRequest, msg: "the poll failed", err: redactErr(err)}
}

// reasonOf returns the failure reason of err. An error without a reason is a
// request error.
func reasonOf(err error) string {
	var pe *pollError
	if errors.As(err, &pe) {
		return pe.reason
	}
	return reasonRequest
}

// requestTarget names a request in error messages. It holds only values
// from the config.
type requestTarget struct {
	rawURL  string
	timeout time.Duration
}

// Error text prefixes of net/http redirect failures. net/http has no typed
// errors for them, so this classification is best effort.
var redirectErrPrefixes = []string{
	"stopped after ",
	"failed to parse Location header",
	"net/http: HTTP/1.x transport connection broken: malformed MIME header",
}

// requestError classifies an error from client.Do. The message never holds
// the text of err, because that text can hold a redirect Location or an
// OAuth2 response.
func requestError(err error, req requestTarget, sent string) error {
	target := redactURL(req.rawURL)
	cause := redactErr(err)
	pe := func(reason, format string, args ...any) error {
		return &pollError{reason: reason, msg: fmt.Sprintf(format, args...), err: cause}
	}

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return pe(reasonTimeout, "request to %s timed out after %s", target, req.timeout)
	case errors.Is(err, context.Canceled):
		return pe(reasonRequest, "request to %s was canceled", target)
	}

	var retrieveErr *oauth2.RetrieveError
	if errors.As(err, &retrieveErr) {
		if retrieveErr.Response != nil {
			return pe(reasonRequest, "OAuth2 token request failed with status %d", retrieveErr.Response.StatusCode)
		}
		return pe(reasonRequest, "OAuth2 token request failed")
	}

	var ue *url.Error
	if !errors.As(err, &ue) {
		return pe(reasonRequest, "request to %s failed", target)
	}
	if isTokenError(ue.Err) {
		return pe(reasonRequest, "OAuth2 token request failed")
	}
	if isRedirectError(ue.Err) {
		return pe(reasonRequest, "invalid redirect from %s", target)
	}
	// After a redirect, a network error is about the server that the
	// Location names, not about the configured URL.
	if !sameURL(ue.URL, sent) {
		return pe(reasonRequest, "request to %s failed after a redirect", target)
	}

	var dnsErr *net.DNSError
	switch {
	case errors.As(err, &dnsErr):
		return pe(reasonRequest, "DNS lookup for %s failed", hostOf(req.rawURL))
	case errors.Is(err, syscall.ECONNREFUSED):
		return pe(reasonRequest, "connection to %s was refused", target)
	case isTLSError(err):
		return pe(reasonRequest, "TLS handshake with %s failed", target)
	}
	return pe(reasonRequest, "request to %s failed", target)
}

// isTokenError tells if err comes from the OAuth2 token source. The token
// request is the only request inside the outer request, so a nested
// *url.Error means that it failed.
func isTokenError(err error) bool {
	var inner *url.Error
	return errors.As(err, &inner) || strings.HasPrefix(err.Error(), "oauth2: ")
}

func isRedirectError(err error) bool {
	text := err.Error()
	for _, p := range redirectErrPrefixes {
		if strings.HasPrefix(text, p) {
			return true
		}
	}
	return false
}

func isTLSError(err error) bool {
	var (
		unknownAuthority x509.UnknownAuthorityError
		hostname         x509.HostnameError
		invalid          x509.CertificateInvalidError
		verification     *tls.CertificateVerificationError
		recordHeader     tls.RecordHeaderError
	)
	return errors.As(err, &unknownAuthority) || errors.As(err, &hostname) ||
		errors.As(err, &invalid) || errors.As(err, &verification) ||
		errors.As(err, &recordHeader)
}

// sameURL tells if got, the URL of a *url.Error, is sent, the URL of the
// request. net/http masks the password in got, so both lose their user info.
func sameURL(got, sent string) bool {
	g, err := url.Parse(got)
	if err != nil {
		return false
	}
	s, err := url.Parse(sent)
	if err != nil {
		return false
	}
	g.User, s.User = nil, nil
	return g.String() == s.String()
}

// hostOf returns the host name of rawURL, which comes from the config.
func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "<invalid url>"
	}
	return u.Hostname()
}
