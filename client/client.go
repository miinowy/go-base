// Package client - Http Client
package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/spf13/viper"

	"github.com/miinowy/go-base"
)

var (
	mtx       sync.Mutex
	clientMap sync.Map

	// ErrArgument indicates a argument error
	ErrArgument = fmt.Errorf("client: unexcept argument")
	// ErrConfigure indicates a config error
	ErrConfigure = fmt.Errorf("client: unexcept configure")
)

// Context of client
type Context struct {
	*base.TaskBase

	name   string
	path   string
	query  url.Values
	header http.Header
	client *resty.Client
}

// NewContext return a new Context of http client
func NewContext(name string) *Context {
	if name == "" {
		return nil
	}

	// Cache
	if value, ok := clientMap.Load(name); ok == true {
		return value.(*Context)
	}

	// Double cache
	mtx.Lock()
	defer mtx.Unlock()
	if value, ok := clientMap.Load(name); ok == true {
		return value.(*Context)
	}

	c := &Context{name: name}
	clientMap.Store(name, c)
	base.NewTaskOnReload(c, fmt.Sprintf("client/%s", name))

	return c
}

// Reload to get lastest config by config file
func (c *Context) Reload(ctx context.Context) error {
	if c.name == "" {
		return ErrArgument
	}

	// Get config from config file
	config := viper.GetStringMap(fmt.Sprintf("http_client.%s", c.name))

	_cli := resty.New()
	_path := "/"
	_query := url.Values{}
	_header := http.Header{}

	// Set host
	if host, ok := config["host"].(string); ok == true {
		if u, err := url.Parse(host); err == nil {
			_cli = _cli.SetHostURL(fmt.Sprintf("%s://%s", u.Scheme, u.Host))
			_path = u.Path
		}
	} else {
		return ErrConfigure
	}

	// Set timeout
	if timeout, ok := config["timeout"].(string); ok == true {
		if du, err := time.ParseDuration(timeout); err == nil {
			_cli = _cli.SetTimeout(du)
		}
	}

	// Set header
	header, ok := config["header"].(map[string]interface{})
	if ok == true {
		for k, v := range header {
			switch v.(type) {
			case []interface{}:
				for _, x := range v.([]interface{}) {
					_header.Set(k, fmt.Sprintf("%v", x))
				}
			default:
				_header.Set(k, fmt.Sprintf("%v", v))
			}
		}
	}

	// Set query
	query, ok := config["query"].(map[string]interface{})
	if ok == true {
		for k, v := range query {
			switch v.(type) {
			case []interface{}:
				for _, x := range v.([]interface{}) {
					_query.Set(k, fmt.Sprintf("%v", x))
				}
			default:
				_query.Set(k, fmt.Sprintf("%v", v))
			}
		}
	}

	c.client = _cli
	c.path = _path
	c.query = _query
	c.header = _header

	return nil
}

// Retire was execute when exit
func (c *Context) Retire(ctx context.Context) error {
	return nil
}

// Schedule was execute when schedule
func (c *Context) Schedule(ctx context.Context) error {
	return nil
}

// R return a *resty.Request
func (c *Context) R() *resty.Request {
	request := c.client.R()
	request.Header = c.header
	request.QueryParam = c.query
	return request
}

// Get resource by http request
func (c *Context) Get(ctx context.Context, args ...string) (*resty.Response, error) {
	if len(args) == 0 {
		c.Log.Trace("Http client get request")
		return c.R().SetContext(ctx).Get(c.path)
	}
	c.Log.WithFields(map[string]interface{}{"uri": args}).Trace("Http client get request")
	return c.R().SetContext(ctx).Get(filepath.Join(args...))
}

// Put resource by http request
func (c *Context) Put(ctx context.Context, body interface{}, args ...string) (*resty.Response, error) {
	if len(args) == 0 {
		c.Log.WithFields(map[string]interface{}{"content": body}).Trace("Http client put request")
		return c.R().SetContext(ctx).SetBody(body).Put(c.path)
	}
	c.Log.WithFields(map[string]interface{}{"uri": args, "content": body}).Trace("Http client put request")
	return c.R().SetContext(ctx).SetBody(body).Put(filepath.Join(args...))
}

// Post resource by http request
func (c *Context) Post(ctx context.Context, body interface{}, args ...string) (*resty.Response, error) {
	if len(args) == 0 {
		c.Log.WithFields(map[string]interface{}{"content": body}).Trace("Http client post request")
		return c.R().SetContext(ctx).SetBody(body).Post(c.path)
	}
	c.Log.WithFields(map[string]interface{}{"uri": args, "content": body}).Trace("Http client post request")
	return c.R().SetContext(ctx).SetBody(body).Post(filepath.Join(args...))
}
