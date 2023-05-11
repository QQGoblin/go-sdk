package httputil

import (
	"github.com/cenkalti/backoff/v4"
	"io/ioutil"
	"net/http"
	"time"
)

func Healthz(cli *http.Client, endpoint string, interval time.Duration, attempts uint64) ([]byte, error) {

	var contents []byte

	f := func() error {
		resp, err := cli.Get(endpoint)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusForbidden {
			return err
		}

		contents, err = ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()

		return err
	}
	if err := backoff.Retry(f, backoff.WithMaxRetries(backoff.NewConstantBackOff(interval), attempts)); err != nil {
		return nil, err
	}

	return contents, nil
}
