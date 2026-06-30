// Package service data.go 实现数据导入导出的业务逻辑
//
// 导出/导入是一对可逆操作，产物为 git 友好的展开目录：
//
//	{path}/
//	├── manifest.json   — 格式版本 / 导出时间 / 数量统计
//	├── data.json       — 全部关系数据(资源/分组/preset 及其关联)
//	└── files/          — 实体文件，按 ~/.aiManager 下的相对路径镜像存放
//
// 只同步「资源实体 + 分组 + preset 关联(含私有资源)」，
// 不导出任何部署信息(deployment / deployment_item / path_group / path_alias)。
// 导入后所有资源回到本地仓库，用户可重新建立部署。
package service

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropic/airesourcemanager/internal/model"
	"github.com/anthropic/airesourcemanager/internal/repo"
	"github.com/anthropic/airesourcemanager/internal/util"
)

// exportFormatVersion 导出格式版本，导入时据此校验兼容性
const exportFormatVersion = 1

// DataService 数据导入导出服务
type DataService struct {
	resourceRepo *repo.ResourceRepo
	groupRepo    *repo.GroupRepo
	presetRepo   *repo.PresetRepo
	baseDir      string // ~/.aiManager 根目录
}

// NewDataService 创建数据服务实例
// 参数 resourceRepo: 资源仓库
// 参数 groupRepo: 分组仓库
// 参数 presetRepo: preset 仓库
// 参数 baseDir: 资源根目录(~/.aiManager)
func NewDataService(resourceRepo *repo.ResourceRepo, groupRepo *repo.GroupRepo, presetRepo *repo.PresetRepo, baseDir string) *DataService {
	return &DataService{
		resourceRepo: resourceRepo,
		groupRepo:    groupRepo,
		presetRepo:   presetRepo,
		baseDir:      baseDir,
	}
}

// ============================ 归档结构 ============================

// manifest manifest.json 结构
type manifest struct {
	FormatVersion int    `json:"format_version"`
	ExportedAt    string `json:"exported_at"`
	ResourceCount int    `json:"resource_count"`
	GroupCount    int    `json:"group_count"`
	PresetCount   int    `json:"preset_count"`
	FileCount     int    `json:"file_count"`
}

// bundle data.json 结构：全部关系数据
type bundle struct {
	Resources       []bundleResource           `json:"resources"`
	Groups          []bundleGroup              `json:"groups"`
	Presets         []bundlePreset             `json:"presets"`
	GroupResources  []model.GroupResourceLink  `json:"group_resources"`
	PresetResources []model.PresetResourceLink `json:"preset_resources"`
}

// bundleResource 导出的资源记录。path 不导出(机器相关)，改用 rel_path(相对仓库布局)
type bundleResource struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Description   string  `json:"description"`
	Metadata      string  `json:"metadata"`
	OwnerPresetID *string `json:"owner_preset_id"`
	RelPath       string  `json:"rel_path"` // 相对 baseDir 的实体文件/目录路径
}

// bundleGroup 导出的分组记录(去掉运行时统计字段)
type bundleGroup struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
}

// bundlePreset 导出的 preset 记录(去掉运行时统计字段)
type bundlePreset struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ============================ 导出 ============================

