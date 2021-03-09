package websvr

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/miinowy/go-base/websvr/helper"
)

func TestWebsvr(t *testing.T) {
	NewWebsvr(handlerFunc, "default")
	select {}
}

func handlerFunc() http.Handler {
	gin.SetMode("release")

	handler := gin.New()
	handler.Use(gin.Recovery())
	handler.Use(webhelper.GinMiddleLogrus())

	handler.GET("/", webhelper.GinHandlerRouterList(handler))
	handler.PUT("/", webhelper.GinHandlerRouterList(handler))
	handler.POST("/", webhelper.GinHandlerRouterList(handler))
	handler.GET("/hello", func(c *gin.Context) {
		c.String(200, "hello")
	})

	return handler
}
