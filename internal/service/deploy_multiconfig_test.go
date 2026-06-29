package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropic/airesourcemanager/internal/model"
	"github.com/anthropic/airesourcemanager/internal/repo"
	"github.com/anthropic/airesourcemanager/internal/util"
)

// setupDeploySvc 建临时 DB + repo + DeployService（含 preset/pathGroup 注入）
func setupDeploySvc(t *testing.T) (*DeployService, *repo.PresetRepo, *repo.PathGroupRepo, *repo.ResourceRepo, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := repo.NewDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("建库失败: %v", err)
	}
	if err := repo.RunMigrations(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	deployRepo := repo.NewDeployRepo(db)
	resourceRepo := repo.NewResourceRepo(db)
	aliasRepo := repo.NewAliasRepo(db)
	groupRepo := repo.NewGroupRepo(db)
	presetRepo := repo.NewPresetRepo(db)
	pathGroupRepo := repo.NewPathGroupRepo(db)

	svc := NewDeployService(deployRepo, resourceRepo, aliasRepo, groupRepo, dir)
	svc.SetPresetRepo(presetRepo)
	svc.SetPathGroupRepo(pathGroupRepo)
	return svc, presetRepo, pathGroupRepo, resourceRepo, dir
}

// createConfigResource 在 storage 写一个 config 片段文件并落库为私有资源
func createConfigResource(t *testing.T, rr *repo.ResourceRepo, dir, presetID, name, content string) *model.Resource {
	t.Helper()
	cfgDir := filepath.Join(dir, "configs")
	_ = os.MkdirAll(cfgDir, 0755)
	id := util.NewUUID()
	p := filepath.Join(cfgDir, id+".json")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("写 config 文件失败: %v", err)
	}
	r := &model.Resource{ID: id, Name: name, Type: "config", Path: p, OwnerPresetID: &presetID}
	if err := rr.InsertResource(r); err != nil {
		t.Fatalf("落库 config 失败: %v", err)
	}
	return r
}

// TestDeployPreset_MultiConfigPaths 多个 config 按分配部署到不同目标文件
func TestDeployPreset_MultiConfigPaths(t *testing.T) {
	svc, presetRepo, pathGroupRepo, resourceRepo, dir := setupDeploySvc(t)

	// preset
	presetID := util.NewUUID()
	if err := presetRepo.InsertPreset(&model.Preset{ID: presetID, Name: "p1"}); err != nil {
		t.Fatalf("建 preset 失败: %v", err)
	}

	// 两个 config，键不冲突
	c1 := createConfigResource(t, resourceRepo, dir, presetID, "claude-cfg", `{"alpha":1}`)
	c2 := createConfigResource(t, resourceRepo, dir, presetID, "cursor-cfg", `{"beta":2}`)

	// 两个目标文件
	targetA := filepath.Join(dir, "A.json")
	targetB := filepath.Join(dir, "B.json")
	_ = os.WriteFile(targetA, []byte("{}"), 0644)
	_ = os.WriteFile(targetB, []byte("{}"), 0644)

	// 路径组：两条 config 路径
	pgID := util.NewUUID()
	if err := pathGroupRepo.InsertPathGroup(&model.PathGroup{
		ID: pgID, Name: "g1", ConfigPaths: []string{targetA, targetB},
	}); err != nil {
		t.Fatalf("建路径组失败: %v", err)
	}

	// 部署：c1→A, c2→B
	_, err := svc.DeployPreset(&model.DeployPresetReq{
		PathGroupID:       &pgID,
		ConfigAssignments: map[string]string{c1.ID: targetA, c2.ID: targetB},
	}, presetID)
	if err != nil {
		t.Fatalf("DeployPreset 失败: %v", err)
	}

	// 校验：A 含 alpha 不含 beta；B 反之
	a, _ := os.ReadFile(targetA)
	b, _ := os.ReadFile(targetB)
	as, bs := string(a), string(b)
	if !contains(as, "alpha") || contains(as, "beta") {
		t.Errorf("A.json 内容错误: %s", as)
	}
	if !contains(bs, "beta") || contains(bs, "alpha") {
		t.Errorf("B.json 内容错误: %s", bs)
	}

	// 校验：生成两条独立 deployment，target 各异
	deps, _ := svc.deployRepo.ListDeploymentsByPreset(presetID)
	if len(deps) != 2 {
		t.Fatalf("应有 2 条 deployment，实际 %d", len(deps))
	}
	paths := map[string]bool{}
	for _, d := range deps {
		paths[d.TargetPath] = true
	}
	if !paths[targetA] || !paths[targetB] {
		t.Errorf("deployment target 路径不符: %v", paths)
	}
}

