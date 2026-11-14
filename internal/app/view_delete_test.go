package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"local-database-manager/internal/config"
	"local-database-manager/internal/core"

	tea "github.com/charmbracelet/bubbletea"
)

func TestActionMenuIncludesDeleteInstance(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	inst := &core.DatabaseInstance{Name: "demo", EngineType: "postgres", Runtime: "docker", Status: core.StatusStopped}
	items := m.getActionMenuItems(inst)
	found := false
	for _, it := range items {
		if it.label == "Delete Instance" && it.shortcut == "D" {
			found = true
			if !strings.Contains(it.description, ".env") {
				t.Fatalf("description=%q", it.description)
			}
		}
	}
	if !found {
		t.Fatal("Delete Instance missing after Purge")
	}
}

func TestKeyDArmsConfirmDelete(t *testing.T) {
	t.Parallel()
	m := NewApp("/tmp", config.Config{EngineHealthInterval: time.Second})
	m.instances = []*core.DatabaseInstance{{Name: "demo", EngineType: "postgres", Runtime: "docker"}}
	m.selectedIndex = 0
	m.mode = ModeMain
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	if !m.confirmDelete {
		t.Fatal("expected confirmDelete")
	}
	if !strings.Contains(m.statusMsg, "removes the .env") {
		t.Fatalf("status=%q", m.statusMsg)
	}
	if m.confirmPurge {
		t.Fatal("purge must not be armed")
	}
}

func TestDeleteInstanceRemovesEnvWhenOnline(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	envPath := filepath.Join(dir, "demo.env")
	if err := os.WriteFile(envPath, []byte("ENGINE=postgres\nRUNTIME=docker\n"), 0644); err != nil {
		t.Fatal(err)
	}
	stub := &stubRunner{docker: core.EngineOnline}
	m := NewApp(dir, config.Config{EngineHealthInterval: time.Second})
	m.instancesDir = dir
	m.runner = stub
	inst := &core.DatabaseInstance{Name: "demo", Runtime: "docker", EnvFilePath: envPath}
	msg := m.deleteInstanceCmd(inst)()
	if _, ok := msg.(errMsg); ok {
		t.Fatalf("unexpected err: %#v", msg)
	}
	if stub.downVolumesCalls != 1 {
		t.Fatalf("DownVolumes calls=%d", stub.downVolumesCalls)
	}
	if _, err := os.Stat(envPath); !os.IsNotExist(err) {
		t.Fatal("expected .env removed")
	}
}

func TestDeleteInstanceRefusesWhenNotInstalled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	envPath := filepath.Join(dir, "demo.env")
	_ = os.WriteFile(envPath, []byte("ENGINE=postgres\nRUNTIME=docker\n"), 0644)
	stub := &stubRunner{docker: core.EngineNotInstalled}
	m := NewApp(dir, config.Config{EngineHealthInterval: time.Second})
	m.runner = stub
	inst := &core.DatabaseInstance{Name: "demo", Runtime: "docker", EnvFilePath: envPath}
	msg := m.deleteInstanceCmd(inst)()
	em, ok := msg.(errMsg)
	if !ok {
		t.Fatalf("got %T", msg)
	}
	if !errors.Is(em.err, core.ErrEngineNotInstalled) {
		t.Fatalf("want ErrEngineNotInstalled, got %v", em.err)
	}
	if stub.downVolumesCalls != 0 {
		t.Fatal("must not purge when engine not installed")
	}
	if _, err := os.Stat(envPath); err != nil {
		t.Fatal("env must remain")
	}
}

func TestDeleteInstanceRefusesWhenOffline(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	envPath := filepath.Join(dir, "demo.env")
	_ = os.WriteFile(envPath, []byte("ENGINE=postgres\nRUNTIME=docker\n"), 0644)
	stub := &stubRunner{docker: core.EngineOffline}
	m := NewApp(dir, config.Config{EngineHealthInterval: time.Second})
	m.runner = stub
	inst := &core.DatabaseInstance{Name: "demo", Runtime: "docker", EnvFilePath: envPath}
	msg := m.deleteInstanceCmd(inst)()
	em, ok := msg.(errMsg)
	if !ok {
		t.Fatalf("got %T", msg)
	}
	if !errors.Is(em.err, core.ErrEngineOffline) {
		t.Fatalf("want ErrEngineOffline, got %v", em.err)
	}
	if stub.downVolumesCalls != 0 {
		t.Fatal("must not purge when offline")
	}
	if _, err := os.Stat(envPath); err != nil {
		t.Fatal("env must remain")
	}
}

