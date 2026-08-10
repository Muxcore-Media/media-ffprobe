package internal

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"

	ffprobev1 "github.com/Muxcore-Media/media-ffprobe/proto/ffprobev1"

	"github.com/Muxcore-Media/core/pkg/contracts"
	_ "modernc.org/sqlite"
)

type Module struct {
	ffprobev1.UnimplementedAnalysisServiceServer

	mu sync.RWMutex
	db *sql.DB

	id       string
	dbPath   string
	grpcAddr string
	grpcSrv  *grpc.Server
	grpcLis  net.Listener
}

type Config struct {
	ID       string
	DBPath   string
	GRPCAddr string
}

func NewModule(cfg Config) *Module {
	if cfg.ID == "" {
		cfg.ID = "media-ffprobe"
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "/var/lib/media-ffprobe/cache.db"
	}
	if cfg.GRPCAddr == "" {
		cfg.GRPCAddr = ":9480"
	}
	if v := os.Getenv("FFPROBE_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("FFPROBE_GRPC_ADDR"); v != "" {
		cfg.GRPCAddr = v
	}
	return &Module{
		id:       cfg.ID,
		dbPath:   cfg.DBPath,
		grpcAddr: cfg.GRPCAddr,
	}
}

func (m *Module) Info() contracts.ModuleInfo {
	return contracts.ModuleInfo{
		ID:             m.id,
		Name:           "Media FFprobe",
		Version:      "0.1.6",
		Roles:          []string{"analyzer"},
		Description:    "Media file analysis via ffprobe — detects codec, resolution, HDR, bitrate, and quality",
		Author:         "MuxCore",
		// Do not advertise bare "metadata" — that collides with metadata-tmdb discovery.
		Capabilities:   []string{"media.analyzer"},
		MinCoreVersion: "0.4.0",
		HTTPAddr:       m.grpcAddr,
	}
}

