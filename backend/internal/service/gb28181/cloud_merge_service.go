package gb28181

// 云端录像多段合并。
//
// 实现选择：
//   - 异步任务表 + ffmpeg concat 无损合并（-c copy），不引入 ffmpeg Go 库；
//   - 环境没有 ffmpeg 时任务立即落为 failed，错误信息明确提示安装方式，
//     而不是让任务永远停在 pending（排障时最难判断的一种状态）；
//   - 单文件 zip 打包下载已有实现（gb_cloud_record_handler.go），本文件不重复。

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	// mergeCommandTimeout ffmpeg 合并的超时上限（长录像合集可能很久）。
	mergeCommandTimeout = 2 * time.Hour
	// mergeErrorTail 失败时保留的 ffmpeg 输出尾部长度。
	mergeErrorTail = 1500
)

// 合并任务错误。
var (
	ErrGbMergeTaskNotFound   = errors.New("合并任务不存在")
	ErrGbMergeRecordNotFound = errors.New("部分云端录像不存在")
	ErrGbMergeTaskRunning    = errors.New("合并任务正在执行中")
	ErrGbMergeTaskNotReady   = errors.New("合并任务尚未完成")
	ErrGbMergeNoFFmpeg       = errors.New("未安装 ffmpeg，无法合并云端录像")
)

// GbCloudMergeService 云端录像合并服务。
type GbCloudMergeService struct {
	repo         *repository.GbCloudMergeRepo
	mediaRepo    *repository.GbMediaServerRepo
	ffmpegLookup func() (string, error)
}

// NewGbCloudMergeService 构造合并服务。
func NewGbCloudMergeService() *GbCloudMergeService {
	return NewGbCloudMergeServiceWithFFmpegLookup(defaultFFmpegLookup)
}

// NewGbCloudMergeServiceWithFFmpegLookup 注入 ffmpeg 查找函数（测试用）。
func NewGbCloudMergeServiceWithFFmpegLookup(lookup func() (string, error)) *GbCloudMergeService {
	if lookup == nil {
		lookup = defaultFFmpegLookup
	}
	return &GbCloudMergeService{
		repo:         repository.NewGbCloudMergeRepo(),
		mediaRepo:    repository.NewGbMediaServerRepo(),
		ffmpegLookup: lookup,
	}
}

func defaultFFmpegLookup() (string, error) { return exec.LookPath("ffmpeg") }