func TestDeleteInstanceKeepsEnvWhenPurgeFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	envPath := filepath.Join(dir, "demo.env")
	_ = os.WriteFile(envPath, []byte("ENGINE=postgres\nRUNTIME=docker\n"), 0644)
	stub := &stubRunner{docker: core.EngineOnline, downVolumesErr: errors.New("boom")}
	m := NewApp(dir, config.Config{EngineHealthInterval: time.Second})
	m.runner = stub
	inst := &core.DatabaseInstance{Name: "demo", Runtime: "docker", EnvFilePath: envPath}
	msg := m.deleteInstanceCmd(inst)()
	if _, ok := msg.(errMsg); !ok {
		t.Fatalf("got %T, want errMsg", msg)
	}
	if _, err := os.Stat(envPath); err != nil {
		t.Fatal("env must remain after purge failure")
	}
}

func TestDeleteConfirmYTriggersReload(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	envPath := filepath.Join(dir, "demo.env")
	if err := os.WriteFile(envPath, []byte("ENGINE=postgres\nRUNTIME=docker\n"), 0644); err != nil {
		t.Fatal(err)
	}
	stub := &stubRunner{docker: core.EngineOnline}
	m := NewApp(dir, config.Config{EngineHealthInterval: time.Second})
	m.instancesDir = dir
	m.runner = stub
	m.instances = []*core.DatabaseInstance{{Name: "demo", Runtime: "docker", EnvFilePath: envPath}}
	m.selectedIndex = 0
	m.mode = ModeMain

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	am := updated.(*AppModel)
	if !am.confirmDelete {
		t.Fatal("expected confirmDelete armed")
	}

	updated, cmd := am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	am = updated.(*AppModel)
	if am.confirmDelete {
		t.Fatal("confirmDelete should be cleared on confirm")
	}
	if cmd == nil {
		t.Fatal("expected delete cmd")
	}
	done, ok := cmd().(deleteDoneMsg)
	if !ok {
		t.Fatalf("got %T, want deleteDoneMsg", cmd())
	}
	if done.err != nil {
		t.Fatalf("unexpected delete error: %v", done.err)
	}
	if _, err := os.Stat(envPath); !os.IsNotExist(err) {
		t.Fatal("expected .env removed by delete cmd")
	}

	updated, reloadCmd := am.Update(done)
	am = updated.(*AppModel)
	if reloadCmd == nil {
		t.Fatal("expected reload cmd after delete success")
	}
	batch, ok := reloadCmd().(tea.BatchMsg)
	if !ok || len(batch) == 0 {
		t.Fatalf("reload cmd produced %T, want tea.BatchMsg with reload", reloadCmd())
	}
	loaded, ok := batch[0]().(instancesLoadedMsg)
	if !ok {
		t.Fatalf("reload produced %T, want instancesLoadedMsg", batch[0]())
	}
	if len(loaded.instances) != 0 {
		t.Fatalf("expected empty instance list after delete, got %d", len(loaded.instances))
	}
	if !strings.Contains(am.statusMsg, "deleted") {
		t.Fatalf("status=%q", am.statusMsg)
	}
}

func TestPurgeStillKeepsEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	envPath := filepath.Join(dir, "demo.env")
	_ = os.WriteFile(envPath, []byte("ENGINE=postgres\nRUNTIME=docker\n"), 0644)
	stub := &stubRunner{docker: core.EngineOnline}
	m := NewApp(dir, config.Config{EngineHealthInterval: time.Second})
	m.runner = stub
	inst := &core.DatabaseInstance{Name: "demo", Runtime: "docker", EnvFilePath: envPath}
	msg := m.purgeInstanceCmd(inst)()
	if _, ok := msg.(errMsg); ok {
		t.Fatalf("purge err: %#v", msg)
	}
	if _, err := os.Stat(envPath); err != nil {
		t.Fatal("purge must keep .env")
	}
}
