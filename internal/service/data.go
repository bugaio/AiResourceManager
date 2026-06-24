// Package service data.go 实现数据导入导出的业务逻辑
// 支持将 ~/.aiManager 下的数据库和资源文件整体导出/导入
package service

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropic/airesourcemanager/internal/model"
	"github.com/anthropic/airesourcemanager/internal/repo"
	"github.com/anthropic/airesourcemanager/internal/util"
	_ "github.com/mattn/go-sqlite3"
)

// DataService 数据导入导出服务
type DataService struct {
	resourceRepo *repo.ResourceRepo
	groupRepo    *repo.GroupRepo
	aliasRepo    *repo.AliasRepo
	baseDir      string // ~/.aiManager 根目录
	dbPath       string // 当前数据库路径
}

// NewDataService 创建数据服务实例
// 参数 resourceRepo: 资源仓库
// 参数 groupRepo: 分组仓库
// 参数 aliasRepo: 别名仓库
// 参数 baseDir: 资源根目录
// 参数 dbPath: 数据库文件路径
// 返回: DataService 指针
func NewDataService(resourceRepo *repo.ResourceRepo, groupRepo *repo.GroupRepo, aliasRepo *repo.AliasRepo, baseDir, dbPath string) *DataService {
	return &DataService{
		resourceRepo: resourceRepo,
		groupRepo:    groupRepo,
		aliasRepo:    aliasRepo,
		baseDir:      baseDir,
		dbPath:       dbPath,
	}
}

// Export 导出数据到指定目录
// 参数 targetPath: 导出目标路径
// 返回: 导出结果、错误信息
func (s *DataService) Export(targetPath string) (*model.ExportResult, error) {
	// 确保目标目录存在
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return nil, model.NewBizError(model.ErrDataWriteFailed, fmt.Sprintf("创建目标目录失败: %v", err))
	}

	var fileCount int
	var totalSize int64

	// 复制数据库文件
	dbTargetDir := filepath.Join(targetPath, "data")
	if err := os.MkdirAll(dbTargetDir, 0755); err != nil {
		return nil, model.NewBizError(model.ErrDataWriteFailed, fmt.Sprintf("创建 data 目录失败: %v", err))
	}
	n, err := copyFile(s.dbPath, filepath.Join(dbTargetDir, "aimanager.db"))
	if err != nil {
		return nil, model.NewBizError(model.ErrDataWriteFailed, fmt.Sprintf("复制数据库失败: %v", err))
	}
	fileCount++
	totalSize += n

	// 复制资源目录: skills/, agents/, configs/, prompts/
	for _, dir := range []string{"skills", "agents", "configs", "prompts"} {
		srcDir := filepath.Join(s.baseDir, dir)
		if !util.IsDir(srcDir) {
			continue
		}
		dstDir := filepath.Join(targetPath, dir)
		count, size, err := copyDirRecursive(srcDir, dstDir)
		if err != nil {
			return nil, model.NewBizError(model.ErrDataWriteFailed, fmt.Sprintf("复制 %s 目录失败: %v", dir, err))
		}
		fileCount += count
		totalSize += size
	}

	// 复制 config.yaml
	configSrc := filepath.Join(s.baseDir, "config.yaml")
	if util.FileExists(configSrc) {
		n, err := copyFile(configSrc, filepath.Join(targetPath, "config.yaml"))
		if err != nil {
			return nil, model.NewBizError(model.ErrDataWriteFailed, fmt.Sprintf("复制 config.yaml 失败: %v", err))
		}
		fileCount++
		totalSize += n
	}

	return &model.ExportResult{
		FileCount: fileCount,
		TotalSize: totalSize,
	}, nil
}

// Import 从指定目录导入数据
// 参数 sourcePath: 导入源路径
// 参数 strategy: 导入策略 (overwrite/skip/keep_both)
// 返回: 导入结果、错误信息
func (s *DataService) Import(sourcePath, strategy string) (*model.ImportResult, error) {
	// 校验策略
	if strategy != "overwrite" && strategy != "skip" && strategy != "keep_both" {
		return nil, model.NewBizError(model.ErrParamValidation, "strategy 必须为 overwrite/skip/keep_both")
	}

	// 校验源路径存在
	if !util.IsDir(sourcePath) {
		return nil, model.NewBizError(model.ErrDataReadFailed, "源路径不存在或不是目录")
	}

	// 验证源目录结构：必须含有 data/aimanager.db
	sourceDBPath := filepath.Join(sourcePath, "data", "aimanager.db")
	if !util.FileExists(sourceDBPath) {
		return nil, model.NewBizError(model.ErrDataReadFailed, "源目录缺少 data/aimanager.db")
	}

	// 打开源数据库（只读）
	srcConn, err := sql.Open("sqlite3", sourceDBPath+"?mode=ro&_journal_mode=WAL")
	if err != nil {
		return nil, model.NewBizError(model.ErrDataReadFailed, fmt.Sprintf("打开源数据库失败: %v", err))
	}
	defer srcConn.Close()

	result := &model.ImportResult{}

	// 导入资源
	if err := s.importResources(srcConn, sourcePath, strategy, result); err != nil {
		return nil, err
	}

	// 导入分组
	if err := s.importGroups(srcConn, strategy, result); err != nil {
		return nil, err
	}

	// 导入别名
	if err := s.importAliases(srcConn, strategy, result); err != nil {
		return nil, err
	}

	return result, nil
}

