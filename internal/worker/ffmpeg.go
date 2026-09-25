package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-kafka/v3/pkg/kafka"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/redis/go-redis/v9"

	"github.com/priyansx01/corn-hub-clone/internal/config"
	"github.com/priyansx01/corn-hub-clone/internal/event"
	"github.com/priyansx01/corn-hub-clone/internal/storage"
)

type VideoProcessor struct {
	cfg     config.Config
	storage *storage.Client
	rdb     *redis.Client
	sub     message.Subscriber
	pub     *event.Publisher
}

func NewVideoProcessor(cfg config.Config, st *storage.Client, rdb *redis.Client, p *event.Publisher) (*VideoProcessor, error) {
	sub, err := kafka.NewSubscriber(
		kafka.SubscriberConfig{
			Brokers:       []string{cfg.Kafka.Brokers},
			Unmarshaler:   kafka.DefaultMarshaler{},
			ConsumerGroup: "ffmpeg-workers",
		},
		watermill.NewStdLogger(false, false),
	)
	if err != nil {
		return nil, fmt.Errorf("create subscriber: %w", err)
	}

	return &VideoProcessor{
		cfg:     cfg,
		storage: st,
		rdb:     rdb,
		sub:     sub,
		pub:     p,
	}, nil
}

func (vp *VideoProcessor) Start(ctx context.Context) error {
	messages, err := vp.sub.Subscribe(ctx, event.TopicVideoUploaded)
	if err != nil {
		return fmt.Errorf("subscribe to %s: %w", event.TopicVideoUploaded, err)
	}

	log.Printf("🎥 VideoProcessor started. Listening to %s...", event.TopicVideoUploaded)

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping video processor...")
			return vp.sub.Close()
		case msg, ok := <-messages:
			if !ok {
				return nil
			}
			vp.handleMessage(ctx, msg)
		}
	}
}

func (vp *VideoProcessor) handleMessage(ctx context.Context, msg *message.Message) {
	var ev event.VideoUploadedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		log.Printf("❌ Failed to unmarshal message: %v", err)
		msg.Ack()
		return
	}

	log.Printf("⚙️ Processing video for course %s, module %s...", ev.CourseID, ev.ModuleID)
	if err := vp.process(ctx, ev); err != nil {
		// Transcoding failures are almost always deterministic (bad input, missing
		// object), so redelivering would loop forever. Surface the failure through
		// the progress endpoint and move on.
		log.Printf("❌ Processing failed for module %s: %v", ev.ModuleID, err)
		vp.updateProgress(ctx, ev.ModuleID, 0, "failed")
		msg.Ack()
		return
	}
	msg.Ack()
}

