// Package service alias.go 实现路径别名的业务逻辑
// 包括创建/更新时的路径展开和清理、名称唯一性校验
package service

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropic/airesourcemanager/internal/model"
	"github.com/anthropic/airesourcemanager/internal/repo"
	"github.com/anthropic/airesourcemanager/internal/util"
)

// AliasService 路径别名业务服务
type AliasService struct {
	repo *repo.AliasRepo
}

// NewAliasService 创建路径别名服务实例
// 参数 repo: 别名数据仓库
// 返回: AliasService 指针
func NewAliasService(repo *repo.AliasRepo) *AliasService {
	return &AliasService{repo: repo}
}

// CreateAlias 创建路径别名
// 参数 req: 创建请求（包含 Name、Type 和 Path）
// 返回: 创建的别名实体、错误信息
// 逻辑: 校验参数 → 检查同类型内名称唯一性 → 展开和清理路径 → 写入数据库
func (s *AliasService) CreateAlias(req *model.CreateAliasReq) (*model.PathAlias, error) {
	// 校验名称
	if strings.TrimSpace(req.Name) == "" {
		return nil, model.NewBizError(model.ErrAliasInvalid, "name 不能为空")
	}
	// 校验类型
	if !isValidType(req.Type) {
		return nil, model.NewBizError(model.ErrAliasInvalid, "type 必须为 skill/agent/config/prompt")
	}
	// 校验路径
	if strings.TrimSpace(req.Path) == "" {
		return nil, model.NewBizError(model.ErrAliasInvalid, "path 不能为空")
	}
	// config 类型的别名 path 必须指向支持格式的配置文件
	if req.Type == "config" && !util.IsConfigFile(req.Path) {
		return nil, model.NewBizError(model.ErrAliasInvalid,
			"Config 别名路径后缀必须是 .json/.jsonc/.yaml/.yml/.toml")
	}
	// prompt 类型的别名 path 必须指向 .md 文件
	if req.Type == "prompt" && !util.IsPromptFile(req.Path) {
		return nil, model.NewBizError(model.ErrAliasInvalid,
			"Prompt 别名路径后缀必须是 .md")
	}

	// 检查同类型内名称唯一性
	if s.repo.CheckAliasNameExists(req.Name, req.Type, "") {
		return nil, model.NewBizError(model.ErrAliasExists, "别名名称已存在")
	}

	// 展开和清理路径
	cleanedPath := expandAndCleanPath(req.Path)

	alias := &model.PathAlias{
		ID:        util.NewUUID(),
		Name:      req.Name,
		Type:      req.Type,
		Path:      cleanedPath,
		CreatedAt: time.Now(),
	}

	if _, err := s.repo.InsertAlias(alias); err != nil {
		return nil, model.NewBizError(model.ErrAliasInvalid, err.Error())
	}

	return alias, nil
}

// ListAliases 按资源类型查询路径别名列表
// 参数 aliasType: 资源类型过滤（skill/agent/mcp）；为空则返回全部
// 返回: 别名列表、错误信息
func (s *AliasService) ListAliases(aliasType string) ([]model.PathAlias, error) {
	return s.repo.ListAliases(aliasType)
}

// UpdateAlias 更新路径别名
// 参数 id: 别名 ID
// 参数 req: 更新请求（包含 Name 和 Path）
// 返回: 错误信息
// 逻辑: 检查别名是否存在 → 校验同类型内名称唯一性（排除自身）→ 展开和清理路径 → 更新数据库
func (s *AliasService) UpdateAlias(id string, req *model.UpdateAliasReq) error {
	// 检查别名是否存在
	existing, err := s.repo.GetAliasByID(id)
	if err != nil {
		return model.NewBizError(model.ErrAliasInvalid, err.Error())
	}
	if existing == nil {
		return model.NewBizError(model.ErrAliasNotFound, "别名不存在")
	}

	// 校验名称
	if strings.TrimSpace(req.Name) == "" {
		return model.NewBizError(model.ErrAliasInvalid, "name 不能为空")
	}
	// 校验路径
	if strings.TrimSpace(req.Path) == "" {
		return model.NewBizError(model.ErrAliasInvalid, "path 不能为空")
	}
	// config 类型的别名 path 后缀校验
	if existing.Type == "config" && !util.IsConfigFile(req.Path) {
		return model.NewBizError(model.ErrAliasInvalid,
			"Config 别名路径后缀必须是 .json/.jsonc/.yaml/.yml/.toml")
	}
	// prompt 类型的别名 path 后缀校验
	if existing.Type == "prompt" && !util.IsPromptFile(req.Path) {
		return model.NewBizError(model.ErrAliasInvalid,
			"Prompt 别名路径后缀必须是 .md")
	}

	// 检查同类型内名称唯一性（排除自身）
	if s.repo.CheckAliasNameExists(req.Name, existing.Type, id) {
		return model.NewBizError(model.ErrAliasExists, "别名名称已存在")
	}

	// 展开和清理路径
	cleanedPath := expandAndCleanPath(req.Path)

	return s.repo.UpdateAlias(id, req.Name, cleanedPath)
}

// DeleteAlias 删除路径别名
// 参数 id: 别名 ID
// 返回: 错误信息
func (s *AliasService) DeleteAlias(id string) error {
	// 检查别名是否存在
	existing, err := s.repo.GetAliasByID(id)
	if err != nil {
		return model.NewBizError(model.ErrAliasInvalid, err.Error())
	}
	if existing == nil {
		return model.NewBizError(model.ErrAliasNotFound, "别名不存在")
	}

	return s.repo.DeleteAlias(id)
}

// expandAndCleanPath 展开并清理路径
// 参数 path: 原始路径字符串
// 返回: 展开 ~ 并经过 filepath.Clean 处理后的路径
// 说明: ~ 展开为 os.UserHomeDir()，展开失败时保留原始 ~
func expandAndCleanPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = home + path[1:]
		}
	}
	return filepath.Clean(path)
}