// importResources 从源数据库导入资源记录和文件
func (s *DataService) importResources(srcConn *sql.DB, sourcePath, strategy string, result *model.ImportResult) error {
	rows, err := srcConn.Query(`SELECT id, name, type, path, description, metadata, created_at, updated_at FROM resource`)
	if err != nil {
		return model.NewBizError(model.ErrDataReadFailed, fmt.Sprintf("读取源资源表失败: %v", err))
	}
	defer rows.Close()

	for rows.Next() {
		var r model.Resource
		if err := rows.Scan(&r.ID, &r.Name, &r.Type, &r.Path, &r.Description, &r.Metadata, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return model.NewBizError(model.ErrDataReadFailed, fmt.Sprintf("扫描资源行失败: %v", err))
		}

		// 检查目标是否已存在
		existing, _ := s.resourceRepo.GetResourceByID(r.ID)

		switch strategy {
		case "skip":
			if existing != nil {
				result.Skipped++
				continue
			}
		case "overwrite":
			if existing != nil {
				// 覆盖：删除旧文件后重新复制
				s.deleteResourceFiles(existing.Type, existing.Path)
			}
		case "keep_both":
			if existing != nil {
				// 生成新 UUID
				r.ID = util.NewUUID()
			}
		}

		// 复制资源文件
		newPath, err := s.copyResourceFiles(r.Type, r.ID, sourcePath, r.Path)
		if err != nil {
			// 文件复制失败跳过该资源
			result.Skipped++
			continue
		}
		r.Path = newPath

		if existing != nil && strategy == "overwrite" {
			// 更新现有记录
			s.resourceRepo.UpdateResource(r.ID, &r.Name, &r.Description)
			result.Overwritten++
		} else {
			// 插入新记录
			r.UpdatedAt = time.Now()
			if err := s.resourceRepo.InsertResource(&r); err != nil {
				result.Skipped++
				continue
			}
			result.Added++
		}
	}
	return nil
}

// importGroups 从源数据库导入分组和关联
func (s *DataService) importGroups(srcConn *sql.DB, strategy string, result *model.ImportResult) error {
	rows, err := srcConn.Query(`SELECT id, name, type, sort_order, created_at, updated_at FROM "group"`)
	if err != nil {
		// 表可能不存在，跳过
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var g model.Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Type, &g.SortOrder, &g.CreatedAt, &g.UpdatedAt); err != nil {
			continue
		}

		existing, _ := s.groupRepo.GetGroupByID(g.ID)
		switch strategy {
		case "skip":
			if existing != nil {
				continue
			}
		case "overwrite":
			if existing != nil {
				s.groupRepo.UpdateGroup(g.ID, &g.Name, &g.SortOrder)
				continue
			}
		case "keep_both":
			if existing != nil {
				g.ID = util.NewUUID()
			}
		}

		if existing == nil || strategy == "keep_both" {
			s.groupRepo.InsertGroup(&g)
		}
	}

	// 导入 group_resource 关联
	grRows, err := srcConn.Query(`SELECT group_id, resource_id FROM group_resource`)
	if err != nil {
		return nil
	}
	defer grRows.Close()

	for grRows.Next() {
		var groupID, resourceID string
		if err := grRows.Scan(&groupID, &resourceID); err != nil {
			continue
		}
		s.groupRepo.AddResourcesToGroup(groupID, []string{resourceID})
	}

	return nil
}

// importAliases 从源数据库导入路径别名
func (s *DataService) importAliases(srcConn *sql.DB, strategy string, result *model.ImportResult) error {
	rows, err := srcConn.Query(`SELECT id, alias, target_path, created_at FROM path_alias`)
	if err != nil {
		// 表可能不存在，跳过
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var a model.PathAlias
		if err := rows.Scan(&a.ID, &a.Name, &a.Path, &a.CreatedAt); err != nil {
			continue
		}

		existing, _ := s.aliasRepo.GetAliasByID(a.ID)
		switch strategy {
		case "skip":
			if existing != nil {
				continue
			}
		case "overwrite":
			if existing != nil {
				s.aliasRepo.UpdateAlias(a.ID, a.Name, a.Path)
				continue
			}
		case "keep_both":
			if existing != nil {
				a.ID = util.NewUUID()
			}
		}

		if existing == nil || strategy == "keep_both" {
			s.aliasRepo.InsertAlias(&a)
		}
	}

	return nil
}