// Export 导出数据到指定目录
// 参数 targetPath: 导出目标目录(用户可对其 git init 实现跨机器同步)
// 参数 clear: 目标目录含非隐藏文件时，true=先清除再导出，false=报 ErrDataDirNotEmpty 让用户确认
// 返回: 导出结果、错误信息
func (s *DataService) Export(targetPath string, clear bool) (*model.ExportResult, error) {
	// 目标已存在且含非隐藏内容时，依据 clear 决定清除或拦截
	if util.IsDir(targetPath) {
		visible, err := visibleEntries(targetPath)
		if err != nil {
			return nil, model.NewBizError(model.ErrDataReadFailed, fmt.Sprintf("读取目标目录失败: %v", err))
		}
		if len(visible) > 0 {
			if !clear {
				return nil, model.NewBizError(model.ErrDataDirNotEmpty, "目标目录不为空")
			}
			// 清除非隐藏文件/目录(保留 .git 等隐藏项)
			for _, name := range visible {
				if err := os.RemoveAll(filepath.Join(targetPath, name)); err != nil {
					return nil, model.NewBizError(model.ErrDataWriteFailed, fmt.Sprintf("清除目标目录失败: %v", err))
				}
			}
		}
	}

	filesDir := filepath.Join(targetPath, "files")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		return nil, model.NewBizError(model.ErrDataWriteFailed, fmt.Sprintf("创建导出目录失败: %v", err))
	}

	b := &bundle{
		Resources:       []bundleResource{},
		Groups:          []bundleGroup{},
		Presets:         []bundlePreset{},
		GroupResources:  []model.GroupResourceLink{},
		PresetResources: []model.PresetResourceLink{},
	}

	var fileCount int
	var totalSize int64

	// 1. 资源(全局 + 私有)。ownerPresetID="" 表示不按归属过滤，返回全部
	resources, _, err := s.resourceRepo.ListResources("", "", "0", "", 1, 1<<30)
	if err != nil {
		return nil, model.NewBizError(model.ErrDataReadFailed, fmt.Sprintf("查询资源失败: %v", err))
	}
	for _, r := range resources {
		relPath, rerr := filepath.Rel(s.baseDir, r.Path)
		if rerr != nil || strings.HasPrefix(relPath, "..") {
			// 资源文件在仓库目录之外，跳过(异常数据，不应出现)
			continue
		}
		relPath = filepath.ToSlash(relPath)

		// 复制实体文件/目录到 files/{relPath}
		cnt, size, cerr := s.copyEntityOut(r.Type, r.Path, filepath.Join(filesDir, relPath))
		if cerr != nil {
			return nil, model.NewBizError(model.ErrDataWriteFailed, fmt.Sprintf("复制资源 %s 失败: %v", r.Name, cerr))
		}
		fileCount += cnt
		totalSize += size

		b.Resources = append(b.Resources, bundleResource{
			ID:            r.ID,
			Name:          r.Name,
			Type:          r.Type,
			Description:   r.Description,
			Metadata:      r.Metadata,
			OwnerPresetID: r.OwnerPresetID,
			RelPath:       relPath,
		})
	}

	// 2. 分组 + group_resource 关联
	groups, _, err := s.groupRepo.ListGroups("", 1, 1<<30)
	if err != nil {
		return nil, model.NewBizError(model.ErrDataReadFailed, fmt.Sprintf("查询分组失败: %v", err))
	}
	for _, g := range groups {
		b.Groups = append(b.Groups, bundleGroup{
			ID:        g.ID,
			Name:      g.Name,
			Type:      g.Type,
			Color:     g.Color,
			SortOrder: g.SortOrder,
		})
		rids, gerr := s.groupRepo.GetGroupResources(g.ID)
		if gerr != nil {
			return nil, model.NewBizError(model.ErrDataReadFailed, fmt.Sprintf("查询分组资源失败: %v", gerr))
		}
		for _, rid := range rids {
			b.GroupResources = append(b.GroupResources, model.GroupResourceLink{GroupID: g.ID, ResourceID: rid})
		}
	}

	// 3. preset + preset_resource 关联
	presets, err := s.presetRepo.ListPresets()
	if err != nil {
		return nil, model.NewBizError(model.ErrDataReadFailed, fmt.Sprintf("查询 preset 失败: %v", err))
	}
	for _, p := range presets {
		b.Presets = append(b.Presets, bundlePreset{ID: p.ID, Name: p.Name, Description: p.Description})
		rids, perr := s.presetRepo.ListPresetResources(p.ID)
		if perr != nil {
			return nil, model.NewBizError(model.ErrDataReadFailed, fmt.Sprintf("查询 preset 关联失败: %v", perr))
		}
		for _, rid := range rids {
			b.PresetResources = append(b.PresetResources, model.PresetResourceLink{PresetID: p.ID, ResourceID: rid})
		}
	}

	// 写 data.json
	if err := writeJSON(filepath.Join(targetPath, "data.json"), b); err != nil {
		return nil, model.NewBizError(model.ErrDataWriteFailed, fmt.Sprintf("写入 data.json 失败: %v", err))
	}

	// 写 manifest.json
	mf := &manifest{
		FormatVersion: exportFormatVersion,
		ExportedAt:    time.Now().Format(time.RFC3339),
		ResourceCount: len(b.Resources),
		GroupCount:    len(b.Groups),
		PresetCount:   len(b.Presets),
		FileCount:     fileCount,
	}
	if err := writeJSON(filepath.Join(targetPath, "manifest.json"), mf); err != nil {
		return nil, model.NewBizError(model.ErrDataWriteFailed, fmt.Sprintf("写入 manifest.json 失败: %v", err))
	}

	return &model.ExportResult{
		ResourceCount: len(b.Resources),
		GroupCount:    len(b.Groups),
		PresetCount:   len(b.Presets),
		FileCount:     fileCount,
		TotalSize:     totalSize,
	}, nil
}

