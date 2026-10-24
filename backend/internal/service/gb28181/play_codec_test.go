package gb28181

import (
	"testing"

	"github.com/boticz/ai-camera-backend/internal/gb28181/mediaserver"
)

// TestCodecFromTracks 锁定「优先视频轨、别名归一」的探测规则。
func TestCodecFromTracks(t *testing.T) {
	cases := []struct {
		name   string
		tracks []mediaserver.MediaTrack
		want   string
	}{
		{
			name: "视频轨优先",
			tracks: []mediaserver.MediaTrack{
				{CodecType: 1, CodecIDName: "AAC"},
				{CodecType: 0, CodecIDName: "H264"},
			},
			want: "H264",
		},
		{
			name:   "H265 原样返回",
			tracks: []mediaserver.MediaTrack{{CodecType: 0, CodecIDName: "H265"}},
			want:   "H265",
		},
		{
			name:   "HEVC 别名归一成 H265",
			tracks: []mediaserver.MediaTrack{{CodecType: 0, CodecIDName: "hevc"}},
			want:   "H265",
		},
		{
			name:   "AVC 别名归一成 H264",
			tracks: []mediaserver.MediaTrack{{CodecType: 0, CodecIDName: "AVC"}},
			want:   "H264",
		},
		{
			name:   "仅音频时回落到音频编码",
			tracks: []mediaserver.MediaTrack{{CodecType: 1, CodecIDName: "aac"}},
			want:   "AAC",
		},
		{
			name:   "空列表返回空串",
			tracks: nil,
			want:   "",
		},
		{
			name:   "编码名为空时跳过",
			tracks: []mediaserver.MediaTrack{{CodecType: 0, CodecIDName: "  "}},
			want:   "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := codecFromTracks(c.tracks); got != c.want {
				t.Fatalf("codecFromTracks = %q，期望 %q", got, c.want)
			}
		})
	}
}