// copyResourceFiles 复制资源文件到本地目录，返回新路径
func (s *DataService) copyResourceFiles(resourceType, uuid, sourcePath, originalPath string) (string, error) {
	switch resourceType {
	case "skill":
		// skills/{uuid}/ 目录
		srcDir := filepath.Join(sourcePath, "skills", filepath.Base(originalPath))
		if !util.IsDir(srcDir) {
			// 尝试用 uuid 查找
			srcDir = filepath.Join(sourcePath, "skills", uuid)
		}
		dstDir := filepath.Join(s.baseDir, "skills", uuid)
		if util.IsDir(srcDir) {
			_, _, err := copyDirRecursive(srcDir, dstDir)
			if err != nil {
				return "", err
			}
		} else {
			// 源目录不存在，创建空目录
			os.MkdirAll(dstDir, 0755)
		}
		return dstDir, nil

	case "agent":
		// agents/{uuid}.md
		srcFile := filepath.Join(sourcePath, "agents", filepath.Base(originalPath))
		if !util.FileExists(srcFile) {
			srcFile = filepath.Join(sourcePath, "agents", uuid+".md")
		}
		dstFile := filepath.Join(s.baseDir, "agents", uuid+".md")
		os.MkdirAll(filepath.Dir(dstFile), 0755)
		if util.FileExists(srcFile) {
			_, err := copyFile(srcFile, dstFile)
			if err != nil {
				return "", err
			}
		}
		return dstFile, nil

	case "config":
		// configs/{uuid}.{ext},ext 沿用原文件名后缀(json/jsonc/yaml/yml/toml)
		srcFile := filepath.Join(sourcePath, "configs", filepath.Base(originalPath))
		if !util.FileExists(srcFile) {
			// 兼容历史:原扩展名为 .jsonc
			srcFile = filepath.Join(sourcePath, "configs", uuid+".jsonc")
		}
		// 目标文件名:沿用源文件后缀,缺失时默认 .jsonc
		ext := filepath.Ext(srcFile)
		if ext == "" {
			ext = ".jsonc"
		}
		dstFile := filepath.Join(s.baseDir, "configs", uuid+ext)
		os.MkdirAll(filepath.Dir(dstFile), 0755)
		if util.FileExists(srcFile) {
			_, err := copyFile(srcFile, dstFile)
			if err != nil {
				return "", err
			}
		}
		return dstFile, nil

	case "prompt":
		// prompts/{uuid}.md
		srcFile := filepath.Join(sourcePath, "prompts", filepath.Base(originalPath))
		if !util.FileExists(srcFile) {
			srcFile = filepath.Join(sourcePath, "prompts", uuid+".md")
		}
		dstFile := filepath.Join(s.baseDir, "prompts", uuid+".md")
		os.MkdirAll(filepath.Dir(dstFile), 0755)
		if util.FileExists(srcFile) {
			_, err := copyFile(srcFile, dstFile)
			if err != nil {
				return "", err
			}
		}
		return dstFile, nil

	default:
		return "", fmt.Errorf("不支持的资源类型: %s", resourceType)
	}
}

// deleteResourceFiles 删除资源文件
func (s *DataService) deleteResourceFiles(resourceType, path string) {
	if path == "" {
		return
	}
	switch resourceType {
	case "skill":
		os.RemoveAll(path)
	default:
		os.Remove(path)
	}
}

// copyFile 复制单个文件，返回文件大小
func copyFile(src, dst string) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return 0, err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer dstFile.Close()

	n, err := io.Copy(dstFile, srcFile)
	return n, err
}

// copyDirRecursive 递归复制目录，返回文件数和总大小
func copyDirRecursive(src, dst string) (int, int64, error) {
	var fileCount int
	var totalSize int64

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 计算相对路径
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		// 跳过符号链接
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		n, err := copyFile(path, dstPath)
		if err != nil {
			return err
		}
		fileCount++
		totalSize += n
		return nil
	})

	return fileCount, totalSize, err
}

// extractUUIDFromPath 从文件路径提取资源 UUID
// skills/{uuid}/... → uuid
// agents/{uuid}.md → uuid
// configs/{uuid}.{ext} → uuid
func extractUUIDFromPath(path, baseDir string) string {
	rel, err := filepath.Rel(baseDir, path)
	if err != nil {
		return ""
	}

	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) < 2 {
		return ""
	}

	switch parts[0] {
	case "skills":
		// skills/{uuid}/...
		return parts[1]
	case "agents":
		// agents/{uuid}.md
		name := parts[1]
		return strings.TrimSuffix(name, ".md")
	case "configs":
		// configs/{uuid}.{ext}, ext 可为 .jsonc/.json/.yaml/.yml/.toml
		name := parts[1]
		if ext := filepath.Ext(name); ext != "" {
			return strings.TrimSuffix(name, ext)
		}
		return name
	case "prompts":
		// prompts/{uuid}.md
		name := parts[1]
		return strings.TrimSuffix(name, ".md")
	case "presets":
		// presets/{presetID}/{uuid}[/...] (skill 目录) 或 presets/{presetID}/{uuid}.ext (agent/config/prompt 文件)
		if len(parts) < 3 {
			return ""
		}
		name := parts[2]
		if ext := filepath.Ext(name); ext != "" {
			return strings.TrimSuffix(name, ext)
		}
		return name
	default:
		return ""
	}
}