// ============================ 导入 ============================

// Import 从指定目录导入数据
// 参数 sourcePath: 导入源目录(含 data.json / files/)
// 参数 strategy: 冲突策略 skip / overwrite / keep_both
// 返回: 导入结果、错误信息
func (s *DataService) Import(sourcePath, strategy string) (*model.ImportResult, error) {
	if strategy != "overwrite" && strategy != "skip" && strategy != "keep_both" {
		return nil, model.NewBizError(model.ErrParamValidation, "strategy 必须为 overwrite/skip/keep_both")
	}
	if !util.IsDir(sourcePath) {
		return nil, model.NewBizError(model.ErrDataReadFailed, "源路径不存在或不是目录")
	}

	// 解析 data.json
	var b bundle
	if err := readJSON(filepath.Join(sourcePath, "data.json"), &b); err != nil {
		return nil, model.NewBizError(model.ErrDataReadFailed, fmt.Sprintf("读取 data.json 失败: %v", err))
	}
	filesDir := filepath.Join(sourcePath, "files")

	result := &model.ImportResult{}
	presetIDMap := map[string]string{}   // 旧 preset id → 新 preset id
	resourceIDMap := map[string]string{} // 旧 resource id → 新 resource id
	groupIDMap := map[string]string{}    // 旧 group id → 新 group id

	// 1. 先导入 preset(资源的 owner_preset_id 需引用其新 id)
	for _, p := range b.Presets {
		newID, err := s.importPreset(p, strategy, result)
		if err != nil {
			return nil, err
		}
		presetIDMap[p.ID] = newID // newID 为空表示 skip(沿用旧 id 即可)
		if newID == "" {
			presetIDMap[p.ID] = p.ID
		}
	}

	// 2. 导入资源(全局 + 私有)
	for _, r := range b.Resources {
		newID, err := s.importResource(r, filesDir, strategy, presetIDMap, result)
		if err != nil {
			return nil, err
		}
		resourceIDMap[r.ID] = newID
		if newID == "" {
			resourceIDMap[r.ID] = r.ID
		}
	}

	// 3. 导入分组
	for _, g := range b.Groups {
		newID, err := s.importGroup(g, strategy)
		if err != nil {
			return nil, err
		}
		groupIDMap[g.ID] = newID
		if newID == "" {
			groupIDMap[g.ID] = g.ID
		}
	}

	// 4. 重建 group_resource 关联(用映射后的新 id)
	for _, link := range b.GroupResources {
		gid, ok1 := groupIDMap[link.GroupID]
		rid, ok2 := resourceIDMap[link.ResourceID]
		if !ok1 || !ok2 || gid == "" || rid == "" {
			continue
		}
		_ = s.groupRepo.AddResourcesToGroup(gid, []string{rid})
	}

	// 5. 重建 preset_resource 关联
	for _, link := range b.PresetResources {
		pid, ok1 := presetIDMap[link.PresetID]
		rid, ok2 := resourceIDMap[link.ResourceID]
		if !ok1 || !ok2 || pid == "" || rid == "" {
			continue
		}
		_ = s.presetRepo.LinkResources(pid, []string{rid})
	}

	return result, nil
}

