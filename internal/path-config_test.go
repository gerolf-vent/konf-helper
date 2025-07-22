package internal

import (
	"strings"
	"testing"
)

func TestParsePathConfig(t *testing.T) {
	tests := []struct {
		name       string
		spec       string
		wantErr    bool
		wantErrMsg string
		want       *PathConfig
	}{
		{
			name:    "only srcPath",
			spec:    "/etc/config",
			wantErr: false,
			want:    &PathConfig{srcPath: "/etc/config"},
		},
		{
			name:    "srcPath and fileGlob",
			spec:    "/etc/config:*.conf",
			wantErr: false,
			want:    &PathConfig{srcPath: "/etc/config", fileGlob: "*.conf"},
		},
		{
			name:    "srcPath, fileGlob, dstPath",
			spec:    "/etc/config:*.conf:/tmp/config",
			wantErr: false,
			want:    &PathConfig{srcPath: "/etc/config", fileGlob: "*.conf", dstPath: "/tmp/config"},
		},
		{
			name:    "srcPath, fileGlob, dstPath, owner",
			spec:    "/etc/config:*.conf:/tmp/config:1000",
			wantErr: false,
			want:    &PathConfig{srcPath: "/etc/config", fileGlob: "*.conf", dstPath: "/tmp/config", owner: 1000},
		},
		{
			name:    "srcPath, fileGlob, dstPath, owner, group",
			spec:    "/etc/config:*.conf:/tmp/config:1000:1001",
			wantErr: false,
			want:    &PathConfig{srcPath: "/etc/config", fileGlob: "*.conf", dstPath: "/tmp/config", owner: 1000, group: 1001},
		},
		{
			name:    "srcPath, fileGlob, dstPath, owner, group, mode",
			spec:    "/etc/config:*.conf:/tmp/config:1000:1001:644",
			wantErr: false,
			want:    &PathConfig{srcPath: "/etc/config", fileGlob: "*.conf", dstPath: "/tmp/config", owner: 1000, group: 1001, mode: 0644},
		},
		{
			name:       "invalid srcPath (not absolute)",
			spec:       "etc/config",
			wantErr:    true,
			wantErrMsg: "source path must be absolute: etc/config",
		},
		{
			name:       "invalid fileGlob",
			spec:       "/etc/config:[",
			wantErr:    true,
			wantErrMsg: "invalid file glob \"[\": syntax error in pattern",
		},
		{
			name:       "invalid dstPath (not absolute)",
			spec:       "/etc/config:*.conf:tmp/config",
			wantErr:    true,
			wantErrMsg: "destination path must be absolute: tmp/config",
		},
		{
			name:       "invalid owner",
			spec:       "/etc/config:*.conf:/tmp/config:notanowner",
			wantErr:    true,
			wantErrMsg: "invalid owner \"notanowner\"",
		},
		{
			name:       "invalid group",
			spec:       "/etc/config:*.conf:/tmp/config:1000:notagroup",
			wantErr:    true,
			wantErrMsg: "invalid group \"notagroup\"",
		},
		{
			name:       "invalid mode",
			spec:       "/etc/config:*.conf:/tmp/config:1000:1001:notamode",
			wantErr:    true,
			wantErrMsg: "invalid mode \"notamode\"",
		},
		{
			name:       "mode out of range",
			spec:       "/etc/config:*.conf:/tmp/config:1000:1001:1777",
			wantErr:    true,
			wantErrMsg: "mode 1777 is out of range, must be 0 to 0777",
		},
		{
			name:       "owner without dstPath",
			spec:       "/etc/config:*.conf::1000",
			wantErr:    true,
			wantErrMsg: "can't set owner/group/mode without a destination path: /etc/config:*.conf::1000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc, err := ParsePathConfig(tt.spec)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for spec %q, got nil", tt.spec)
					return
				}
				if tt.wantErrMsg != "" && !strings.HasPrefix(err.Error(), tt.wantErrMsg) {
					t.Errorf("error for spec %q: got %q, want prefix %q", tt.spec, err.Error(), tt.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for spec %q: %v", tt.spec, err)
				return
			}
			if pc == nil {
				t.Errorf("expected PathConfig, got nil")
				return
			}
			// Check fields if want is set
			if tt.want != nil {
				if pc.srcPath != tt.want.srcPath {
					t.Errorf("srcPath: got %q, want %q", pc.srcPath, tt.want.srcPath)
				}
				if pc.fileGlob != tt.want.fileGlob {
					t.Errorf("fileGlob: got %q, want %q", pc.fileGlob, tt.want.fileGlob)
				}
				if pc.dstPath != tt.want.dstPath {
					t.Errorf("dstPath: got %q, want %q", pc.dstPath, tt.want.dstPath)
				}
				if pc.owner != tt.want.owner {
					t.Errorf("owner: got %d, want %d", pc.owner, tt.want.owner)
				}
				if pc.group != tt.want.group {
					t.Errorf("group: got %d, want %d", pc.group, tt.want.group)
				}
				if pc.mode != tt.want.mode {
					t.Errorf("mode: got %o, want %o", pc.mode, tt.want.mode)
				}
			}
		})
	}
}

