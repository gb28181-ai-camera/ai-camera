package sipua

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
)

// TestInboundMessageTriggersSeparateOutboundMessage 锁定 GB28181 最容易写错的一处语义：
//
//	平台发 <Query><CmdType>Catalog</CmdType> → 设备**先回空的 200 OK**，
//	再**另发一条独立 MESSAGE** 携带 <Response><CmdType>Catalog</CmdType>。
//
// 把目录塞进 200 OK 的 body 是发不通的（WVP 的 CatalogQueryMessageHandler 只解独立 MESSAGE）。
// 本测试同时覆盖「一个查询触发多条响应 MESSAGE」（目录按 SumNum 分片）的情形。
func TestInboundMessageTriggersSeparateOutboundMessage(t *testing.T) {
	platform := startFakePlatform(t, testDomain, testPassword, true)
	defer platform.close()

	gotBody := make(chan string, 4)
	gotSource := make(chan string, 4)
	platform.srv.OnMessage(func(req *sip.Request, tx sip.ServerTransaction) {
		gotBody <- string(req.Body())
		gotSource <- req.Source()
		_ = tx.Respond(sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil))
	})

	ua := newTestUA(t, platform.addr, reserveUDPPort(t))
	defer ua.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := ua.Start(ctx); err != nil {
		t.Fatalf("启动 UA 失败：%v", err)
	}

	// 处理器模拟「目录查询 → 回两片目录」，与主程序 messageDispatcher 的行为一致。
	ua.SetMessageHandler(func(body []byte, contentType string) [][]byte {
		if !strings.Contains(string(body), "Catalog") {
			t.Errorf("入向正文不符：%s", body)
		}
		return [][]byte{
			[]byte(`<?xml version="1.0" encoding="GB2312"?><Response><CmdType>Catalog</CmdType><SN>1</SN><SumNum>2</SumNum><DeviceList Num="1"><Item><DeviceID>34020000001310000001</DeviceID></Item></DeviceList></Response>`),
			[]byte(`<?xml version="1.0" encoding="GB2312"?><Response><CmdType>Catalog</CmdType><SN>1</SN><SumNum>2</SumNum><DeviceList Num="1"><Item><DeviceID>34020000001310000002</DeviceID></Item></DeviceList></Response>`),
		}
	})

	// 平台向设备发目录查询
	query := []byte(`<?xml version="1.0" encoding="GB2312"?><Query><CmdType>Catalog</CmdType><SN>1</SN><DeviceID>` + testDeviceID + `</DeviceID></Query>`)
	req := sip.NewRequest(sip.MESSAGE, sip.Uri{
		Scheme: "sip", User: testDeviceID, Host: "127.0.0.1", Port: ua.cfg.LocalPort,
	})
	req.AppendHeader(&sip.FromHeader{Address: sip.Uri{Scheme: "sip", User: "34020000002000000001", Host: testDomain}})
	req.AppendHeader(&sip.ToHeader{Address: sip.Uri{Scheme: "sip", User: testDeviceID, Host: testDomain}})
	req.AppendHeader(sip.NewHeader("Content-Type", ContentTypeMANSCDP))
	req.SetBody(query)
	req.SetTransport("UDP")

	client, err := sipgo.NewClient(platform.ua)
	if err != nil {
		t.Fatalf("构造平台客户端失败：%v", err)
	}
	if _, err := client.Do(ctx, req); err != nil {
		t.Fatalf("下发目录查询失败：%v", err)
	}

	// 设备应回两条独立的目录响应 MESSAGE
	var bodies []string
	for i := 0; i < 2; i++ {
		select {
		case got := <-gotBody:
			bodies = append(bodies, got)
			src := <-gotSource
			_, port, _ := net.SplitHostPort(src)
			if port != strconv.Itoa(ua.cfg.LocalPort) {
				t.Fatalf("第 %d 条响应 MESSAGE 的源端口 %s 应等于设备监听端口 %d",
					i, port, ua.cfg.LocalPort)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("只收到 %d 条响应 MESSAGE，期望 2 条", len(bodies))
		}
	}

	joined := strings.Join(bodies, "")
	if !strings.Contains(joined, "34020000001310000001") || !strings.Contains(joined, "34020000001310000002") {
		t.Fatalf("响应 MESSAGE 未包含全部通道：%s", joined)
	}
	if strings.Count(joined, "<Response>") != 2 {
		t.Fatalf("应回两条独立的 Response 报文：%s", joined)
	}
	if strings.Contains(joined, "<Query>") {
		t.Fatalf("响应里不应回带 Query 根元素：%s", joined)
	}
}

// TestInboundMessageHandlerReturningNil 处理器返回 nil 时只回 200 OK，不产生多余 MESSAGE。
func TestInboundMessageHandlerReturningNil(t *testing.T) {
	platform := startFakePlatform(t, testDomain, testPassword, true)
	defer platform.close()

	unexpected := make(chan string, 1)
	platform.srv.OnMessage(func(req *sip.Request, tx sip.ServerTransaction) {
		// 平台发出的查询自己不会收到；这里只在设备误发响应时触发
		select {
		case unexpected <- string(req.Body()):
		default:
		}
		_ = tx.Respond(sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil))
	})

	ua := newTestUA(t, platform.addr, reserveUDPPort(t))
	defer ua.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := ua.Start(ctx); err != nil {
		t.Fatalf("启动 UA 失败：%v", err)
	}
	ua.SetMessageHandler(func(body []byte, contentType string) [][]byte { return nil })

	req := sip.NewRequest(sip.MESSAGE, sip.Uri{
		Scheme: "sip", User: testDeviceID, Host: "127.0.0.1", Port: ua.cfg.LocalPort,
	})
	req.AppendHeader(&sip.FromHeader{Address: sip.Uri{Scheme: "sip", User: "34020000002000000001", Host: testDomain}})
	req.AppendHeader(&sip.ToHeader{Address: sip.Uri{Scheme: "sip", User: testDeviceID, Host: testDomain}})
	req.AppendHeader(sip.NewHeader("Content-Type", ContentTypeMANSCDP))
	req.SetBody([]byte(`<Notify><CmdType>Keepalive</CmdType></Notify>`))
	req.SetTransport("UDP")

	client, err := sipgo.NewClient(platform.ua)
	if err != nil {
		t.Fatalf("构造平台客户端失败：%v", err)
	}
	if _, err := client.Do(ctx, req); err != nil {
		t.Fatalf("下发 MESSAGE 失败：%v", err)
	}

	select {
	case body := <-unexpected:
		t.Fatalf("处理器返回 nil 时不应发出 MESSAGE，实际收到：%s", body)
	case <-time.After(500 * time.Millisecond):
	}
}
