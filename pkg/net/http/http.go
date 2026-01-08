package http

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

// GetBody will fetch the url and return the body as a byte slice.
func GetBody(url string) ([]byte, error) {
	client := http.Client{Timeout: 5 * time.Minute}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request for %q: %v", url, err)
	}
	req.Header.Set("User-Agent", "quake-kube/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot get url %q: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("cannot get url %q: %v", url, http.StatusText(resp.StatusCode))
	}

	return io.ReadAll(resp.Body)
}

// GetUntil will keep polling the url until it returns a 200 OK.
func GetUntil(url string, stop <-chan struct{}) error {
	client := http.Client{Timeout: 1 * time.Second}

	for {
		select {
		case <-stop:
			return errors.Errorf("not available: %q", url)
		default:
			resp, err := client.Get(url)
			if err != nil {
				continue
			}
			resp.Body.Close()
			return nil
		}
	}
}
