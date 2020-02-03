package internal

import (
	"net/http"
)

type HttpClient interface {
	Get(url string) (resp *http.Response, err error)
}

func ImagePull(client HttpClient, url string) error {
	return nil
}