func TestPathConfigMethods(t *testing.T) {
	cases := []struct {
		name      string
		pc        PathConfig
		want      map[string]interface{}
		wantFlags map[string]bool
	}{
		{
			name: "all fields set",
			pc: PathConfig{
				srcPath:  "/etc/config",
				fileGlob: "*.conf",
				dstPath:  "/tmp/config",
				owner:    1000,
				group:    1001,
				mode:     0644,
				isSet:    0x1f,
			},
			want: map[string]interface{}{
				"SrcPath":  "/etc/config",
				"FileGlob": "*.conf",
				"DstPath":  "/tmp/config",
				"Owner":    uint32(1000),
				"Group":    uint32(1001),
				"Mode":     uint16(0644),
			},
			wantFlags: map[string]bool{
				"HasFileGlob": true,
				"HasDstPath":  true,
				"HasOwner":    true,
				"HasGroup":    true,
				"HasMode":     true,
			},
		},
		{
			name: "only srcPath",
			pc: PathConfig{
				srcPath: "/etc/config",
				isSet:   0x00,
			},
			want: map[string]interface{}{
				"SrcPath":  "/etc/config",
				"FileGlob": "",
				"DstPath":  "",
				"Owner":    uint32(0),
				"Group":    uint32(0),
				"Mode":     uint16(0),
			},
			wantFlags: map[string]bool{
				"HasFileGlob": false,
				"HasDstPath":  false,
				"HasOwner":    false,
				"HasGroup":    false,
				"HasMode":     false,
			},
		},
		{
			name: "srcPath and fileGlob",
			pc: PathConfig{
				srcPath:  "/etc/config",
				fileGlob: "*.conf",
				isSet:    0x01,
			},
			want: map[string]interface{}{
				"SrcPath":  "/etc/config",
				"FileGlob": "*.conf",
				"DstPath":  "",
				"Owner":    uint32(0),
				"Group":    uint32(0),
				"Mode":     uint16(0),
			},
			wantFlags: map[string]bool{
				"HasFileGlob": true,
				"HasDstPath":  false,
				"HasOwner":    false,
				"HasGroup":    false,
				"HasMode":     false,
			},
		},
		{
			name: "srcPath, dstPath, owner",
			pc: PathConfig{
				srcPath: "/etc/config",
				dstPath: "/tmp/config",
				owner:   1000,
				isSet:   0x06,
			},
			want: map[string]interface{}{
				"SrcPath":  "/etc/config",
				"FileGlob": "",
				"DstPath":  "/tmp/config",
				"Owner":    uint32(1000),
				"Group":    uint32(0),
				"Mode":     uint16(0),
			},
			wantFlags: map[string]bool{
				"HasFileGlob": false,
				"HasDstPath":  true,
				"HasOwner":    true,
				"HasGroup":    false,
				"HasMode":     false,
			},
		},
		{
			name: "srcPath, dstPath, owner, group, mode",
			pc: PathConfig{
				srcPath: "/etc/config",
				dstPath: "/tmp/config",
				owner:   1000,
				group:   1001,
				mode:    0644,
				isSet:   0x1e,
			},
			want: map[string]interface{}{
				"SrcPath":  "/etc/config",
				"FileGlob": "",
				"DstPath":  "/tmp/config",
				"Owner":    uint32(1000),
				"Group":    uint32(1001),
				"Mode":     uint16(0644),
			},
			wantFlags: map[string]bool{
				"HasFileGlob": false,
				"HasDstPath":  true,
				"HasOwner":    true,
				"HasGroup":    true,
				"HasMode":     true,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.pc.SrcPath(); got != tc.want["SrcPath"] {
				t.Errorf("SrcPath: got %q, want %q", got, tc.want["SrcPath"])
			}
			if got := tc.pc.FileGlob(); got != tc.want["FileGlob"] {
				t.Errorf("FileGlob: got %q, want %q", got, tc.want["FileGlob"])
			}
			if got := tc.pc.DstPath(); got != tc.want["DstPath"] {
				t.Errorf("DstPath: got %q, want %q", got, tc.want["DstPath"])
			}
			if got := tc.pc.Owner(); got != tc.want["Owner"] {
				t.Errorf("Owner: got %d, want %d", got, tc.want["Owner"])
			}
			if got := tc.pc.Group(); got != tc.want["Group"] {
				t.Errorf("Group: got %d, want %d", got, tc.want["Group"])
			}
			if got := tc.pc.Mode(); got != tc.want["Mode"] {
				t.Errorf("Mode: got %o, want %o", got, tc.want["Mode"])
			}
			if got := tc.pc.HasFileGlob(); got != tc.wantFlags["HasFileGlob"] {
				t.Errorf("HasFileGlob: got %v, want %v", got, tc.wantFlags["HasFileGlob"])
			}
			if got := tc.pc.HasDstPath(); got != tc.wantFlags["HasDstPath"] {
				t.Errorf("HasDstPath: got %v, want %v", got, tc.wantFlags["HasDstPath"])
			}
			if got := tc.pc.HasOwner(); got != tc.wantFlags["HasOwner"] {
				t.Errorf("HasOwner: got %v, want %v", got, tc.wantFlags["HasOwner"])
			}
			if got := tc.pc.HasGroup(); got != tc.wantFlags["HasGroup"] {
				t.Errorf("HasGroup: got %v, want %v", got, tc.wantFlags["HasGroup"])
			}
			if got := tc.pc.HasMode(); got != tc.wantFlags["HasMode"] {
				t.Errorf("HasMode: got %v, want %v", got, tc.wantFlags["HasMode"])
			}
		})
	}
}

