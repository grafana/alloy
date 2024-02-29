package s3

import (
	"io"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"

	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type watcher struct {
	mut        sync.Mutex
	bucket     string
	file       string
	output     chan result
	dlTicker   *time.Ticker
	downloader *s3.Client
}

type result struct {
	result []byte
	err    error
}

func newWatcher(
	bucket, file string,
	out chan result,
	frequency time.Duration,
	downloader *s3.Client,
) *watcher {

	return &watcher{
		bucket:     bucket,
		file:       file,
		output:     out,
		dlTicker:   time.NewTicker(frequency),
		downloader: downloader,
	}
}

func (w *watcher) updateValues(bucket, file string, frequency time.Duration, downloader *s3.Client) {
	w.mut.Lock()
	defer w.mut.Unlock()
	w.bucket = bucket
	w.file = file
	w.dlTicker.Reset(frequency)
	w.downloader = downloader
}

func (w *watcher) run(ctx context.Context) {
	w.download(ctx)
	defer w.dlTicker.Stop()
	for {
		select {
		case <-w.dlTicker.C:
			w.download(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// download actually downloads the file from s3
func (w *watcher) download(ctx context.Context) {
	w.mut.Lock()
	defer w.mut.Unlock()
	buf, err := w.getObject(context.Background())
	r := result{
		result: buf,
		err:    err,
	}
	select {
	case <-ctx.Done():
		return
	case w.output <- r:
	}
}

func (w *watcher) downloadSynchronously() (string, error) {
	w.mut.Lock()
	defer w.mut.Unlock()
	buf, err := w.getObject(context.Background())
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// getObject ensure that the return []byte is never nil
func (w *watcher) getObject(ctx context.Context) ([]byte, error) {
	output, err := w.downloader.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(w.bucket),
		Key:    aws.String(w.file),
	})
	if err != nil {
		return []byte{}, err
	}
	defer output.Body.Close()

	buf := make([]byte, *output.ContentLength)

	_, err = io.ReadFull(output.Body, buf)

	if err != nil {
		return []byte{}, err
	}

	return buf, nil
}
