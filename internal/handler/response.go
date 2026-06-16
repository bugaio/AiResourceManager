// Package handler response.go 定义统一的 API 响应结构和健康检查处理器
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response API 统一响应结构
// code: 业务状态码，0 表示成功
// msg: 响应消息
// data: 响应数据，可为 null
type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// HealthHandler 健康检查处理器
type HealthHandler struct{}

// NewHealthHandler 创建健康检查处理器实例
// 返回: HealthHandler 指针
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Health 健康检查接口，返回服务运行状态
// 响应: {code:0, msg:"ok", data:null}
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, Response{
		Code: 0,
		Msg:  "ok",
		Data: nil,
	})
}

// Success 返回成功响应的快捷方法
// 参数 c: gin 上下文
// 参数 data: 响应数据
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: 0,
		Msg:  "ok",
		Data: data,
	})
}

// Error 返回错误响应的快捷方法
// 参数 c: gin 上下文
// 参数 code: 业务错误码
// 参数 msg: 错误消息
func Error(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, Response{
		Code: code,
		Msg:  msg,
		Data: nil,
	})
}

// ErrorWithData 返回带数据的错误响应
// 参数 c: gin 上下文
// 参数 code: 业务错误码
// 参数 msg: 错误消息
// 参数 data: 附加数据（如关联部署信息）
func ErrorWithData(c *gin.Context, code int, msg string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: code,
		Msg:  msg,
		Data: data,
	})
}