func TestPathConfigString(t *testing.T) {
	cases := []struct {
		name string
		pc   PathConfig
		want string
	}{
		{
			name: "all fields set",
			pc: PathConfig{
				srcPath:  "/etc/config",
				fileGlob: "*.conf",
				dstPath:  "/tmp/config",
				owner:    1000,
				group:    1001,
				mode:     0644,
				isSet:    0x1f,
			},
			want: "/etc/config:*.conf:/tmp/config:1000:1001:644",
		},
		{
			name: "only srcPath",
			pc: PathConfig{
				srcPath: "/etc/config",
				isSet:   0x00,
			},
			want: "/etc/config",
		},
		{
			name: "srcPath and fileGlob",
			pc: PathConfig{
				srcPath:  "/etc/config",
				fileGlob: "*.conf",
				isSet:    0x01,
			},
			want: "/etc/config:*.conf",
		},
		{
			name: "srcPath, fileGlob, dstPath",
			pc: PathConfig{
				srcPath:  "/etc/config",
				fileGlob: "*.conf",
				dstPath:  "/tmp/config",
				isSet:    0x03,
			},
			want: "/etc/config:*.conf:/tmp/config",
		},
		{
			name: "srcPath, dstPath, owner",
			pc: PathConfig{
				srcPath: "/etc/config",
				dstPath: "/tmp/config",
				owner:   1000,
				isSet:   0x06,
			},
			want: "/etc/config:*:/tmp/config:1000",
		},
		{
			name: "srcPath, dstPath, owner, group",
			pc: PathConfig{
				srcPath: "/etc/config",
				dstPath: "/tmp/config",
				owner:   1000,
				group:   1001,
				isSet:   0x0e,
			},
			want: "/etc/config:*:/tmp/config:1000:1001",
		},
		{
			name: "srcPath, dstPath, owner, group, mode",
			pc: PathConfig{
				srcPath: "/etc/config",
				dstPath: "/tmp/config",
				owner:   1000,
				group:   1001,
				mode:    0644,
				isSet:   0x1e,
			},
			want: "/etc/config:*:/tmp/config:1000:1001:644",
		},
		{
			name: "srcPath, fileGlob, dstPath, mode only",
			pc: PathConfig{
				srcPath:  "/etc/config",
				fileGlob: "*.conf",
				dstPath:  "/tmp/config",
				mode:     0644,
				isSet:    0x13,
			},
			want: "/etc/config:*.conf:/tmp/config:::644",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.pc.String()
			if got != tc.want {
				t.Errorf("String() for %s: got %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestPathConfigID(t *testing.T) {
	tests := []struct {
		name string
		pc1  *PathConfig
		pc2  *PathConfig
		same bool
	}{
		{
			name: "identical configs should have same ID",
			pc1: &PathConfig{
				srcPath:  "/etc/config",
				fileGlob: "*.conf",
				dstPath:  "/tmp/config",
				isSet:    0x03,
			},
			pc2: &PathConfig{
				srcPath:  "/etc/config",
				fileGlob: "*.conf",
				dstPath:  "/tmp/config",
				isSet:    0x03,
			},
			same: true,
		},
		{
			name: "different srcPaths should have different IDs",
			pc1: &PathConfig{
				srcPath: "/etc/config",
				isSet:   0x00,
			},
			pc2: &PathConfig{
				srcPath: "/etc/other",
				isSet:   0x00,
			},
			same: false,
		},
		{
			name: "different fileGlobs should have different IDs",
			pc1: &PathConfig{
				srcPath:  "/etc/config",
				fileGlob: "*.conf",
				isSet:    0x01,
			},
			pc2: &PathConfig{
				srcPath:  "/etc/config",
				fileGlob: "*.txt",
				isSet:    0x01,
			},
			same: false,
		},
		{
			name: "different dstPaths should have different IDs",
			pc1: &PathConfig{
				srcPath: "/etc/config",
				dstPath: "/tmp/config",
				isSet:   0x02,
			},
			pc2: &PathConfig{
				srcPath: "/etc/config",
				dstPath: "/var/config",
				isSet:   0x02,
			},
			same: false,
		},
		{
			name: "owner/group/mode don't affect ID",
			pc1: &PathConfig{
				srcPath: "/etc/config",
				dstPath: "/tmp/config",
				owner:   1000,
				group:   1001,
				mode:    0644,
				isSet:   0x1e,
			},
			pc2: &PathConfig{
				srcPath: "/etc/config",
				dstPath: "/tmp/config",
				owner:   2000,
				group:   2001,
				mode:    0755,
				isSet:   0x1e,
			},
			same: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id1, err1 := tt.pc1.ID()
			if err1 != nil {
				t.Fatalf("pc1.ID() error = %v", err1)
			}

			id2, err2 := tt.pc2.ID()
			if err2 != nil {
				t.Fatalf("pc2.ID() error = %v", err2)
			}

			if tt.same && id1 != id2 {
				t.Errorf("Expected same IDs: %s != %s", id1, id2)
			}
			if !tt.same && id1 == id2 {
				t.Errorf("Expected different IDs but got same: %s", id1)
			}

			// Verify IDs are valid hex strings and have expected length
			if len(id1) != 16 { // 64-bit hash = 8 bytes = 16 hex chars
				t.Errorf("ID length should be 16, got %d", len(id1))
			}
			if len(id2) != 16 {
				t.Errorf("ID length should be 16, got %d", len(id2))
			}
		})
	}
}

func TestPathConfigMarshalLogObject(t *testing.T) {
	tests := []struct {
		name string
		pc   *PathConfig
		want map[string]interface{}
	}{
		{
			name: "all fields set",
			pc: &PathConfig{
				srcPath:  "/etc/config",
				fileGlob: "*.conf",
				dstPath:  "/tmp/config",
				owner:    1000,
				group:    1001,
				mode:     0644,
				isSet:    0x1f,
			},
			want: map[string]interface{}{
				"srcPath":  "/etc/config",
				"fileGlob": "*.conf",
				"dstPath":  "/tmp/config",
				"owner":    uint32(1000),
				"group":    uint32(1001),
				"mode":     "644",
			},
		},
		{
			name: "only srcPath",
			pc: &PathConfig{
				srcPath: "/etc/config",
				isSet:   0x00,
			},
			want: map[string]interface{}{
				"srcPath":  "/etc/config",
				"fileGlob": "*",
			},
		},
		{
			name: "srcPath and dstPath only",
			pc: &PathConfig{
				srcPath: "/etc/config",
				dstPath: "/tmp/config",
				isSet:   0x02,
			},
			want: map[string]interface{}{
				"srcPath":  "/etc/config",
				"fileGlob": "*",
				"dstPath":  "/tmp/config",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := NewMockObjectEncoder()
			err := tt.pc.MarshalLogObject(enc)
			if err != nil {
				t.Fatalf("MarshalLogObject() error = %v", err)
			}

			for key, expectedValue := range tt.want {
				if actualValue, exists := enc.Fields[key]; !exists {
					t.Errorf("Expected field %s not found", key)
				} else if actualValue != expectedValue {
					t.Errorf("Field %s: got %v, want %v", key, actualValue, expectedValue)
				}
			}

			// Check that no unexpected fields are present
			for key := range enc.Fields {
				if _, expected := tt.want[key]; !expected {
					t.Errorf("Unexpected field %s with value %v", key, enc.Fields[key])
				}
			}
		})
	}
}

func TestPathConfigEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		spec       string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:       "empty spec",
			spec:       "",
			wantErr:    true,
			wantErrMsg: "source path must be absolute:",
		},
		{
			name:       "too many parts",
			spec:       "/a:b:c:d:e:f:g",
			wantErr:    true,
			wantErrMsg: "invalid path config spec:",
		},
		{
			name:       "mode set without dst path",
			spec:       "/etc/config:::1000",
			wantErr:    true,
			wantErrMsg: "can't set owner/group/mode without a destination path:",
		},
		{
			name:       "group set without dst path",
			spec:       "/etc/config::::1001",
			wantErr:    true,
			wantErrMsg: "can't set owner/group/mode without a destination path:",
		},
		{
			name:       "mode set without dst path via flag",
			spec:       "/etc/config:::::644",
			wantErr:    true,
			wantErrMsg: "can't set owner/group/mode without a destination path:",
		},
		{
			name:    "empty fileGlob is treated as not set",
			spec:    "/etc/config:",
			wantErr: false,
		},
		{
			name:    "wildcard fileGlob is treated as not set",
			spec:    "/etc/config:*",
			wantErr: false,
		},
		{
			name:    "empty dstPath",
			spec:    "/etc/config:*.conf:",
			wantErr: false,
		},
		{
			name:    "large but valid owner",
			spec:    "/etc/config:*.conf:/tmp:4294967295", // max uint32
			wantErr: false,
		},
		{
			name:       "owner too large",
			spec:       "/etc/config:*.conf:/tmp:4294967296", // max uint32 + 1
			wantErr:    true,
			wantErrMsg: "invalid owner",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePathConfig(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePathConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
				t.Errorf("ParsePathConfig() error = %v, want error containing %v", err, tt.wantErrMsg)
			}
		})
	}
}