func (m *Module) Init(ctx context.Context) error {
	dir := filepath.Dir(m.dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", m.dbPath)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.ExecContext(ctx, `PRAGMA journal_mode=WAL`); err != nil {
		db.Close()
		return fmt.Errorf("enable WAL: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS analysis_cache (
			file_path    TEXT PRIMARY KEY,
			file_size    INTEGER NOT NULL,
			file_modtime INTEGER NOT NULL,
			result_json  TEXT NOT NULL,
			created_at   TEXT NOT NULL
		)
	`); err != nil {
		db.Close()
		return fmt.Errorf("create cache table: %w", err)
	}

	m.mu.Lock()
	m.db = db
	m.mu.Unlock()

	lis, err := net.Listen("tcp", m.grpcAddr)
	if err != nil {
		db.Close()
		return fmt.Errorf("listen gRPC %s: %w", m.grpcAddr, err)
	}
	m.grpcLis = lis

	slog.Info("media-ffprobe initialized", "db", m.dbPath, "grpc", m.grpcAddr)
	return nil
}

func (m *Module) Start(ctx context.Context) error {
	m.grpcSrv = grpc.NewServer()
	ffprobev1.RegisterAnalysisServiceServer(m.grpcSrv, m)

	go func() {
		slog.Info("media-ffprobe gRPC service started", "addr", m.grpcAddr)
		if err := m.grpcSrv.Serve(m.grpcLis); err != nil {
			slog.Error("media-ffprobe gRPC serve error", "error", err)
		}
	}()
	return nil
}

func (m *Module) Stop(ctx context.Context) error {
	if m.grpcSrv != nil {
		m.grpcSrv.GracefulStop()
	}
	m.mu.Lock()
	if m.db != nil {
		m.db.Close()
		m.db = nil
	}
	m.mu.Unlock()
	slog.Info("media-ffprobe stopped")
	return nil
}

func (m *Module) Health(ctx context.Context) error {
	m.mu.RLock()
	db := m.db
	m.mu.RUnlock()
	if db == nil {
		return fmt.Errorf("not initialized")
	}
	return db.PingContext(ctx)
}

// ── ffprobe types ─────────────────────────────────────────────

type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeFormat struct {
	Filename   string `json:"filename"`
	FormatName string `json:"format_name"`
	Size       string `json:"size"`
	BitRate    string `json:"bit_rate"`
	Duration   string `json:"duration"`
}

type ffprobeStream struct {
	Index          int               `json:"index"`
	CodecType      string            `json:"codec_type"`
	CodecName      string            `json:"codec_name"`
	CodecLongName  string            `json:"codec_long_name"`
	Profile        string            `json:"profile"`
	Width          int               `json:"width"`
	Height         int               `json:"height"`
	CodedWidth     int               `json:"coded_width"`
	CodedHeight    int               `json:"coded_height"`
	RFrameRate     string            `json:"r_frame_rate"`
	AvgFrameRate   string            `json:"avg_frame_rate"`
	BitRate        string            `json:"bit_rate"`
	PixelFormat    string            `json:"pix_fmt"`
	ColorPrimaries string            `json:"color_primaries"`
	ColorTransfer  string            `json:"color_transfer"`
	ColorSpace     string            `json:"color_space"`
	FieldOrder     string            `json:"field_order"`
	Language       string            `json:"language"`
	ChannelLayout  string            `json:"channel_layout"`
	Channels       int               `json:"channels"`
	SampleRate     string            `json:"sample_rate"`
	AspectRatio    string            `json:"display_aspect_ratio"`
	SideDataList   []ffprobeSideData `json:"side_data_list"`
	Tags           map[string]string `json:"tags"`
	Disposition    map[string]int    `json:"disposition"`
}

type ffprobeSideData struct {
	SideDataType string `json:"side_data_type"`
}

// ── gRPC API ───────────────────────────────────────────────────

func (m *Module) Analyze(ctx context.Context, req *ffprobev1.AnalyzeRequest) (*ffprobev1.AnalyzeResponse, error) {
	path := req.GetFilePath()
	if path == "" {
		return nil, fmt.Errorf("file_path is required")
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	if cached := m.checkCache(path); cached != nil {
		return cached, nil
	}

	result, err := m.runFFprobe(path)
	if err != nil {
		return nil, fmt.Errorf("ffprobe analysis: %w", err)
	}

	m.storeCache(path, result)
	return result, nil
}

func (m *Module) GetCached(ctx context.Context, req *ffprobev1.GetCachedRequest) (*ffprobev1.GetCachedResponse, error) {
	if cached := m.checkCache(req.GetFilePath()); cached != nil {
		return &ffprobev1.GetCachedResponse{Result: cached, Found: true}, nil
	}
	return &ffprobev1.GetCachedResponse{Found: false}, nil
}

// ── Cache ──────────────────────────────────────────────────────

func (m *Module) checkCache(path string) *ffprobev1.AnalyzeResponse {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	size := info.Size()
	modTime := info.ModTime().Unix()

	m.mu.RLock()
	defer m.mu.RUnlock()

	var resultJSON string
	var cachedSize int64
	var cachedModTime int64
	err = m.db.QueryRow(
		`SELECT file_size, file_modtime, result_json FROM analysis_cache WHERE file_path = ?`, path,
	).Scan(&cachedSize, &cachedModTime, &resultJSON)

	if err != nil || cachedSize != size || cachedModTime != modTime {
		return nil
	}

	var resp ffprobev1.AnalyzeResponse
	if err := json.Unmarshal([]byte(resultJSON), &resp); err != nil {
		return nil
	}
	return &resp
}

func (m *Module) storeCache(path string, result *ffprobev1.AnalyzeResponse) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.db.Exec(`INSERT OR REPLACE INTO analysis_cache (file_path, file_size, file_modtime, result_json, created_at) VALUES (?, ?, ?, ?, ?)`,
		path, info.Size(), info.ModTime().Unix(), string(jsonBytes), time.Now().UTC().Format(time.RFC3339))
}

// ── ffprobe invocation ─────────────────────────────────────────

func (m *Module) runFFprobe(path string) (*ffprobev1.AnalyzeResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe exec: %w\nstderr: %s", err, stderr.String())
	}

	var output ffprobeOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}

	return m.parseOutput(path, &output), nil
}

func (m *Module) parseOutput(path string, out *ffprobeOutput) *ffprobev1.AnalyzeResponse {
	resp := &ffprobev1.AnalyzeResponse{
		FilePath: path,
	}

	size, _ := strconv.ParseInt(out.Format.Size, 10, 64)
	resp.SizeBytes = size
	resp.Container = out.Format.FormatName

	if d, err := strconv.ParseFloat(out.Format.Duration, 64); err == nil {
		resp.DurationSeconds = d
	}
	if b, err := strconv.ParseFloat(out.Format.BitRate, 64); err == nil {
		resp.OverallBitrate = b
	}

	var videos []*ffprobev1.VideoStream
	var audios []*ffprobev1.AudioStream
	var subs []*ffprobev1.SubtitleStream

	for _, s := range out.Streams {
		switch s.CodecType {
		case "video":
			v := m.parseVideo(s)
			videos = append(videos, v)
		case "audio":
			a := m.parseAudio(s)
			audios = append(audios, a)
		case "subtitle":
			sub := m.parseSubtitle(s)
			subs = append(subs, sub)
		}
	}

	if len(videos) > 0 {
		resp.Video = videos[0]
	}
	resp.Audio = audios
	resp.Subtitles = subs

	resp.Quality = classifyQuality(resp.Video)
	return resp
}

func (m *Module) parseVideo(s ffprobeStream) *ffprobev1.VideoStream {
	v := &ffprobev1.VideoStream{
		Index:          int32(s.Index),
		Codec:          s.CodecName,
		CodecLong:      s.CodecLongName,
		Width:          int32(s.Width),
		Height:         int32(s.Height),
		PixelFormat:    s.PixelFormat,
		ColorPrimaries: s.ColorPrimaries,
		ColorTransfer:  s.ColorTransfer,
		ColorSpace:     s.ColorSpace,
		AspectRatio:    s.AspectRatio,
	}

	v.FrameRate = parseFrameRate(s.RFrameRate)
	if v.FrameRate == 0 {
		v.FrameRate = parseFrameRate(s.AvgFrameRate)
	}

	if b, err := strconv.ParseFloat(s.BitRate, 64); err == nil {
		v.Bitrate = b
	}

	v.ResolutionLabel = resolutionLabel(int(s.Width), int(s.Height))
	v.Hdr, v.HdrType = detectHDR(s)
	return v
}

func (m *Module) parseAudio(s ffprobeStream) *ffprobev1.AudioStream {
	a := &ffprobev1.AudioStream{
		Index:         int32(s.Index),
		Codec:         s.CodecName,
		CodecLong:     s.CodecLongName,
		Profile:       s.Profile,
		Language:      s.Language,
		Channels:      int32(s.Channels),
		ChannelLayout: s.ChannelLayout,
	}
	if b, err := strconv.ParseFloat(s.BitRate, 64); err == nil {
		a.Bitrate = b
	}
	if a.Language == "" {
		if tagLang, ok := s.Tags["language"]; ok {
			a.Language = tagLang
		}
	}
	return a
}

func (m *Module) parseSubtitle(s ffprobeStream) *ffprobev1.SubtitleStream {
	sub := &ffprobev1.SubtitleStream{
		Index: int32(s.Index),
		Codec: s.CodecName,
	}
	if s.Language != "" {
		sub.Language = s.Language
	} else if tagLang, ok := s.Tags["language"]; ok {
		sub.Language = tagLang
	}
	if s.Disposition != nil {
		sub.Forced = s.Disposition["forced"] == 1
		sub.HearingImpaired = s.Disposition["hearing_impaired"] == 1
	}
	return sub
}

// ── HDR Detection ──────────────────────────────────────────────

func detectHDR(s ffprobeStream) (bool, string) {
	ct := strings.ToLower(s.ColorTransfer)
	cp := strings.ToLower(s.ColorPrimaries)

	switch {
	case ct == "smpte2084" || ct == "smpte-st-2084" || ct == "pq":
		if hasDolbyVision(s) {
			return true, "Dolby Vision"
		}
		if hasHDR10Plus(s) {
			return true, "HDR10+"
		}
		return true, "HDR10"

	case ct == "arib-std-b67" || ct == "hlg":
		return true, "HLG"

	case cp == "bt2020" || cp == "bt.2020":
		return true, "HDR"
	}

	return false, ""
}

func hasDolbyVision(s ffprobeStream) bool {
	for _, sd := range s.SideDataList {
		if strings.Contains(strings.ToLower(sd.SideDataType), "dolby vision") {
			return true
		}
	}
	if s.Profile != "" && strings.Contains(strings.ToLower(s.Profile), "dolby vision") {
		return true
	}
	return false
}

func hasHDR10Plus(s ffprobeStream) bool {
	for _, sd := range s.SideDataList {
		if strings.Contains(strings.ToLower(sd.SideDataType), "hdr10+") {
			return true
		}
	}
	return false
}

// ── Quality Classification ─────────────────────────────────────

func classifyQuality(v *ffprobev1.VideoStream) *ffprobev1.MediaQuality {
	if v == nil {
		return &ffprobev1.MediaQuality{Label: "Unknown", Score: 0}
	}

	q := &ffprobev1.MediaQuality{
		Resolution: resolutionLabel(int(v.Width), int(v.Height)),
		CodecGroup: codecGroup(v.Codec),
		Hdr:        v.Hdr,
	}

	q.Source = classifySource(v)
	q.Score = int32(qualityScore(q.Resolution, q.Source, v.Hdr))
	q.Label = qualityLabel(q.Resolution, q.Source, v.Hdr, q.CodecGroup)
	return q
}

func classifySource(v *ffprobev1.VideoStream) string {
	codec := strings.ToLower(v.Codec)
	pix := strings.ToLower(v.PixelFormat)

	switch {
	case strings.Contains(codec, "raw"):
		return "Remux"
	case pix == "yuv420p10le" || pix == "yuv420p12le" || pix == "yuv444p10le":
		return "BluRay"
	default:
		return "WEB-DL"
	}
}

func resolutionLabel(width, height int) string {
	maxDim := width
	if height > maxDim {
		maxDim = height
	}
	switch {
	case maxDim >= 3840:
		return "2160p"
	case maxDim >= 1920:
		return "1080p"
	case maxDim >= 1280:
		return "720p"
	case maxDim >= 720:
		return "576p"
	default:
		return "SD"
	}
}

func codecGroup(codec string) string {
	switch {
	case strings.Contains(codec, "av1"):
		return "av1"
	case strings.Contains(codec, "hevc") || strings.Contains(codec, "h265") || strings.Contains(codec, "265"):
		return "hevc"
	case strings.Contains(codec, "vp9"):
		return "vp9"
	case strings.Contains(codec, "264") || strings.Contains(codec, "h264") || strings.Contains(codec, "avc"):
		return "h264"
	default:
		return "other"
	}
}

func qualityScore(resolution, source string, hdr bool) int {
	score := 0

	switch resolution {
	case "2160p":
		score += 120
	case "1080p":
		score += 100
	case "720p":
		score += 80
	case "576p":
		score += 60
	default:
		score += 40
	}

	switch source {
	case "Remux":
		score += 40
	case "BluRay":
		score += 30
	case "WEB-DL":
		score += 20
	}

	if hdr {
		score += 10
	}

	return score
}

func qualityLabel(resolution, source string, hdr bool, codec string) string {
	parts := []string{resolution}
	if source != "" && source != "WEB-DL" {
		parts = append(parts, source)
	} else {
		parts = append(parts, "WEB-DL")
	}
	if hdr {
		parts = append(parts, "HDR")
	}
	if codec != "" && codec != "other" {
		parts = append(parts, strings.ToUpper(codec))
	}
	return strings.Join(parts, " ")
}

// ── Helpers ────────────────────────────────────────────────────

func parseFrameRate(r string) float64 {
	if r == "" {
		return 0
	}
	parts := strings.Split(r, "/")
	if len(parts) == 2 {
		num, _ := strconv.ParseFloat(parts[0], 64)
		den, _ := strconv.ParseFloat(parts[1], 64)
		if den > 0 {
			return math.Round(num/den*1000) / 1000
		}
	}
	v, _ := strconv.ParseFloat(r, 64)
	return v
}

var _ contracts.Module = (*Module)(nil)
