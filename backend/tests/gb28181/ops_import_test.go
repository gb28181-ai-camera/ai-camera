package gb28181_test

// ════════════════════════════════════════════════════════════════
// 运维增强：推流配置批量导入（CSV / TSV）
//
// 覆盖：表头识别、无表头列序、通道编码补齐、推流地址解析、
// 文件内重复、库内重复、缺字段与 TSV 分隔符。
// ════════════════════════════════════════════════════════════════

import (
	"context"
	"strings"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/model"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

func TestOpsStreamPushImportCSV(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		dev := createDevice(t, ctx, "CAM_IMPORT_001", "导入设备", "34020000001320008001")
		createChannel(t, ctx, dev, "34020000001320008001", "导入通道", "")

		// 先插入一条已存在的推流配置，验证库内重复。
		exist := &model.GbStreamPush{Name: "已有", App: "rtp", Stream: "exists", Status: model.GbStatusOff}
		if err := tx.Create(exist).Error; err != nil {
			t.Fatalf("预置推流配置失败: %v", err)
		}

		csv := "name,app,stream,media_server_id,remark\n" +
			"导入一,rtp,cam1,zlm-1,ok\n" +
			"导入二,rtp,cam1,,文件内重复\n" +
			"重复库,rtp,exists,,库内已存在\n" +
			"bad,rtp,cam3\n" // 库内不存在，应成功（列数不足按空值处理）

		svc := gbsvc.NewGbStreamImportService()
		res, err := svc.Import(ctx, strings.NewReader(csv))
		if err != nil {
			t.Fatalf("导入 CSV 失败: %v", err)
		}
		if res.Total != 4 || res.SuccessCount != 2 || res.FailedCount != 2 {
			t.Fatalf("导入统计不正确: %+v", res)
		}
		reasons := ""
		for _, item := range res.Items {
			reasons += item.Reason + "|"
		}
		if !strings.Contains(reasons, "文件内 app+stream 重复") {
			t.Fatalf("应报告文件内重复，实际 %s", reasons)
		}
		if !strings.Contains(reasons, "已存在推流配置") {
			t.Fatalf("应报告库内重复，实际 %s", reasons)
		}

		var count int64
		if err := tx.Model(&model.GbStreamPush{}).Where("app = ? AND stream IN ?", "rtp", []string{"cam1", "cam3"}).Count(&count).Error; err != nil {
			t.Fatalf("统计导入结果失败: %v", err)
		}
		if count != 2 {
			t.Fatalf("应导入 2 条推流配置，实际 %d", count)
		}
	})
}

func TestOpsStreamPushImportChannelAndPushURL(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		dev := createDevice(t, ctx, "CAM_IMPORT_002", "导入设备2", "34020000001320008002")
		ch := createChannel(t, ctx, dev, "34020000001320008002", "东门通道", "")

		// 表头别名 + 通道编码 + 推流地址（TSV）。
		tsv := "通道编码\t流名称\t推流地址\t备注\n" +
			ch.GbChannelID + "\t\trtmp://127.0.0.1:1935/live/cam9\t通道+地址\n" +
			"gbs-missing\tchan2\t\t不存在通道\n"

		svc := gbsvc.NewGbStreamImportService()
		res, err := svc.Import(ctx, strings.NewReader(tsv))
		if err != nil {
			t.Fatalf("导入 TSV 失败: %v", err)
		}
		if res.Total != 2 || res.SuccessCount != 1 || res.FailedCount != 1 {
			t.Fatalf("TSV 导入统计不正确: %+v", res)
		}
		if !strings.Contains(res.Items[0].Reason, "通道编码不存在") {
			t.Fatalf("应报告通道编码不存在，实际 %+v", res.Items[0])
		}

		var got model.GbStreamPush
		if err := tx.Where("app = ? AND stream = ?", "live", "cam9").First(&got).Error; err != nil {
			t.Fatalf("未找到从推流地址解析出的配置: %v", err)
		}
		if got.Name != "东门通道" || got.MediaServerID != "" {
			t.Fatalf("通道名称/节点字段不正确: %+v", got)
		}
	})
}

func TestOpsStreamPushImportNoHeaderAndErrors(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := gbsvc.NewGbStreamImportService()

		// 无表头：name,app,stream,media_server_id,remark
		res, err := svc.Import(ctx, strings.NewReader("无表头,rtp,nh1,zlm-1,ok\n"))
		if err != nil {
			t.Fatalf("无表头导入失败: %v", err)
		}
		if res.SuccessCount != 1 {
			t.Fatalf("无表头应导入 1 条，实际 %+v", res)
		}

		// 无表头且缺少 stream 列（列序 name,app,stream,...）-> 失败行。
		res, err = svc.Import(ctx, strings.NewReader("只有名称,rtp\n"))
		if err != nil {
			t.Fatalf("缺字段导入失败: %v", err)
		}
		if res.SuccessCount != 0 || res.FailedCount != 1 || !strings.Contains(res.Items[0].Reason, "app 与 stream 不能为空") {
			t.Fatalf("缺字段行应失败，实际 %+v", res)
		}

		// 非法推流地址。
		res, err = svc.Import(ctx, strings.NewReader("name,push_url\n坏地址,ftp://host/a/b\n"))
		if err != nil {
			t.Fatalf("非法地址导入失败: %v", err)
		}
		if res.FailedCount != 1 || !strings.Contains(res.Items[0].Reason, "协议") {
			t.Fatalf("非法协议应失败，实际 %+v", res)
		}

		// 空文件。
		if _, err := svc.Import(ctx, strings.NewReader("")); err == nil {
			t.Fatalf("空文件应返回错误")
		}
	})
}
