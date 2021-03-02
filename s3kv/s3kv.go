// Package s3kv provide k/v storage by S3 Resty API
package s3kv

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/golang/snappy"

	"github.com/miinowy/go-base/client"
)

var (
	// Default context
	s3   *Context
	once sync.Once

	// Errors

	// ErrNetwork indicates a network error
	ErrNetwork = fmt.Errorf("s3kv: network error")
	// ErrNotFound indicates http status code is 404
	ErrNotFound = fmt.Errorf("s3kv: not found")
	// ErrForbidden indicates http status code is 403
	ErrForbidden = fmt.Errorf("s3kv: forbidden")
	// ErrStatusCode indicates got an unexcect status code
	ErrStatusCode = fmt.Errorf("s3kv: unexpect status code")
	// ErrEncoding indicates an error occured while decoding
	ErrEncoding = fmt.Errorf("s3kv: faild to decode content")
)

// Initialize default context
func trigger() {
	once.Do(func() {
		s3 = NewContext("default")
	})
}

// Context of s3kv client
type Context struct {
	name   string
	client *client.Context
}

// NewContext return a new Context of s3kv
//
// Context was initialized from client that named `s3kv_name` in config file
// NewContext will return nil if no client found by name `s3kv_name`
func NewContext(name string) *Context {
	cli := client.NewContext(fmt.Sprintf("s3kv_%s", name))
	if cli == nil {
		return nil
	}
	return &Context{name: name, client: cli}
}

// Has is used to check the existence of a key
func Has(key []byte) (bool, error) {
	trigger()
	return s3.Has(key)
}

// Has is used to check the existence of a key
func (c *Context) Has(key []byte) (bool, error) {
	_, err := assertStatus(http.StatusOK)(
		c.client.R().Head(string(key)),
	)
	switch err {
	case nil:
		return true, nil
	case ErrNotFound:
		return false, nil
	default:
		return false, err
	}
}

// Get value of key
func Get(key []byte) ([]byte, error) {
	trigger()
	return s3.Get(key)
}

// Get value of key
func (c *Context) Get(key []byte) ([]byte, error) {
	resp, err := assertStatus(http.StatusOK)(
		c.client.Get(context.TODO(), string(key)),
	)
	if err != nil {
		return nil, err
	}
	decode, err := snappy.Decode(nil, resp.Body())
	if err != nil {
		err = ErrEncoding
		decode = resp.Body()
	}
	return decode, err
}

// Put key/value
func Put(key []byte, value []byte) error {
	trigger()
	return s3.Put(key, value)
}

// Put key/value
func (c *Context) Put(key []byte, value []byte) error {
	encode := snappy.Encode(nil, value)
	_, err := assertStatus(http.StatusOK)(
		c.client.Put(context.TODO(), encode, string(key)),
	)
	return err
}

// Delete key/value
func Delete(key []byte) error {
	trigger()
	return s3.Delete(key)
}

// Delete key/value
func (c *Context) Delete(key []byte) error {
	_, err := assertStatus(http.StatusNoContent)(
		c.client.R().Delete(string(key)),
	)
	return err
}

// List keys
func List(params map[string]string) (*ListResult, error) {
	trigger()
	return s3.List(params)
}

// List keys
func (c *Context) List(params map[string]string) (*ListResult, error) {
	resp, err := assertStatus(http.StatusOK)(
		c.client.R().SetResult(&ListResult{}).SetQueryParams(params).Get(""),
	)
	if err != nil {
		return nil, err
	}
	return resp.Result().(*ListResult), err
}

// Return an assert function to assert status code
func assertStatus(code int) func(*resty.Response, error) (*resty.Response, error) {
	return func(resp *resty.Response, err error) (*resty.Response, error) {
		if err != nil {
			return resp, ErrNetwork
		}
		switch resp.StatusCode() {
		case code:
			return resp, nil
		case http.StatusForbidden:
			return resp, ErrForbidden
		case http.StatusNotFound:
			return resp, ErrNotFound
		default:
			return resp, ErrStatusCode
		}
	}
}

// ListResult indicates xml of s3 list result
type ListResult struct {
	Name           string         `xml:"Name"`
	Marker         string         `xml:"Marker"`
	Prefix         string         `xml:"Prefix"`
	MaxKeys        string         `xml:"MaxKeys"`
	Delimiter      string         `xml:"Delimiter"`
	NextMarker     string         `xml:"NextMarker"`
	IsTruncated    string         `xml:"IsTruncated"`
	Contents       []Content      `xml:"Contents"`
	CommonPrefixes CommonPrefixes `xml:"CommonPrefixes"`
}

// CommonPrefixes is container of ListResult
type CommonPrefixes struct {
	Prefix []string `xml:"Prefix"`
}

// Content is container of ListResult
type Content struct {
	Key          string `xml:"Key"`
	ETag         string `xml:"ETag"`
	Size         string `xml:"Size"`
	Type         string `xml:"Type"`
	LastModified string `xml:"LastModified"`
	StorageClass string `xml:"StorageClass"`
	Owner        Owner  `xml:"Owner"`
}

// Owner is container of Content
type Owner struct {
	ID          string `xml:"ID"`
	DisplayName string `xml:"DisplayName"`
}
