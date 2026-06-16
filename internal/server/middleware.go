// Package server middleware.go 提供 HTTP 中间件
package server

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CORSMiddleware 跨域中间件，允许前端开发时跨域访问
// 返回: gin.HandlerFunc 中间件函数
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// LoggerMiddleware 请求日志中间件，记录每个请求的方法、路径和耗时
// 参数 logger: zap 日志实例
// 返回: gin.HandlerFunc 中间件函数
func LoggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		logger.Info("请求完成",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("latency", latency),
		)
	}
}
