package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompletionCmd_Bash(t *testing.T) {
	root := NewRootCmd()
	cmd := newCompletionCmd(root)

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"bash"}); err != nil {
		t.Fatalf("completion bash: %v", err)
	}
	if !strings.Contains(out.String(), "bash") {
		t.Error("expected bash completion output")
	}
}

func TestCompletionCmd_Zsh(t *testing.T) {
	root := NewRootCmd()
	cmd := newCompletionCmd(root)

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"zsh"}); err != nil {
		t.Fatalf("completion zsh: %v", err)
	}
	if !strings.Contains(out.String(), "zsh") {
		t.Error("expected zsh completion output")
	}
}

func TestCompletionCmd_Fish(t *testing.T) {
	root := NewRootCmd()
	cmd := newCompletionCmd(root)

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"fish"}); err != nil {
		t.Fatalf("completion fish: %v", err)
	}
	if !strings.Contains(out.String(), "fish") {
		t.Error("expected fish completion output")
	}
}

func TestCompletionCmd_PowerShell(t *testing.T) {
	root := NewRootCmd()
	cmd := newCompletionCmd(root)

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"powershell"}); err != nil {
		t.Fatalf("completion powershell: %v", err)
	}
	if !strings.Contains(out.String(), "powershell") && !strings.Contains(out.String(), "PowerShell") {
		t.Error("expected powershell completion output")
	}
}

func TestCompletionCmd_InvalidShell(t *testing.T) {
	root := NewRootCmd()
	cmd := newCompletionCmd(root)

	err := cmd.RunE(cmd, []string{"invalid"})
	if err != nil {
		t.Errorf("expected no error for invalid shell (shows usage), got: %v", err)
	}
}

func TestVersionCmd_Default(t *testing.T) {
	orig := appVersion
	defer func() { appVersion = orig }()
	appVersion = "v1.2.3"

	cmd := newVersionCmd()

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("version: %v", err)
	}
	if !strings.Contains(out.String(), "kleido version v1.2.3") {
		t.Errorf("expected version output, got: %q", out.String())
	}
}

func TestVersionCmd_Short(t *testing.T) {
	orig := appVersion
	defer func() { appVersion = orig }()
	appVersion = "v2.0.0-beta"

	cmd := newVersionCmd()
	cmd.Flags().Set("short", "true")

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("version --short: %v", err)
	}
	if !strings.Contains(out.String(), "v2.0.0-beta") {
		t.Errorf("expected short version output, got: %q", out.String())
	}
}
