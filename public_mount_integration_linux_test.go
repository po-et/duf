package main

// This disposable-host validation harness is not part of the upstream patch.
import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPublicMountBoundary(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("GITHUB_REPOSITORY") != "po-et/duf" {
		t.Skip("real mounts are confined to the fork's disposable hosted Linux runner")
	}
	base := t.TempDir()
	mounted := filepath.Join(base, "data")
	sibling := filepath.Join(base, "database")
	for _, dir := range []string{mounted, sibling} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	out, err := exec.Command("sudo", "-n", "mount", "-t", "tmpfs", "-o", "size=1m,mode=1777", "duf-test-fixture", mounted).CombinedOutput()
	if err != nil {
		t.Fatalf("cannot establish isolated 1 MiB fixture mount: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		out, err := exec.Command("sudo", "-n", "umount", mounted).CombinedOutput()
		if err != nil {
			t.Errorf("unmount owned fixture: %v\n%s", err, out)
		}
	})
	for _, dir := range []string{mounted, sibling} {
		if err := os.WriteFile(filepath.Join(dir, "file"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(filepath.Join(sibling, "file"), link); err != nil {
		t.Fatal(err)
	}
	binary, err := filepath.Abs("duf")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{mounted, filepath.Join(mounted, "file"), filepath.Join(sibling, "file"), link} {
		df, err := exec.Command("df", "--output=target", "--", target).CombinedOutput()
		if err != nil {
			t.Fatalf("df oracle for %q: %v\n%s", target, err, df)
		}
		lines := strings.Split(strings.TrimSpace(string(df)), "\n")
		want := strings.TrimSpace(lines[len(lines)-1])
		onMounted := target == mounted || target == filepath.Join(mounted, "file")
		if (want == mounted) != onMounted {
			t.Fatalf("fixture has not established distinct mounts: target %q, df says %q", target, want)
		}
		cmd := exec.Command(binary, "--all", "--output", "mountpoint", "--style", "ascii", "--width", "200", target)
		cmd.Env = append(os.Environ(), "NO_COLOR=1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("public CLI for %q: %v\n%s", target, err, output)
		}
		var got []string
		for _, row := range strings.Split(string(output), "\n") {
			if strings.HasPrefix(row, "|") {
				cells := strings.Split(row, "|")
				cell := strings.TrimSpace(cells[1])
				if strings.HasPrefix(cell, "/") {
					got = append(got, cell)
				}
			}
		}
		t.Logf("target=%q\ndf oracle:\n%sduf table:\n%s", target, df, output)
		if !reflect.DeepEqual(got, []string{want}) {
			t.Errorf("target %q: duf mountpoints %v, df says %q", target, got, want)
		}
	}
}