func (vp *VideoProcessor) process(ctx context.Context, ev event.VideoUploadedEvent) error {
	vp.updateProgress(ctx, ev.ModuleID, 0, "downloading")

	// 1. Create a temp directory
	tmpDir, err := os.MkdirTemp("", "ffmpeg-lms-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	inputPath := filepath.Join(tmpDir, "input"+filepath.Ext(ev.MinioKey))

	// 2. Download video
	if err := vp.storage.DownloadRawFile(ctx, ev.MinioKey, inputPath); err != nil {
		return fmt.Errorf("download raw file: %w", err)
	}

	vp.updateProgress(ctx, ev.ModuleID, 5, "probing")

	// 3. Probe duration and streams
	durationStr, err := getDuration(inputPath)
	if err != nil {
		return fmt.Errorf("probe duration: %w", err)
	}
	totalSeconds, _ := strconv.ParseFloat(durationStr, 64)
	withAudio, err := hasAudio(inputPath)
	if err != nil {
		return fmt.Errorf("probe audio: %w", err)
	}

	vp.updateProgress(ctx, ev.ModuleID, 10, "transcoding")

	// 4. Transcode
	outDir := filepath.Join(tmpDir, "hls")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", BuildHLSArgs(inputPath, outDir, withAudio)...)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("pipe stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	// Keep the tail of stderr so a failure is diagnosable from the worker log.
	var tail []string
	timeRe := regexp.MustCompile(`time=([0-9]{2}):([0-9]{2}):([0-9]{2}\.[0-9]{2})`)
	scanner := bufio.NewScanner(stderr)
	scanner.Split(scanLinesOrCR)
	for scanner.Scan() {
		line := scanner.Text()
		if tail = append(tail, line); len(tail) > 10 {
			tail = tail[1:]
		}
		matches := timeRe.FindStringSubmatch(line)
		if len(matches) == 4 {
			h, _ := strconv.ParseFloat(matches[1], 64)
			m, _ := strconv.ParseFloat(matches[2], 64)
			s, _ := strconv.ParseFloat(matches[3], 64)
			currentTime := h*3600 + m*60 + s
			if totalSeconds > 0 {
				progress := 10 + int((currentTime/totalSeconds)*80) // 10% to 90%
				if progress > 90 {
					progress = 90
				}
				vp.updateProgress(ctx, ev.ModuleID, progress, "transcoding")
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg: %w\n%s", err, strings.Join(tail, "\n"))
	}

	vp.updateProgress(ctx, ev.ModuleID, 90, "thumbnailing")

	// 5. Generate Thumbnail (non-fatal: very short clips may have no frame at 1s)
	thumbPath := filepath.Join(tmpDir, "thumb.jpg")
	thumbAt := "00:00:01.000"
	if totalSeconds < 2 {
		thumbAt = "00:00:00.000"
	}
	if out, err := exec.CommandContext(ctx, "ffmpeg", "-y", "-ss", thumbAt, "-i", inputPath, "-vframes", "1", thumbPath).CombinedOutput(); err != nil {
		log.Printf("⚠ Thumbnail generation failed for module %s: %v\n%s", ev.ModuleID, err, out)
		thumbPath = ""
	}

	vp.updateProgress(ctx, ev.ModuleID, 95, "uploading")

	// 6. Upload files to MinIO
	hlsPrefix := storage.HLSPrefix(ev.CourseID, ev.ModuleID)
	files, err := filepath.Glob(filepath.Join(outDir, "*"))
	if err != nil {
		return fmt.Errorf("list hls output: %w", err)
	}
	for _, f := range files {
		fileName := filepath.Base(f)
		objectKey := fmt.Sprintf("%s/%s", hlsPrefix, fileName)
		contentType := "video/MP2T"
		if strings.HasSuffix(fileName, ".m3u8") {
			contentType = "application/vnd.apple.mpegurl"
		}
		if err := vp.storage.UploadHLSFile(ctx, objectKey, f, contentType); err != nil {
			return err
		}
	}

	var thumbnailURL string
	if thumbPath != "" {
		thumbObjectKey := hlsPrefix + "/thumb.jpg"
		if err := vp.storage.UploadThumbnail(ctx, thumbObjectKey, thumbPath, "image/jpeg"); err != nil {
			log.Printf("⚠ Thumbnail upload failed for module %s: %v", ev.ModuleID, err)
		} else {
			thumbnailURL = vp.storage.ThumbnailURL(thumbObjectKey)
		}
	}

	// 7. Publish success
	processedEv := event.VideoProcessedEvent{
		CourseID:        ev.CourseID,
		ModuleID:        ev.ModuleID,
		HLSUrl:          vp.storage.HLSMasterURL(ev.CourseID, ev.ModuleID),
		ThumbnailUrl:    thumbnailURL,
		DurationSeconds: int(totalSeconds),
		Status:          "ready",
	}

	if vp.pub != nil {
		if err := vp.pub.PublishJSON(ctx, event.TopicVideoProcessed, processedEv); err != nil {
			return fmt.Errorf("publish processed event: %w", err)
		}
	}

	vp.updateProgress(ctx, ev.ModuleID, 100, "ready")
	log.Printf("✅ Processing complete for course %s, module %s.", ev.CourseID, ev.ModuleID)
	return nil
}

// rendition is one rung of the HLS adaptive bitrate ladder.
type rendition struct {
	width, height int
	profile       string
	crf           int
	maxrate       string
	bufsize       string
}

var ladder = []rendition{
	{1920, 1080, "high", 20, "5350k", "7500k"},
	{1280, 720, "high", 21, "3000k", "4200k"},
	{854, 480, "main", 22, "1500k", "2100k"},
	{640, 360, "baseline", 23, "856k", "1200k"},
}

// BuildHLSArgs returns the ffmpeg arguments that transcode inputPath into a
// multi-bitrate HLS stream (master.m3u8 + one playlist per rendition) in outDir.
// Audio is mapped only when the source has an audio track; mapping a missing
// stream makes ffmpeg abort.
func BuildHLSArgs(inputPath, outDir string, withAudio bool) []string {
	splits := make([]string, len(ladder))
	scales := make([]string, len(ladder))
	for i, r := range ladder {
		splits[i] = fmt.Sprintf("[v%d]", i)
		// format=yuv420p: browsers only decode 4:2:0 H.264, and x264's
		// high/main/baseline profiles reject 4:2:2/4:4:4 input.
		scales[i] = fmt.Sprintf("[v%d]scale=w=%d:h=%d:force_original_aspect_ratio=decrease,pad=ceil(iw/2)*2:ceil(ih/2)*2,format=yuv420p[v%dout]",
			i, r.width, r.height, i)
	}
	filter := fmt.Sprintf("[0:v]split=%d%s; %s", len(ladder), strings.Join(splits, ""), strings.Join(scales, "; "))

	args := []string{"-y", "-i", inputPath, "-filter_complex", filter}
	streamMap := make([]string, len(ladder))
	for i, r := range ladder {
		args = append(args, "-map", fmt.Sprintf("[v%dout]", i))
		if withAudio {
			args = append(args, "-map", "0:a:0")
			streamMap[i] = fmt.Sprintf("v:%d,a:%d", i, i)
		} else {
			streamMap[i] = fmt.Sprintf("v:%d", i)
		}
		args = append(args,
			fmt.Sprintf("-c:v:%d", i), "libx264",
			fmt.Sprintf("-profile:v:%d", i), r.profile,
			fmt.Sprintf("-crf:v:%d", i), strconv.Itoa(r.crf),
			fmt.Sprintf("-maxrate:v:%d", i), r.maxrate,
			fmt.Sprintf("-bufsize:v:%d", i), r.bufsize,
		)
	}
	args = append(args, "-preset", "ultrafast", "-g", "48", "-keyint_min", "48", "-sc_threshold", "0")
	if withAudio {
		args = append(args, "-c:a", "aac", "-b:a", "128k", "-ac", "2")
	}
	args = append(args,
		"-f", "hls", "-hls_time", "6", "-hls_playlist_type", "vod", "-hls_flags", "independent_segments", "-hls_segment_type", "mpegts",
		"-hls_segment_filename", filepath.Join(outDir, "v%v_segment_%03d.ts"),
		"-master_pl_name", "master.m3u8",
		"-var_stream_map", strings.Join(streamMap, " "),
		filepath.Join(outDir, "v%v_index.m3u8"),
	)
	return args
}

func (vp *VideoProcessor) updateProgress(ctx context.Context, moduleID string, percent int, status string) {
	key := fmt.Sprintf("module:progress:%s", moduleID)
	data, _ := json.Marshal(map[string]interface{}{
		"percent": percent,
		"status":  status,
	})
	vp.rdb.Set(ctx, key, data, 24*time.Hour)
}

func getDuration(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", filePath)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func hasAudio(filePath string) (bool, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "a", "-show_entries", "stream=index", "-of", "csv=p=0", filePath)
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// scanLinesOrCR splits on \n or \r: ffmpeg rewrites its progress line in place
// with \r, so a plain line scanner would only see progress once ffmpeg exits.
func scanLinesOrCR(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i, b := range data {
		if b == '\n' || b == '\r' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}
