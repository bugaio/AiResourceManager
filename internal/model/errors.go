// Package model errors.go 定义业务错误码和错误类型
// 错误码分段：1000-1999 资源，2000-2999 分组，3000-3999 部署，4000-4999 别名，
// 5000-5999 数据，1100-1199 Preset，1200-1299 PathGroup，9000-9999 系统
package model

import "fmt"

// 资源相关错误码 (1000-1999)
const (
	ErrResourceNotFound       = 1001 // 资源不存在
	ErrResourceExists         = 1002 // 资源名称重复
	ErrResourceFileIO         = 1003 // 文件读写错误
	ErrResourceHasDeploy      = 1004 // 存在关联部署，需确认删除
	ErrResourceLockedByPreset = 1005 // 资源被 preset 管理，需 unlink=true 才能删除
	ErrPrivateResourceCross   = 1106 // 私有资源不能被其他 preset 引用
)

// Preset 相关错误码 (1100-1199)
const (
	ErrPresetNotFound      = 1101 // preset 不存在
	ErrPresetDuplicateName = 1102 // preset 名称重复
	ErrPresetInvalid       = 1103 // preset 参数无效
)

// PathGroup 相关错误码 (1200-1299)
const (
	ErrPathGroupNotFound      = 1201 // path_group 不存在
	ErrPathGroupDuplicateName = 1202 // path_group 名称重复
	ErrPathGroupEmpty         = 1203 // 4 个子路径全空
	ErrPathGroupBadFormat     = 1204 // 子路径格式不符合 type 约束
)

// 参数校验错误码
const (
	ErrParamValidation = 9001 // 参数校验失败
)

// 分组相关错误码 (2000-2999)
const (
	ErrGroupNotFound = 2001 // 分组不存在
	ErrGroupExists   = 2002 // 分组已存在
	ErrGroupInvalid  = 2003 // 分组参数无效
)

// 部署相关错误码 (3000-3999)
const (
	ErrDeployNotFound  = 3001 // 部署任务不存在
	ErrDeployFailed    = 3002 // 部署执行失败
	ErrDeployInvalid   = 3003 // 部署参数无效
	ErrDeployRunning   = 3004 // 部署正在执行中
)

// 别名相关错误码 (4000-4999)
const (
	ErrAliasNotFound  = 4001 // 别名不存在
	ErrAliasExists    = 4002 // 别名已存在
	ErrAliasInvalid   = 4003 // 别名参数无效
)

// 数据相关错误码 (5000-5999)
const (
	ErrDataReadFailed  = 5001 // 数据读取失败
	ErrDataWriteFailed = 5002 // 数据写入失败
	ErrDataCorrupted   = 5003 // 数据损坏
)

// 系统相关错误码 (9000-9999)
const (
	ErrSystemInternal = 9001 // 系统内部错误
	ErrSystemConfig   = 9002 // 配置错误
	ErrSystemDB       = 9003 // 数据库错误
	ErrSystemIO       = 9004 // IO 操作错误
)

// BizError 业务错误类型，包含错误码和错误消息
type BizError struct {
	Code int         // 业务错误码
	Msg  string      // 错误描述
	Data interface{} // 可选附加数据（冲突详情等）
}

// Error 实现 error 接口
func (e *BizError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Msg)
}

// NewBizError 创建业务错误
// 参数 code: 业务错误码
// 参数 msg: 错误描述
// 返回: BizError 指针
func NewBizError(code int, msg string) *BizError {
	return &BizError{Code: code, Msg: msg}
}

// NewBizErrorWithData 创建带附加数据的业务错误
func NewBizErrorWithData(code int, msg string, data interface{}) *BizError {
	return &BizError{Code: code, Msg: msg, Data: data}
}
