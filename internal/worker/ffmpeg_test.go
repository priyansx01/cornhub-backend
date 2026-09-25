package worker

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuildHLSArgs transcodes short generated clips with the real ffmpeg
// binary and checks that a playable multi-rendition HLS stream comes out.
func TestBuildHLSArgs(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	cases := []struct {
		name  string
		input []string // ffmpeg args that generate the source clip
		audio bool
	}{
		{
			name:  "with audio",
			input: []string{"-f", "lavfi", "-i", "testsrc=size=1280x720:rate=30", "-f", "lavfi", "-i", "sine=frequency=440", "-t", "3", "-pix_fmt", "yuv420p", "-c:v", "libx264", "-c:a", "aac", "-shortest"},
			audio: true,
		},
		{
			name:  "no audio track",
			input: []string{"-f", "lavfi", "-i", "testsrc=size=1280x720:rate=30", "-t", "3", "-pix_fmt", "yuv420p", "-c:v", "libx264"},
		},
		{
			name:  "4:4:4 source",
			input: []string{"-f", "lavfi", "-i", "testsrc=size=640x360:rate=30", "-t", "3", "-pix_fmt", "yuv444p", "-c:v", "libx264"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "src.mp4")
			gen := append([]string{"-y", "-loglevel", "error"}, tc.input...)
			if out, err := exec.Command("ffmpeg", append(gen, src)...).CombinedOutput(); err != nil {
				t.Fatalf("generate source: %v\n%s", err, out)
			}

			withAudio, err := hasAudio(src)
			if err != nil {
				t.Fatalf("hasAudio: %v", err)
			}
			if withAudio != tc.audio {
				t.Fatalf("hasAudio = %v, want %v", withAudio, tc.audio)
			}

			outDir := filepath.Join(dir, "hls")
			if err := os.MkdirAll(outDir, 0755); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command("ffmpeg", BuildHLSArgs(src, outDir, withAudio)...).CombinedOutput(); err != nil {
				t.Fatalf("transcode: %v\n%s", err, out)
			}

			master, err := os.ReadFile(filepath.Join(outDir, "master.m3u8"))
			if err != nil {
				t.Fatalf("read master playlist: %v", err)
			}
			for i := range ladder {
				variant := filepath.Join(outDir, "v"+string(rune('0'+i))+"_index.m3u8")
				if !strings.Contains(string(master), filepath.Base(variant)) {
					t.Errorf("master playlist does not reference %s:\n%s", filepath.Base(variant), master)
				}
				if _, err := os.Stat(variant); err != nil {
					t.Errorf("missing variant playlist: %v", err)
				}
			}
			segs, _ := filepath.Glob(filepath.Join(outDir, "*.ts"))
			if len(segs) < len(ladder) {
				t.Errorf("got %d segments, want at least %d", len(segs), len(ladder))
			}
		})
	}
}