// Create 创建合并任务。
//
// ffmpeg 缺失时直接落 failed 并返回任务（HTTP 仍为 200）：调用方拿到
// task.status=failed 与明确的 error_msg，比 4xx/5xx 更容易在列表中追溯。
func (s *GbCloudMergeService) Create(ctx context.Context, req *request.GbMergeTaskCreateReq) (*model.GbRecordMergeTask, error) {
	if req == nil {
		return nil, errors.New("请求不能为空")
	}
	ids := dedupePositiveIDs(req.IDs)
	if len(ids) < 2 {
		return nil, errors.New("至少选择两段录像才能合并")
	}

	records, err := s.repo.GetRecordsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	if len(records) != len(ids) {
		found := make(map[int64]bool, len(records))
		for _, r := range records {
			found[r.ID] = true
		}
		missing := make([]string, 0, len(ids)-len(records))
		for _, id := range ids {
			if !found[id] {
				missing = append(missing, strconv.FormatInt(id, 10))
			}
		}
		return nil, fmt.Errorf("%w: %s", ErrGbMergeRecordNotFound, strings.Join(missing, ","))
	}

	sort.SliceStable(records, func(i, j int) bool {
		a, b := records[i], records[j]
		if a.StartTime != nil && b.StartTime != nil && !a.StartTime.Equal(*b.StartTime) {
			return a.StartTime.Before(*b.StartTime)
		}
		if a.StartTime != nil && b.StartTime == nil {
			return true
		}
		if a.StartTime == nil && b.StartTime != nil {
			return false
		}
		return a.ID < b.ID
	})

	var totalLen int64
	orderedIDs := make([]string, 0, len(records))
	for _, r := range records {
		totalLen += r.TimeLen
		orderedIDs = append(orderedIDs, strconv.FormatInt(r.ID, 10))
	}

	name := trim(req.Name)
	if name == "" {
		first := records[0]
		base := strings.TrimSuffix(first.FileName, filepath.Ext(first.FileName))
		if base == "" {
			base = "录像合并"
		}
		name = base + "_" + time.Now().Format("20060102150405")
	}
	name = truncateRunes(name, 255)

	task := &model.GbRecordMergeTask{
		Name:        name,
		Status:      model.GbMergeStatusPending,
		RecordIDs:   strings.Join(orderedIDs, ","),
		RecordCount: len(records),
		DurationSec: totalLen,
	}
	if err := s.repo.CreateTask(ctx, task); err != nil {
		return nil, err
	}

	// 先探测 ffmpeg：缺失时立即失败，避免任务长期 pending。
	if _, err := s.ffmpegLookup(); err != nil {
		now := time.Now()
		msg := ErrGbMergeNoFFmpeg.Error() + "（请在后端所在主机安装 ffmpeg 后重新创建任务）"
		_ = s.repo.UpdateTaskFields(ctx, task.ID, map[string]interface{}{
			"status":      model.GbMergeStatusFailed,
			"error_msg":   msg,
			"started_dt":  now,
			"finished_dt": now,
		})
		task.Status = model.GbMergeStatusFailed
		task.ErrorMsg = msg
		task.StartedDT = &now
		task.FinishedDT = &now
		return task, nil
	}

	go func(id int64) {
		bg := context.Background()
		if _, err := s.Process(bg, id); err != nil {
			utils.Warn("云端录像合并任务执行失败",
				zap.String("task_id", strconv.FormatInt(id, 10)),
				zap.Error(err))
		}
	}(task.ID)

	return task, nil
}