// importPreset 导入单个 preset，返回写入后使用的 id
// 返回空串表示 skip(命中已有且策略为 skip，调用方沿用旧 id)
func (s *DataService) importPreset(p bundlePreset, strategy string, result *model.ImportResult) (string, error) {
	existing, _ := s.presetRepo.GetPresetByID(p.ID)

	if existing != nil {
		switch strategy {
		case "skip":
			result.Skipped++
			return "", nil
		case "overwrite":
			_ = s.presetRepo.UpdatePreset(p.ID, &p.Name, &p.Description)
			result.Overwritten++
			return p.ID, nil
		case "keep_both":
			p.ID = util.NewUUID()
			result.Renamed++
		}
	}

	// preset.name 有 UNIQUE 约束，重名必须改名
	name, err := s.uniquePresetName(p.Name)
	if err != nil {
		return "", err
	}
	np := &model.Preset{ID: p.ID, Name: name, Description: p.Description, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := s.presetRepo.InsertPreset(np); err != nil {
		return "", model.NewBizError(model.ErrSystemDB, fmt.Sprintf("插入 preset 失败: %v", err))
	}
	if existing == nil {
		result.Added++
	}
	return p.ID, nil
}

// importResource 导入单个资源(含实体文件)，返回写入后使用的 id
func (s *DataService) importResource(r bundleResource, filesDir, strategy string, presetIDMap map[string]string, result *model.ImportResult) (string, error) {
	// 重映射私有资源归属 preset
	var ownerPresetID *string
	if r.OwnerPresetID != nil && *r.OwnerPresetID != "" {
		mapped := presetIDMap[*r.OwnerPresetID]
		if mapped == "" {
			mapped = *r.OwnerPresetID
		}
		ownerPresetID = &mapped
	}

	srcEntity := filepath.Join(filesDir, filepath.FromSlash(r.RelPath))
	existing, _ := s.resourceRepo.GetResourceByID(r.ID)

	if existing != nil {
		switch strategy {
		case "skip":
			result.Skipped++
			return "", nil
		case "overwrite":
			// 原地覆盖实体文件，更新名称/描述
			if err := s.overwriteEntity(existing.Type, existing.Path, srcEntity); err != nil {
				return "", model.NewBizError(model.ErrDataWriteFailed, fmt.Sprintf("覆盖资源 %s 文件失败: %v", r.Name, err))
			}
			_ = s.resourceRepo.UpdateResource(r.ID, &r.Name, &r.Description)
			result.Overwritten++
			return r.ID, nil
		case "keep_both":
			r.ID = util.NewUUID()
			result.Renamed++
		}
	}

	// 新增：计算目标路径 → 复制文件 → 插入记录
	name := s.uniqueResourceName(r.Type, r.Name)
	ext := filepath.Ext(r.RelPath)
	dstPath := s.destPathFor(r.Type, r.ID, ownerPresetID, ext)

	if err := s.copyEntityIn(r.Type, srcEntity, dstPath); err != nil {
		return "", model.NewBizError(model.ErrDataWriteFailed, fmt.Sprintf("写入资源 %s 文件失败: %v", r.Name, err))
	}

	now := time.Now()
	meta := r.Metadata
	if meta == "" {
		meta = "{}"
	}
	nr := &model.Resource{
		ID: r.ID, Name: name, Type: r.Type, Path: dstPath,
		Description: r.Description, Metadata: meta,
		CreatedAt: now, UpdatedAt: now, OwnerPresetID: ownerPresetID,
	}
	if err := s.resourceRepo.InsertResource(nr); err != nil {
		_ = util.RemoveFileOrDir(dstPath)
		return "", model.NewBizError(model.ErrSystemDB, fmt.Sprintf("插入资源失败: %v", err))
	}
	if existing == nil {
		result.Added++
	}
	return r.ID, nil
}

// importGroup 导入单个分组，返回写入后使用的 id
func (s *DataService) importGroup(g bundleGroup, strategy string) (string, error) {
	existing, _ := s.groupRepo.GetGroupByID(g.ID)
	if existing != nil {
		switch strategy {
		case "skip":
			return "", nil
		case "overwrite":
			_ = s.groupRepo.UpdateGroup(g.ID, &g.Name, &g.SortOrder)
			return g.ID, nil
		case "keep_both":
			g.ID = util.NewUUID()
		}
	}
	name := s.uniqueGroupName(g.Name, g.Type)
	ng := &model.Group{
		ID: g.ID, Name: name, Type: g.Type, Color: g.Color,
		SortOrder: g.SortOrder, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := s.groupRepo.InsertGroup(ng); err != nil {
		return "", model.NewBizError(model.ErrSystemDB, fmt.Sprintf("插入分组失败: %v", err))
	}
	return g.ID, nil
}

// ============================ 路径 / 命名辅助 ============================

// destPathFor 计算导入资源在本地仓库的目标路径
// 参数 ext: config 类型沿用源文件扩展名(.jsonc/.json/.yaml...)，其余类型忽略
func (s *DataService) destPathFor(typ, uuid string, ownerPresetID *string, ext string) string {
	sub := typeSubdir(typ)
	root := s.baseDir
	if ownerPresetID != nil && *ownerPresetID != "" {
		root = filepath.Join(s.baseDir, "presets", *ownerPresetID)
	}
	dir := filepath.Join(root, sub)
	switch typ {
	case "skill":
		return filepath.Join(dir, uuid) // 目录
	case "config":
		if ext == "" {
			ext = ".jsonc"
		}
		return filepath.Join(dir, uuid+ext)
	default: // agent / prompt
		return filepath.Join(dir, uuid+".md")
	}
}

// uniqueResourceName 在同类型下生成不冲突的资源名(应用层唯一约束)
func (s *DataService) uniqueResourceName(typ, name string) string {
	candidate := name
	for i := 1; ; i++ {
		exists, _ := s.resourceRepo.CheckNameExists(typ, candidate, "")
		if !exists {
			return candidate
		}
		candidate = fmt.Sprintf("%s (%d)", name, i)
	}
}

// uniquePresetName 生成不冲突的 preset 名(DB UNIQUE 约束)
func (s *DataService) uniquePresetName(name string) (string, error) {
	candidate := name
	for i := 1; ; i++ {
		exists, err := s.presetRepo.CheckPresetNameExists(candidate, "")
		if err != nil {
			return "", model.NewBizError(model.ErrSystemDB, err.Error())
		}
		if !exists {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s (%d)", name, i)
	}
}

// uniqueGroupName 在同类型下生成不冲突的分组名(应用层唯一约束)
func (s *DataService) uniqueGroupName(name, typ string) string {
	candidate := name
	for i := 1; ; i++ {
		exists, _ := s.groupRepo.CheckGroupNameExists(candidate, typ, "")
		if !exists {
			return candidate
		}
		candidate = fmt.Sprintf("%s (%d)", name, i)
	}
}

// ============================ 文件复制 ============================

// copyEntityOut 导出时把实体文件/目录复制到归档，返回文件数与总大小
func (s *DataService) copyEntityOut(typ, src, dst string) (int, int64, error) {
	if !util.FileExists(src) {
		return 0, 0, nil // 源缺失静默跳过(记录仍写入 data.json)
	}
	if typ == "skill" {
		return copyDirRecursive(src, dst)
	}
	n, err := copyFile(src, dst)
	if err != nil {
		return 0, 0, err
	}
	return 1, n, nil
}

// copyEntityIn 导入时把归档中的实体复制到本地仓库目标位置
func (s *DataService) copyEntityIn(typ, src, dst string) error {
	if !util.FileExists(src) {
		// 源缺失：建空目录/空文件以保证记录与文件一致
		if typ == "skill" {
			return os.MkdirAll(dst, 0755)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		return os.WriteFile(dst, []byte{}, 0644)
	}
	if typ == "skill" {
		_, _, err := copyDirRecursive(src, dst)
		return err
	}
	_, err := copyFile(src, dst)
	return err
}

// overwriteEntity 覆盖导入：原地替换已有资源的实体文件
func (s *DataService) overwriteEntity(typ, existingPath, src string) error {
	if typ == "skill" {
		_ = os.RemoveAll(existingPath)
		return s.copyEntityIn(typ, src, existingPath)
	}
	return s.copyEntityIn(typ, src, existingPath)
}

// visibleEntries 返回目录下的非隐藏顶层条目名(忽略以 . 开头的，如 .git)
func visibleEntries(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		names = append(names, e.Name())
	}
	return names, nil
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

// copyDirRecursive 递归复制目录，返回文件数和总大小(跳过符号链接)
func copyDirRecursive(src, dst string) (int, int64, error) {
	var fileCount int
	var totalSize int64

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}
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

// ============================ JSON 读写 ============================

// writeJSON 以缩进格式写入 JSON 文件(便于 git diff)
func writeJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// readJSON 读取并解析 JSON 文件
func readJSON(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// extractUUIDFromPath 从文件路径提取资源 UUID(文件监听器据此定位资源)
// skills/{uuid}/...           → uuid
// agents/{uuid}.md            → uuid
// configs/{uuid}.{ext}        → uuid
// prompts/{uuid}.md           → uuid
// presets/{presetID}/{uuid}.. → uuid
func extractUUIDFromPath(path, baseDir string) string {
	rel, err := filepath.Rel(baseDir, path)
	if err != nil {
		return ""
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) < 2 {
		return ""
	}
	trimExt := func(name string) string {
		if ext := filepath.Ext(name); ext != "" {
			return strings.TrimSuffix(name, ext)
		}
		return name
	}
	switch parts[0] {
	case "skills":
		return parts[1]
	case "agents", "configs", "prompts":
		return trimExt(parts[1])
	case "presets":
		// presets/{presetID}/{uuid}[/...] 或 presets/{presetID}/{uuid}.ext
		if len(parts) < 3 {
			return ""
		}
		return trimExt(parts[2])
	default:
		return ""
	}
}
