package http

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// GetBody will fetch the url and return the body as a byte slice.
func GetBody(url string) ([]byte, error) {
	client := http.Client{Timeout: 5 * time.Minute}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request for %q: %w", url, err)
	}
	req.Header.Set("User-Agent", "quake-kube/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot get url %q: %w", url, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("failed to close response body: %v\n", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cannot get url %q: %s", url, http.StatusText(resp.StatusCode))
	}

	return io.ReadAll(resp.Body)
}

// GetUntil will keep polling the url until it returns a 200 OK.
func GetUntil(url string, stop <-chan struct{}) error {
	client := http.Client{Timeout: 1 * time.Second}

	for {
		select {
		case <-stop:
			return fmt.Errorf("not available: %q", url)
		default:
			resp, err := client.Get(url)
			if err != nil {
				continue
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					fmt.Printf("failed to close response body: %v\n", err)
				}
			}()
			return nil
		}
	}
}
