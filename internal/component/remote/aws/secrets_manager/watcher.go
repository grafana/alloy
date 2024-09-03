package secrets_manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type watcher struct {
	mut           sync.Mutex
	secretId      string
	secretVersion string
	output        chan result
	pollTicker    *time.Ticker
	client        *secretsmanager.Client
}

type result struct {
	secret        string
	secretId      string
	secretVersion string
	err           error
}

func (r result) Validate() bool {
	return r.secretId != "" && r.secretVersion != "" && r.err == nil
}

func newWatcher(
	secretId, secretVersion string,
	out chan result,
	frequency time.Duration,
	client *secretsmanager.Client,
) *watcher {
	w := &watcher{
		secretId:      secretId,
		secretVersion: secretVersion,
		output:        out,
		client:        client,
	}

	if frequency > 0 {
		w.pollTicker = time.NewTicker(frequency)
	}
	return w
}

func (w *watcher) updateValues(secretId, secretVersion string, frequency time.Duration, client *secretsmanager.Client) {
	w.mut.Lock()
	defer w.mut.Unlock()
	w.secretId = secretId
	w.secretVersion = secretVersion
	if w.pollTicker != nil && frequency > 0 {
		w.pollTicker.Reset(frequency)
	}
	w.client = client
}

func (w *watcher) run(ctx context.Context) {
	w.getSecretAsynchronously(ctx)
	defer w.pollTicker.Stop()
	for {
		select {
		case <-w.pollTicker.C:
			w.getSecretAsynchronously(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (w *watcher) getSecretAsynchronously(ctx context.Context) {
	w.mut.Lock()
	defer w.mut.Unlock()

	res := w.getSecret(context.Background())

	select {
	case <-ctx.Done():
		return
	case w.output <- res:
	}
}

func (w *watcher) getSecret(ctx context.Context) result {
	res := result{}
	fetchedResult, err := w.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(w.secretId),
		VersionStage: aws.String(w.secretVersion),
	})

	if err != nil {
		res.err = err
	}

	if fetchedResult != nil {
		res.secret = *fetchedResult.SecretString
		res.secretId = *fetchedResult.Name
		res.secretVersion = *fetchedResult.VersionId
	} else {
		res.err = fmt.Errorf("error fetching secret %s from AWS Secrets Manager: %s", w.secretId, err.Error())
	}

	return res
}
