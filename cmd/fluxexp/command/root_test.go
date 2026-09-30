package command

import (
	"bytes"
	"strings"
	"testing"
)

// The --version flag is cobra's, added only when Version is non-empty. These
// tests drive it through Execute rather than inspecting the flag set, because
// cobra registers the flag during execution and not at construction.
func TestNewRootCmd_VersionFlag(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantOut string
		wantErr string
	}{
		{
			name:    "reports the version it was built with",
			version: "1.2.3",
			wantOut: "1.2.3",
		},
		{
			name:    "a prerelease version is reported verbatim",
			version: "1.5.0-rc1",
			wantOut: "1.5.0-rc1",
		},
		{
			name:    "no version means no flag",
			version: "",
			wantErr: "unknown flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCmd(tt.version)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"--version"})

			err := cmd.Execute()

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected an error containing %q, got none (output %q)", tt.wantErr, out.String())
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(out.String(), tt.wantOut) {
				t.Fatalf("output = %q, want it to contain %q", out.String(), tt.wantOut)
			}
		})
	}
}

func TestNewRootCmd_VersionIsCarried(t *testing.T) {
	if got := NewRootCmd("4.5.6").Version; got != "4.5.6" {
		t.Fatalf("Version = %q, want 4.5.6", got)
	}
}
