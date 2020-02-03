package internal

import (
	"net/http"
	"testing"
)

func TestPull(t *testing.T) {
	mock := &ClientMock{}
	err := ImagePull(mock, "http://foo.bar")
	if err != nil {
		t.Errorf("Error returned %s", err)
	}
	if !mock.getCalled {
		t.Errorf("Get not called")
	}
}

type ClientMock struct {
	getCalled bool
}

func (c *ClientMock) Get(url string) (resp *http.Response, err error) {
	c.getCalled = true
	return &http.Response{}, nil
}