// Process 同步执行合并任务（异步入口与测试共用）。
func (s *GbCloudMergeService) Process(ctx context.Context, id int64) (*model.GbRecordMergeTask, error) {
	task, err := s.repo.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbMergeTaskNotFound
		}
		return nil, err
	}
	if task.Status == model.GbMergeStatusSuccess {
		return task, nil
	}
	if task.Status == model.GbMergeStatusRunning {
		return task, ErrGbMergeTaskRunning
	}

	now := time.Now()
	if err := s.repo.UpdateTaskFields(ctx, id, map[string]interface{}{
		"status":     model.GbMergeStatusRunning,
		"started_dt": now,
		"error_msg":  "",
	}); err != nil {
		return nil, err
	}
	task.Status = model.GbMergeStatusRunning
	task.StartedDT = &now

	fail := func(msg string) (*model.GbRecordMergeTask, error) {
		finished := time.Now()
		_ = s.repo.UpdateTaskFields(ctx, id, map[string]interface{}{
			"status":      model.GbMergeStatusFailed,
			"error_msg":   truncateText(msg, 2000),
			"finished_dt": finished,
		})
		task.Status = model.GbMergeStatusFailed
		task.ErrorMsg = truncateText(msg, 2000)
		task.FinishedDT = &finished
		return task, nil
	}

	ids := parseIDList(task.RecordIDs)
	if len(ids) < 2 {
		return fail("任务中的录像段不足两段")
	}
	records, err := s.repo.GetRecordsByIDs(ctx, ids)
	if err != nil {
		return fail("读取云端录像失败: " + err.Error())
	}
	byID := make(map[int64]model.GbCloudRecord, len(records))
	for _, r := range records {
		byID[r.ID] = r
	}
	ordered := make([]model.GbCloudRecord, 0, len(ids))
	for _, rid := range ids {
		r, ok := byID[rid]
		if !ok {
			return fail(fmt.Sprintf("录像 %d 不存在或已被删除", rid))
		}
		ordered = append(ordered, r)
	}

	ffmpegPath, err := s.ffmpegLookup()
	if err != nil {
		return fail(ErrGbMergeNoFFmpeg.Error())
	}

	// 解析每个录像的绝对路径；节点录像根目录（record_path）用于补全相对路径。
	recordRoots := s.recordRoots(ctx, ordered)
	paths := make([]string, 0, len(ordered))
	var missing []string
	for _, r := range ordered {
		p := resolveCloudRecordPath(r, recordRoots[r.MediaServerID])
		if p == "" {
			missing = append(missing, fmt.Sprintf("录像 %d 未记录文件路径", r.ID))
			continue
		}
		st, statErr := os.Stat(p)
		if statErr != nil || st.IsDir() {
			missing = append(missing, fmt.Sprintf("录像 %d 文件不存在: %s", r.ID, p))
			continue
		}
		paths = append(paths, p)
	}
	if len(missing) > 0 {
		return fail("合并失败：" + strings.Join(missing, "；"))
	}

	outDir := mergeOutputDir()
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fail("创建合并输出目录失败: " + err.Error())
	}
	listFile, err := os.CreateTemp(outDir, ".concat-*.txt")
	if err != nil {
		return fail("创建合并清单失败: " + err.Error())
	}
	listPath := listFile.Name()
	defer os.Remove(listPath)

	for _, p := range paths {
		// ffmpeg concat 清单使用单引号包裹路径；路径内的单引号按 ffmpeg 规则转义。
		escaped := strings.ReplaceAll(p, "'", `'\''`)
		if _, err := fmt.Fprintf(listFile, "file '%s'\n", escaped); err != nil {
			_ = listFile.Close()
			return fail("写入合并清单失败: " + err.Error())
		}
	}
	if err := listFile.Close(); err != nil {
		return fail("写入合并清单失败: " + err.Error())
	}

	fileName := fmt.Sprintf("merge_%d_%s.mp4", task.ID, time.Now().Format("20060102150405"))
	outPath := filepath.Join(outDir, fileName)

	cmdCtx, cancel := context.WithTimeout(ctx, mergeCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, ffmpegPath,
		"-hide_banner", "-nostdin", "-loglevel", "error", "-y",
		"-f", "concat", "-safe", "0",
		"-i", listPath,
		"-c", "copy",
		outPath,
	)
	output, runErr := cmd.CombinedOutput()
	if runErr != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return fail("ffmpeg 合并超时（超过 2 小时）")
		}
		return fail("ffmpeg 合并失败: " + runErr.Error() + "；输出: " + tailText(string(output), mergeErrorTail))
	}

	st, err := os.Stat(outPath)
	if err != nil {
		return fail("合并完成但读取输出文件失败: " + err.Error())
	}

	finished := time.Now()
	if err := s.repo.UpdateTaskFields(ctx, id, map[string]interface{}{
		"status":      model.GbMergeStatusSuccess,
		"file_name":   fileName,
		"file_path":   outPath,
		"file_size":   st.Size(),
		"error_msg":   "",
		"finished_dt": finished,
	}); err != nil {
		return nil, err
	}
	task.Status = model.GbMergeStatusSuccess
	task.FileName = fileName
	task.FilePath = outPath
	task.FileSize = st.Size()
	task.ErrorMsg = ""
	task.FinishedDT = &finished
	return task, nil
}

// Page 合并任务分页。
func (s *GbCloudMergeService) Page(ctx context.Context, req *request.PageGbMergeTaskReq, ds *utils.DataScope) (*response.PageGbMergeTaskResp, error) {
	if req == nil {
		req = &request.PageGbMergeTaskReq{}
	}
	req.Normalize()
	rows, total, err := s.repo.PageTasks(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	items := make([]response.GbMergeTaskItem, 0, len(rows))
	for _, m := range rows {
		items = append(items, toGbMergeTaskItem(m))
	}
	return &response.PageGbMergeTaskResp{
		Items:    items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// Get 合并任务详情。
func (s *GbCloudMergeService) Get(ctx context.Context, id int64) (*response.GbMergeTaskItem, error) {
	m, err := s.repo.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbMergeTaskNotFound
		}
		return nil, err
	}
	item := toGbMergeTaskItem(*m)
	return &item, nil
}

// DownloadPath 返回可下载的合并文件路径与文件名。
func (s *GbCloudMergeService) DownloadPath(ctx context.Context, id int64) (path, name string, err error) {
	m, err := s.repo.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", ErrGbMergeTaskNotFound
		}
		return "", "", err
	}
	if m.Status != model.GbMergeStatusSuccess || trim(m.FilePath) == "" {
		return "", "", ErrGbMergeTaskNotReady
	}
	st, statErr := os.Stat(m.FilePath)
	if statErr != nil || st.IsDir() {
		return "", "", fmt.Errorf("合并文件不存在: %s", m.FilePath)
	}
	return m.FilePath, firstNonEmpty(m.FileName, filepath.Base(m.FilePath)), nil
}

