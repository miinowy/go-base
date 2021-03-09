package webhelper

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// GinMiddleLogrus middleware use logrus to log requests
func GinMiddleLogrus() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		c.Next()

		costTime := time.Now().Sub(startTime)

		logrus.WithFields(logrus.Fields{
			"cost":   costTime,
			"client": c.ClientIP(),
			"status": c.Writer.Status(),
			"method": c.Request.Method,
			"uri":    c.Request.RequestURI,
		}).Info("HTTP request details")
	}
}

// GinHandlerRouterList return a gin handler that show all route supported
func GinHandlerRouterList(handler *gin.Engine) func(*gin.Context) {
	var routes []string
	return func(c *gin.Context) {
		if len(routes) == 0 {
			var routesMap []struct{ path, method string }
			for _, route := range handler.Routes() {
				routesMap = append(routesMap, struct{ path, method string }{
					path:   route.Path,
					method: route.Method,
				})
			}
			sort.Slice(routesMap, func(i, j int) bool {
				if routesMap[i].path != routesMap[j].path {
					return routesMap[i].path < routesMap[j].path
				}
				if len(routesMap[i].method) != len(routesMap[j].method) {
					return len(routesMap[i].method) < len(routesMap[j].method)
				}
				return routesMap[i].method < routesMap[j].method
			})
			for _, item := range routesMap {
				routes = append(routes, fmt.Sprintf("%s %s", item.method, item.path))
			}
		}
		buf, _ := json.MarshalIndent(routes, "", "  ")
		c.String(200, string(buf))
	}
}