// TestDeployPreset_MissingAssignment 多 config 路径但未分配 → 报错
func TestDeployPreset_MissingAssignment(t *testing.T) {
	svc, presetRepo, pathGroupRepo, resourceRepo, dir := setupDeploySvc(t)
	presetID := util.NewUUID()
	_ = presetRepo.InsertPreset(&model.Preset{ID: presetID, Name: "p2"})
	createConfigResource(t, resourceRepo, dir, presetID, "c", `{"x":1}`)
	tA := filepath.Join(dir, "A.json")
	tB := filepath.Join(dir, "B.json")
	_ = os.WriteFile(tA, []byte("{}"), 0644)
	_ = os.WriteFile(tB, []byte("{}"), 0644)
	pgID := util.NewUUID()
	_ = pathGroupRepo.InsertPathGroup(&model.PathGroup{ID: pgID, Name: "g2", ConfigPaths: []string{tA, tB}})

	_, err := svc.DeployPreset(&model.DeployPresetReq{PathGroupID: &pgID}, presetID)
	if err == nil {
		t.Fatal("多 config 路径未分配应报错，但成功了")
	}
}

// TestDeployPreset_SingleConfigAutoAssign 单条 config 路径自动归一，无需分配
func TestDeployPreset_SingleConfigAutoAssign(t *testing.T) {
	svc, presetRepo, pathGroupRepo, resourceRepo, dir := setupDeploySvc(t)
	presetID := util.NewUUID()
	_ = presetRepo.InsertPreset(&model.Preset{ID: presetID, Name: "p3"})
	createConfigResource(t, resourceRepo, dir, presetID, "c1", `{"a":1}`)
	createConfigResource(t, resourceRepo, dir, presetID, "c2", `{"b":2}`)
	target := filepath.Join(dir, "only.json")
	_ = os.WriteFile(target, []byte("{}"), 0644)
	pgID := util.NewUUID()
	_ = pathGroupRepo.InsertPathGroup(&model.PathGroup{ID: pgID, Name: "g3", ConfigPaths: []string{target}})

	_, err := svc.DeployPreset(&model.DeployPresetReq{PathGroupID: &pgID}, presetID)
	if err != nil {
		t.Fatalf("单 config 路径自动分配应成功: %v", err)
	}
	data, _ := os.ReadFile(target)
	s := string(data)
	if !contains(s, "a") || !contains(s, "b") {
		t.Errorf("两个 config 应都合并进 only.json: %s", s)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestSummarizeAndUndeployAtPaths 删 config 路径前能查到内容、确认后能移除
func TestSummarizeAndUndeployAtPaths(t *testing.T) {
	svc, presetRepo, pathGroupRepo, resourceRepo, dir := setupDeploySvc(t)
	presetID := util.NewUUID()
	_ = presetRepo.InsertPreset(&model.Preset{ID: presetID, Name: "ps"})
	c1 := createConfigResource(t, resourceRepo, dir, presetID, "c1", `{"a":1}`)
	c2 := createConfigResource(t, resourceRepo, dir, presetID, "c2", `{"b":2}`)
	tA := filepath.Join(dir, "A.json")
	tB := filepath.Join(dir, "B.json")
	_ = os.WriteFile(tA, []byte("{}"), 0644)
	_ = os.WriteFile(tB, []byte("{}"), 0644)
	pgID := util.NewUUID()
	_ = pathGroupRepo.InsertPathGroup(&model.PathGroup{ID: pgID, Name: "gs", ConfigPaths: []string{tA, tB}})
	if _, err := svc.DeployPreset(&model.DeployPresetReq{
		PathGroupID:       &pgID,
		ConfigAssignments: map[string]string{c1.ID: tA, c2.ID: tB},
	}, presetID); err != nil {
		t.Fatalf("部署失败: %v", err)
	}

	// 删 B 前：summarize 应报告 B 有内容
	sum, err := svc.SummarizeDeploymentsAtPaths([]string{tB})
	if err != nil {
		t.Fatalf("summarize 失败: %v", err)
	}
	if len(sum[tB]) == 0 {
		t.Fatalf("B 路径应有部署内容，实际为空")
	}

	// 撤销 B
	n, err := svc.UndeployAtPaths([]string{tB})
	if err != nil || n == 0 {
		t.Fatalf("撤销失败: n=%d err=%v", n, err)
	}
	// 撤销后 B 文件里不应还有 b 键
	data, _ := os.ReadFile(tB)
	if contains(string(data), "\"b\"") {
		t.Errorf("撤销后 B 仍含 b 键: %s", string(data))
	}
	// A 不受影响
	deps, _ := svc.deployRepo.ListDeploymentsByPreset(presetID)
	if len(deps) != 1 {
		t.Errorf("应只剩 A 一条部署，实际 %d", len(deps))
	}
}