// recordRoots 批量读取录像所属节点的录像根目录。
func (s *GbCloudMergeService) recordRoots(ctx context.Context, records []model.GbCloudRecord) map[string]string {
	roots := make(map[string]string)
	for _, r := range records {
		key := trim(r.MediaServerID)
		if key == "" {
			continue
		}
		if _, ok := roots[key]; ok {
			continue
		}
		ms, err := s.mediaRepo.GetByServerID(ctx, key)
		if err != nil {
			roots[key] = ""
			continue
		}
		roots[key] = trim(ms.RecordPath)
	}
	return roots
}

// resolveCloudRecordPath 把录像记录换算为可访问的本地路径。
func resolveCloudRecordPath(r model.GbCloudRecord, recordRoot string) string {
	p := trim(r.FilePath)
	if p == "" && trim(r.Folder) != "" && trim(r.FileName) != "" {
		p = filepath.Join(trim(r.Folder), trim(r.FileName))
	}
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	if recordRoot != "" {
		return filepath.Clean(filepath.Join(recordRoot, p))
	}
	return filepath.Clean(p)
}

// mergeOutputDir 合并结果目录。
//
// ★ 刻意不放在 media.upload_dir 下：/uploads 是公开静态目录，把录像合并
// 结果放进去等于绕过鉴权暴露录像内容。默认落到工作目录的 data/gb_merge，
// 可用 GB_MERGE_DIR 覆盖（容器部署时挂载持久卷）。
func mergeOutputDir() string {
	if v := strings.TrimSpace(os.Getenv("GB_MERGE_DIR")); v != "" {
		return v
	}
	base := "./data"
	if cfg := bootstrap.Config(); cfg != nil && trim(cfg.Media.UploadDir) != "" {
		// 上传目录的同级目录，避免公开静态路径。
		base = filepath.Dir(trim(cfg.Media.UploadDir))
	}
	return filepath.Join(base, "gb_merge")
}

// ToGbMergeTaskItem 把模型转为响应 DTO（Handler 复用）。
func ToGbMergeTaskItem(m model.GbRecordMergeTask) response.GbMergeTaskItem {
	return toGbMergeTaskItem(m)
}

func toGbMergeTaskItem(m model.GbRecordMergeTask) response.GbMergeTaskItem {
	return response.GbMergeTaskItem{
		ID:          m.ID,
		Name:        m.Name,
		Status:      m.Status,
		RecordIDs:   m.RecordIDs,
		RecordCount: m.RecordCount,
		FileName:    m.FileName,
		FilePath:    m.FilePath,
		FileSize:    m.FileSize,
		DurationSec: m.DurationSec,
		ErrorMsg:    m.ErrorMsg,
		StartedDT:   m.StartedDT,
		FinishedDT:  m.FinishedDT,
		CreatedBy:   m.CreatedBy,
		CreatedDT:   m.CreatedDT,
		UpdatedDT:   m.UpdatedDT,
	}
}

// truncateRunes 按字符截断，避免截断多字节 UTF-8。
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// dedupePositiveIDs 去重并剔除非法 ID。
func dedupePositiveIDs(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// parseIDList 解析逗号分隔的 ID。
func parseIDList(s string) []int64 {
	parts := strings.Split(strings.TrimSpace(s), ",")
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		out = append(out, id)
	}
	return out
}

// truncateText 截断文本（按字节，保留尾部更利于展示 ffmpeg 错误）。
func truncateText(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}

// tailText 保留文本尾部。
func tailText(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}
