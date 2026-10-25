package gb28181

// 推流配置批量导入。
//
// 支持 CSV / TSV，同步逐行校验并落库，不建任务表：
//   - 单次最多 1000 行，文件大小由 Handler 限制为 4MB；
//   - 表头可选（中英文别名），无表头时按固定列序解析；
//   - 每行独立事务语义：某行失败不影响其他行，失败原因随响应返回；
//   - 只支持 CSV/TSV，xlsx 需要额外依赖，本批次刻意不做（文档注明）。

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
)

const (
	// streamImportMaxRows 单次导入行数上限。
	streamImportMaxRows = 1000
	// streamImportDefaultApp 未给出 app 时的缺省值（与国标单端口收流一致）。
	streamImportDefaultApp = "rtp"
)

// 列名别名。key 已经过 normalizeImportHeader 归一化。
var streamImportHeaderAliases = map[string]string{
	"name":          "name",
	"名称":            "name",
	"通道名称":          "name",
	"app":           "app",
	"应用名":           "app",
	"应用":            "app",
	"stream":        "stream",
	"流id":           "stream",
	"流名":            "stream",
	"流名称":           "stream",
	"mediaserverid": "media_server_id",
	"媒体节点":          "media_server_id",
	"节点":            "media_server_id",
	"pushurl":       "push_url",
	"推流地址":          "push_url",
	"gbchannelid":   "gb_channel_id",
	"通道编码":          "gb_channel_id",
	"通道国标编码":        "gb_channel_id",
	"国标编码":          "gb_channel_id",
	"remark":        "remark",
	"备注":            "remark",
}

// 无表头时的列序。
var streamImportDefaultColumns = []string{"name", "app", "stream", "media_server_id", "remark"}

// GbStreamImportService 推流导入服务。
type GbStreamImportService struct {
	repo *repository.GbStreamImportRepo
}

// NewGbStreamImportService 构造导入服务。
func NewGbStreamImportService() *GbStreamImportService {
	return &GbStreamImportService{repo: repository.NewGbStreamImportRepo()}
}

// Import 解析并导入推流配置。
func (s *GbStreamImportService) Import(ctx context.Context, r io.Reader) (*response.GbStreamImportResultResp, error) {
	if r == nil {
		return nil, errors.New("导入文件不能为空")
	}
	data, err := io.ReadAll(io.LimitReader(r, 4<<20+1))
	if err != nil {
		return nil, fmt.Errorf("读取导入文件失败: %w", err)
	}
	if len(data) == 0 {
		return nil, errors.New("导入文件为空")
	}
	if len(data) > 4<<20 {
		return nil, errors.New("导入文件不能超过 4MB")
	}
	// 去掉 UTF-8 BOM（Excel 导出的 CSV 常见）。
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

	records, err := readImportRecords(data)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, errors.New("导入文件没有数据行")
	}

	columns, start := resolveImportColumns(records[0])
	if len(columns) == 0 {
		// 无表头：按固定列序。
		columns = streamImportDefaultColumns
		start = 0
	}
	if len(records)-start > streamImportMaxRows {
		return nil, fmt.Errorf("单次最多导入 %d 行", streamImportMaxRows)
	}

	result := &response.GbStreamImportResultResp{
		Items: []response.GbStreamImportRowError{},
	}
	seen := make(map[string]bool)
	for idx := start; idx < len(records); idx++ {
		row := records[idx]
		line := idx + 1
		if importRowEmpty(row) {
			continue
		}
		result.Total++

		values := make(map[string]string, len(columns))
		for ci, col := range columns {
			if ci < len(row) {
				values[col] = strings.TrimSpace(row[ci])
			}
		}

		app, stream, name, remark, reason := s.resolveImportRow(ctx, values)
		if reason == "" {
			key := app + "\x00" + stream
			if seen[key] {
				reason = "文件内 app+stream 重复"
			} else if exists, err := s.repo.PushExists(ctx, app, stream); err != nil {
				reason = "检查重复失败: " + err.Error()
			} else if exists {
				reason = "该 app+stream 已存在推流配置"
			} else {
				seen[key] = true
			}
		}
		if reason == "" {
			m := &model.GbStreamPush{
				Name:             name,
				App:              app,
				Stream:           stream,
				MediaServerID:    trim(values["media_server_id"]),
				Status:           model.GbStatusOff,
				Pushing:          boolOr(nil, false),
				SelfPush:         boolOr(nil, false),
				StartOfflinePush: boolOr(nil, false),
				Remark:           remark,
				Active:           boolOr(nil, true),
			}
			if err := s.repo.CreatePush(ctx, m); err != nil {
				reason = "保存失败: " + err.Error()
			}
		}

		if reason != "" {
			result.Items = append(result.Items, response.GbStreamImportRowError{
				Row:    result.Total,
				Line:   line,
				Name:   values["name"],
				App:    app,
				Stream: stream,
				Reason: reason,
			})
			continue
		}
		result.SuccessCount++
	}
	result.FailedCount = len(result.Items)
	return result, nil
}

