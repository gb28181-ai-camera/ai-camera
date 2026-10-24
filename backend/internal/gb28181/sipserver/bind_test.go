package sipserver

import (
	"context"
	"errors"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/emiago/sipgo/sip"
)

type fakeBinder struct {
	product, device, gbID string
	dev                   *model.Device
	err                   error
	called                bool
}

func (f *fakeBinder) BindBySIP(_ context.Context, productCode, deviceCode, gbDeviceID string) (*model.Device, error) {
	f.called = true
	f.product, f.device, f.gbID = productCode, deviceCode, gbDeviceID
	return f.dev, f.err
}

// TestTryBindBySIP 只有同时带设备编码与产品编码时才调用 binder。
func TestTryBindBySIP(t *testing.T) {
	newReq := func(deviceCode, productCode string) *sip.Request {
		req := sip.NewRequest(sip.REGISTER, sip.Uri{})
		if deviceCode != "" {
			req.AppendHeader(sip.NewHeader("X-Device-Code", deviceCode))
		}
		if productCode != "" {
			req.AppendHeader(sip.NewHeader("X-Product-Code", productCode))
		}
		return req
	}

	dev := &model.Device{BaseModel: model.BaseModel{ID: 7}, GbDeviceID: "34020000001320000001"}
	binder := &fakeBinder{dev: dev}
	svc := &Service{}
	svc.SetDeviceBinder(binder)

	got, err := svc.tryBindBySIP(context.Background(), newReq("CAM_001", "PROD_001"), dev.GbDeviceID)
	if err != nil || got != dev {
		t.Fatalf("应返回绑定后的设备: got=%v err=%v", got, err)
	}
	if !binder.called || binder.product != "PROD_001" || binder.device != "CAM_001" || binder.gbID != dev.GbDeviceID {
		t.Fatalf("binder 参数不正确: %+v", binder)
	}

	// 只带一个头：不触发绑定
	binder.called = false
	got, err = svc.tryBindBySIP(context.Background(), newReq("CAM_001", ""), dev.GbDeviceID)
	if got != nil || err != nil || binder.called {
		t.Fatalf("缺少产品编码时不应绑定: got=%v err=%v called=%v", got, err, binder.called)
	}

	// 未注入 binder：静默返回 nil,nil
	svc2 := &Service{}
	got, err = svc2.tryBindBySIP(context.Background(), newReq("CAM_001", "PROD_001"), dev.GbDeviceID)
	if got != nil || err != nil {
		t.Fatalf("未注入 binder 时应返回 nil,nil: got=%v err=%v", got, err)
	}

	// binder 返回错误：原样透传（调用方据此写拒绝日志）
	errBinder := &fakeBinder{err: errors.New("产品不匹配")}
	svc3 := &Service{}
	svc3.SetDeviceBinder(errBinder)
	if _, err := svc3.tryBindBySIP(context.Background(), newReq("CAM_001", "PROD_001"), dev.GbDeviceID); err == nil {
		t.Fatal("binder 错误应透传")
	}
}
