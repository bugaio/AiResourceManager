package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropic/airesourcemanager/internal/model"
	"github.com/anthropic/airesourcemanager/internal/repo"
	"github.com/anthropic/airesourcemanager/internal/util"
)

// setupDataTest 建一个临时库 + 临时 baseDir，跑迁移，返回三个 repo 与 baseDir
func setupDataTest(t *testing.T) (*repo.ResourceRepo, *repo.GroupRepo, *repo.PresetRepo, string) {
	t.Helper()
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	db, err := repo.NewDB(dbPath)
	if err != nil {
		t.Fatalf("建库失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := repo.RunMigrations(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	baseDir := filepath.Join(tmp, "aiManager")
	return repo.NewResourceRepo(db), repo.NewGroupRepo(db), repo.NewPresetRepo(db), baseDir
}

// TestExportImportRoundTrip 验证导出→导入到一个全新仓库后，
// 资源/分组/preset 及其关联与私有资源全部还原。
func TestExportImportRoundTrip(t *testing.T) {
	// --- 源仓库 ---
	srcRes, srcGrp, srcPre, srcBase := setupDataTest(t)
	srcSvc := NewDataService(srcRes, srcGrp, srcPre, srcBase)

	// 全局 skill 资源 + 文件
	skillDir := filepath.Join(srcBase, "skills", "u-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# hello skill"), 0644)
	srcRes.InsertResource(&model.Resource{ID: "u-skill", Name: "我的技能", Type: "skill", Path: skillDir, Metadata: "{}"})

	// 全局 config 资源 + 文件
	cfgPath := filepath.Join(srcBase, "configs", "u-config.jsonc")
	os.MkdirAll(filepath.Dir(cfgPath), 0755)
	os.WriteFile(cfgPath, []byte(`{"a":1}`), 0644)
	srcRes.InsertResource(&model.Resource{ID: "u-config", Name: "我的配置", Type: "config", Path: cfgPath, Metadata: "{}"})

	// 分组 + 关联
	srcGrp.InsertGroup(&model.Group{ID: "g1", Name: "组A", Type: "skill"})
	srcGrp.AddResourcesToGroup("g1", []string{"u-skill"})

	// preset + 关联 u-config + 一个私有 agent
	srcPre.InsertPreset(&model.Preset{ID: "p1", Name: "预设一"})
	srcPre.LinkResources("p1", []string{"u-config"})
	privPath := filepath.Join(srcBase, "presets", "p1", "agents", "u-priv.md")
	os.MkdirAll(filepath.Dir(privPath), 0755)
	os.WriteFile(privPath, []byte("# private agent"), 0644)
	pid := "p1"
	srcRes.InsertResource(&model.Resource{ID: "u-priv", Name: "私有助手", Type: "agent", Path: privPath, Metadata: "{}", OwnerPresetID: &pid})

	// --- 导出 ---
	exportDir := filepath.Join(t.TempDir(), "export")
	exp, err := srcSvc.Export(exportDir, false)
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	if exp.ResourceCount != 3 || exp.GroupCount != 1 || exp.PresetCount != 1 {
		t.Fatalf("导出统计错误: %+v", exp)
	}
	if !util.FileExists(filepath.Join(exportDir, "data.json")) {
		t.Fatal("缺少 data.json")
	}
	if !util.FileExists(filepath.Join(exportDir, "files", "skills", "u-skill", "SKILL.md")) {
		t.Fatal("skill 文件未导出")
	}
	if !util.FileExists(filepath.Join(exportDir, "files", "presets", "p1", "agents", "u-priv.md")) {
		t.Fatal("私有 agent 文件未导出")
	}

	// --- 导入到全新空仓库 ---
	dstRes, dstGrp, dstPre, dstBase := setupDataTest(t)
	dstSvc := NewDataService(dstRes, dstGrp, dstPre, dstBase)
	imp, err := dstSvc.Import(exportDir, "keep_both")
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	if imp.Added != 4 {
		t.Fatalf("应新增 3 资源 + 1 preset = 4，实际: %+v", imp)
	}

	// 校验资源还原 + 文件落地
	r, _ := dstRes.GetResourceByID("u-skill")
	if r == nil || r.Name != "我的技能" {
		t.Fatal("skill 资源未还原")
	}
	if !util.FileExists(filepath.Join(r.Path, "SKILL.md")) {
		t.Fatalf("skill 文件未落地: %s", r.Path)
	}

	// 校验私有资源归属还原
	priv, _ := dstRes.GetResourceByID("u-priv")
	if priv == nil || priv.OwnerPresetID == nil || *priv.OwnerPresetID != "p1" {
		t.Fatalf("私有资源归属未还原: %+v", priv)
	}
	if !util.FileExists(priv.Path) {
		t.Fatalf("私有 agent 文件未落地: %s", priv.Path)
	}

	// 校验分组关联还原
	gids, _ := dstGrp.GetGroupResources("g1")
	if len(gids) != 1 || gids[0] != "u-skill" {
		t.Fatalf("分组关联未还原: %v", gids)
	}

	// 校验 preset 关联还原
	pres, _ := dstPre.ListPresetResources("p1")
	if len(pres) != 1 || pres[0] != "u-config" {
		t.Fatalf("preset 关联未还原: %v", pres)
	}
}

// TestImportSkipExisting 验证 skip 策略下命中已有资源时跳过
func TestImportSkipExisting(t *testing.T) {
	srcRes, srcGrp, srcPre, srcBase := setupDataTest(t)
	srcSvc := NewDataService(srcRes, srcGrp, srcPre, srcBase)
	p := filepath.Join(srcBase, "prompts", "u-p.md")
	os.MkdirAll(filepath.Dir(p), 0755)
	os.WriteFile(p, []byte("hi"), 0644)
	srcRes.InsertResource(&model.Resource{ID: "u-p", Name: "提示", Type: "prompt", Path: p, Metadata: "{}"})

	exportDir := filepath.Join(t.TempDir(), "export")
	if _, err := srcSvc.Export(exportDir, false); err != nil {
		t.Fatal(err)
	}

	// 导入回同一仓库(u-p 已存在) → skip
	imp, err := srcSvc.Import(exportDir, "skip")
	if err != nil {
		t.Fatal(err)
	}
	if imp.Skipped != 1 || imp.Added != 0 {
		t.Fatalf("skip 策略错误: %+v", imp)
	}
}
