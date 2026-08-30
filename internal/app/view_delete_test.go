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
