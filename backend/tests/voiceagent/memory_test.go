package voiceagent_test

import (
	"context"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/service/voiceagent"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

func TestMemoryWriteDedupeAndIsolation(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := voiceagent.NewMemoryService(&staticProvider{cfg: testConfig()})

		item, dedup, err := svc.Write(ctx, &request.MemoryWriteReq{
			OwnerType: "user", OwnerID: 101, Kind: "preference",
			Content: "用户喜欢安静的环境", Importance: 4,
		})
		if err != nil {
			t.Fatalf("写入失败: %v", err)
		}
		if dedup {
			t.Fatal("首次写入不应去重")
		}
		if item.EmbeddingStatus != model.VoiceEmbeddingFailed {
			t.Errorf("未配置 embedding 时状态应为 failed，实际 %q", item.EmbeddingStatus)
		}
		if item.OwnerKey != "user:101" {
			t.Errorf("owner_key = %q", item.OwnerKey)
		}

		// 完全相同内容 → 去重
		item2, dedup, err := svc.Write(ctx, &request.MemoryWriteReq{
			OwnerType: "user", OwnerID: 101, Content: "用户喜欢安静的环境",
		})
		if err != nil {
			t.Fatalf("重复写入失败: %v", err)
		}
		if !dedup || item2.ID != item.ID {
			t.Errorf("应去重到同一记录: dedup=%v id=%d/%d", dedup, item2.ID, item.ID)
		}

		// 另一用户同名内容 → 隔离
		other, dedup, err := svc.Write(ctx, &request.MemoryWriteReq{
			OwnerType: "user", OwnerID: 202, Content: "用户喜欢安静的环境",
		})
		if err != nil {
			t.Fatalf("其他用户写入失败: %v", err)
		}
		if dedup || other.ID == item.ID {
			t.Errorf("不同 owner 不应去重: dedup=%v id=%d/%d", dedup, other.ID, item.ID)
		}

		// 检索隔离
		list, err := svc.Retrieve(ctx, &request.MemoryRetrieveReq{
			OwnerKeys: []string{"user:101"}, Query: "安静",
		})
		if err != nil {
			t.Fatalf("检索失败: %v", err)
		}
		if len(list) != 1 || list[0].OwnerKey != "user:101" {
			t.Fatalf("检索隔离失败: %+v", list)
		}

		// 不存在的归属
		empty, err := svc.Retrieve(ctx, &request.MemoryRetrieveReq{OwnerKeys: []string{"user:999"}})
		if err != nil {
			t.Fatalf("空检索失败: %v", err)
		}
		if len(empty) != 0 {
			t.Errorf("不存在的归属应返回空，实际 %+v", empty)
		}
	})
}

func TestMemoryVectorRetrieveAndDedupe(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := voiceagent.NewMemoryService(&staticProvider{cfg: testConfig()})
		vecA := makeVector(0)
		vecB := makeVector(1)

		a, _, err := svc.Write(ctx, &request.MemoryWriteReq{
			OwnerType: "global", Content: "设备大厅有一台摄像机", Embedding: vecA,
		})
		if err != nil {
			t.Fatalf("写入 A 失败: %v", err)
		}
		if a.EmbeddingStatus != model.VoiceEmbeddingOK {
			t.Fatalf("带向量写入状态应为 ok，实际 %q", a.EmbeddingStatus)
		}
		if _, _, err := svc.Write(ctx, &request.MemoryWriteReq{
			OwnerType: "global", Content: "主楼门口有报警器", Embedding: vecB,
		}); err != nil {
			t.Fatalf("写入 B 失败: %v", err)
		}

		// 余弦检索：查询向量接近 A 时应先命中 A。
		list, err := svc.Retrieve(ctx, &request.MemoryRetrieveReq{
			OwnerKeys: []string{"global"}, TopK: 5, Embedding: vecA,
		})
		if err != nil {
			t.Fatalf("向量检索失败: %v", err)
		}
		if len(list) != 2 || list[0].ID != a.ID {
			t.Fatalf("向量排序异常: %+v", list)
		}

		// 语义近似去重：不同内容 + 相同向量 → 去重
		dup, dedup, err := svc.Write(ctx, &request.MemoryWriteReq{
			OwnerType: "global", Content: "大厅里放了摄像机设备", Embedding: vecA,
		})
		if err != nil {
			t.Fatalf("语义重复写入失败: %v", err)
		}
		if !dedup || dup.ID != a.ID {
			t.Errorf("余弦 >0.90 应去重: dedup=%v id=%d/%d", dedup, dup.ID, a.ID)
		}
	})
}

func TestMemoryAdminDeleteAndExport(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		mem := voiceagent.NewMemoryService(&staticProvider{cfg: testConfig()})
		admin := voiceagent.NewAdminService()
		item, _, err := mem.Write(ctx, &request.MemoryWriteReq{
			OwnerType: "user", OwnerID: 7, Kind: "fact", Content: "用户住在杭州",
		})
		if err != nil {
			t.Fatal(err)
		}

		ds := &utils.DataScope{UserID: 7, DataPerm: "all"}
		page, err := admin.PageMemories(ctx, &request.PageVoiceMemoryReq{PageReq: request.PageReq{Page: 1, PageSize: 20}}, ds)
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if page.Total != 1 || len(page.Items) != 1 {
			t.Fatalf("分页结果异常: %+v", page)
		}

		export, err := admin.ExportMemories(ctx, &request.PageVoiceMemoryReq{}, ds)
		if err != nil || len(export) != 1 {
			t.Fatalf("导出异常: err=%v len=%d", err, len(export))
		}

		if err := admin.DeleteMemory(ctx, item.ID, ds); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		page, err = admin.PageMemories(ctx, &request.PageVoiceMemoryReq{PageReq: request.PageReq{Page: 1, PageSize: 20}}, ds)
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 0 {
			t.Errorf("删除后仍有 %d 条", page.Total)
		}
	})
}