// resolveImportRow 归一化一行并做字段校验，返回空 reason 表示通过。
func (s *GbStreamImportService) resolveImportRow(ctx context.Context, v map[string]string) (app, stream, name, remark, reason string) {
	app = trim(v["app"])
	stream = trim(v["stream"])
	name = trim(v["name"])
	remark = trim(v["remark"])

	// 推流地址优先用于推导 app/stream：地址是用户对「流在哪里」最直接的表达。
	if pushURL := trim(v["push_url"]); pushURL != "" {
		pApp, pStream, err := parsePushURL(pushURL)
		if err != nil {
			return app, stream, name, remark, err.Error()
		}
		if app == "" {
			app = pApp
		}
		if stream == "" {
			stream = pStream
		}
	}

	// 通道编码：补齐名称，并在仍无 stream 时用通道国标编码兜底。
	if gbID := trim(v["gb_channel_id"]); gbID != "" {
		ch, err := s.repo.GetChannelByGbID(ctx, gbID)
		if err != nil {
			return app, stream, name, remark, "通道编码不存在: " + gbID
		}
		if name == "" {
			name = ch.Name
		}
		if stream == "" {
			stream = ch.GbChannelID
		}
	}

	if app == "" {
		app = streamImportDefaultApp
	}
	if name == "" {
		name = stream
	}
	if app == "" || stream == "" {
		return app, stream, name, remark, "app 与 stream 不能为空（需提供 app/stream、推流地址或通道编码）"
	}
	if strings.ContainsAny(app, " \t/\\") {
		return app, stream, name, remark, "app 不能包含空白或斜杠"
	}
	if strings.ContainsAny(stream, " \t/\\") {
		return app, stream, name, remark, "stream 不能包含空白或斜杠"
	}
	return app, stream, name, remark, ""
}

// readImportRecords 按分隔符解析 CSV/TSV。
func readImportRecords(data []byte) ([][]string, error) {
	delim := ','
	if idx := bytes.IndexByte(data, '\n'); idx >= 0 {
		if bytes.ContainsRune(data[:idx], '\t') {
			delim = '\t'
		}
	}
	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = delim
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("解析文件失败（支持 CSV/TSV）: %w", err)
	}
	return records, nil
}

// resolveImportColumns 识别表头；返回列名映射与数据起始下标。
func resolveImportColumns(first []string) ([]string, int) {
	cols := make([]string, len(first))
	recognized := 0
	for i, cell := range first {
		if col, ok := streamImportHeaderAliases[normalizeImportHeader(cell)]; ok {
			cols[i] = col
			recognized++
		}
	}
	if recognized == 0 {
		return nil, 0
	}
	// 表头只要识别出一列就按表头处理；未识别的列置空，后续忽略。
	return cols, 1
}

// normalizeImportHeader 归一化表头：去 BOM、空白、下划线、连字符并小写。
func normalizeImportHeader(s string) string {
	s = strings.TrimPrefix(strings.TrimSpace(s), "\xef\xbb\xbf")
	s = strings.ToLower(s)
	replacer := strings.NewReplacer(" ", "", "\t", "", "_", "", "-", "")
	return replacer.Replace(s)
}

// importRowEmpty 判断整行是否为空。
func importRowEmpty(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

// parsePushURL 从推流地址解析 app / stream。
func parsePushURL(raw string) (string, string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return "", "", errors.New("推流地址格式非法")
	}
	switch strings.ToLower(u.Scheme) {
	case "rtmp", "rtsp", "http", "https":
	default:
		return "", "", errors.New("推流地址协议仅支持 rtmp/rtsp/http/https")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch {
	case len(parts) >= 2:
		return parts[0], trimStreamSuffix(parts[len(parts)-1]), nil
	case len(parts) == 1 && parts[0] != "":
		return streamImportDefaultApp, trimStreamSuffix(parts[0]), nil
	default:
		return "", "", errors.New("推流地址缺少 app/stream 路径")
	}
}

// trimStreamSuffix 去掉播放地址常见的容器后缀。
func trimStreamSuffix(stream string) string {
	for _, suffix := range []string{".live.flv", ".flv", ".m3u8", ".mp4", ".hls"} {
		if strings.HasSuffix(strings.ToLower(stream), suffix) {
			return stream[:len(stream)-len(suffix)]
		}
	}
	return stream
}
