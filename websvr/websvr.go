package websvr

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/miinowy/go-base"
	"github.com/spf13/viper"
)

var (
	websvrMap sync.Map
	websvrMtx sync.Mutex
)

// Context provide web server
// Handler will generate by handlerFunc every time on reload
type Context struct {
	*base.TaskBase

	name   string
	listen string

	mtx sync.Mutex
	svr *http.Server

	handlerFunc func() http.Handler
}

// NewWebsvr return a new Task instance of websvr
func NewWebsvr(handlerFunc func() http.Handler, name string) (*base.Task, error) {
	if value, ok := websvrMap.Load(name); ok == true {
		return value.(*base.Task), nil
	}

	websvrMtx.Lock()
	defer websvrMtx.Unlock()
	if value, ok := websvrMap.Load(name); ok == true {
		return value.(*base.Task), nil
	}

	c := &Context{name: name, handlerFunc: handlerFunc}
	websvrMap.Store(name, c)
	return base.NewTaskOnReload(c, fmt.Sprintf("websvr/%s", name))
}

// Reload is used to reload websvr context
func (c *Context) Reload(ctx context.Context) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	listen := viper.GetString(fmt.Sprintf("websvr.%s.listen", c.name))

	// diff from pre listen
	if c.listen != listen {
		// stop pre listen
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		c.Retire(ctx)

		c.listen = listen
		if c.listen == "" {
			return nil
		}

		c.Log.Debugf("Web server will listen: %s", c.listen)
		c.svr = &http.Server{
			Addr: c.listen,
		}

		go func() {
			c.Log.Debug("Web server is starting")
			if err := c.svr.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				c.Log.WithFields(map[string]interface{}{"listen": c.listen}).
					WithError(err).Fatal("An error occurred while ListenAndServe")
				select {}
			}
		}()
	}

	if c.svr != nil {
		c.svr.Handler = c.handlerFunc()
	}

	return nil
}

// Retire is used to retire websvr context
func (c *Context) Retire(ctx context.Context) (err error) {
	if c.svr != nil {
		c.Log.Debug("Web server is stopping")
		err = c.svr.Shutdown(ctx)
		c.svr = nil
	}
	return err
}

// Schedule is used to schedule websvr context
func (c *Context) Schedule(ctx context.Context) error {
	return nil
}
