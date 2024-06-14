package cli

import (
	"os"
	"os/exec"
	"path"
	"testing"
)

// go test -v -run TestFirstupPRTemplate
func TestFirstupPRTemplate(t *testing.T) {

	testcases := map[string]struct {
		want string
		prep func(t *testing.T, dir string)
	}{
		"uses repo template": {
			want: "repo template",
			prep: func(t *testing.T, dir string) {
				initRepo(t, dir)
				templatePath := path.Join(dir, ".github", "PULL_REQUEST_TEMPLATE")
				os.MkdirAll(templatePath, 0755)
				templateFile := path.Join(templatePath, "PULL_REQUEST_TEMPLATE.md")
				if err := os.WriteFile(templateFile, []byte("repo template"), 0644); err != nil {
					t.Fatal(err)
				}

				if err := exec.Command("git", "checkout", "-b", "FE-1234-test-branch").Run(); err != nil {
					t.Fatal(err)
				}
			},
		},
		"uses default template": {
			want: defaultPRTemplate,
			prep: func(t *testing.T, dir string) {
				initRepo(t, dir)
				if err := exec.Command("git", "checkout", "-b", "test-branch").Run(); err != nil {
					t.Fatal(err)
				}
			},
		},
		"inserts ticket in templates that have a JIRA URL": {
			want: "https://firstup-io.atlassian.net/browse/FE-1234",
			prep: func(t *testing.T, dir string) {
				initRepo(t, dir)
				templatePath := path.Join(dir, ".github", "PULL_REQUEST_TEMPLATE")
				os.MkdirAll(templatePath, 0755)
				templateFile := path.Join(templatePath, "PULL_REQUEST_TEMPLATE.md")
				if err := os.WriteFile(templateFile, []byte("https://firstup-io.atlassian.net/browse/FE-"), 0644); err != nil {
					t.Fatal(err)
				}

				if err := exec.Command("git", "checkout", "-b", "FE-1234-test-branch").Run(); err != nil {
					t.Fatal(err)
				}
			},
		},
		"no ticket in branch name": {
			want: "# PR Title",
			prep: func(t *testing.T, dir string) {
				initRepo(t, dir)
				templatePath := path.Join(dir, ".github", "PULL_REQUEST_TEMPLATE")
				os.MkdirAll(templatePath, 0755)
				templateFile := path.Join(templatePath, "PULL_REQUEST_TEMPLATE.md")
				if err := os.WriteFile(templateFile, []byte("# PR Title"), 0644); err != nil {
					t.Fatal(err)
				}

				if err := exec.Command("git", "checkout", "-b", "test-branch").Run(); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tc.prep(t, tmpDir)
			got, err := firstupPRTemplate()
			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}

			if got != tc.want {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}

}

func TestIsWorkstationDir(t *testing.T) {
	testcases := map[string]struct {
		want bool
		dir  string
	}{
		"socialchorus dir": {
			want: true,
			dir:  "/opt/socialchorus/optimus",
		},
		"firstup dir": {
			want: true,
			dir:  "/opt/firstup/pythia",
		},
		"not a workstation dir": {
			want: false,
			dir:  "~/dev/mercury",
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			got := isWorkstationDir(tc.dir)
			if got != tc.want {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func initRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	init := exec.Command("git", "init")
	if err := init.Run(); err != nil {
		t.Fatal("error running git init:", err)
	}

}
