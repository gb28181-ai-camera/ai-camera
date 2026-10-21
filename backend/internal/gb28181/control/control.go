// Package control 定义 HTTP 业务层调用 SIP 信令能力的抽象。
//
// 为什么要有这一层：`cmd/sipsvc`（信令进程）与 `cmd/server`（HTTP 进程）共享同一份
// service 层，但 HTTP 进程并不总是启动 SIP 服务（测试环境、只读部署）。业务代码
// 如果直接 import sipserver，一来会与信令实现强耦合，二来会让两个进程的初始化顺序
// 变成编译期约束。
//
// 因此这里只放接口：sipserver 启动时调用 Set 注入实现，业务层用 Default() 取用，
// 取不到（nil）时按「信令服务未启用」处理 —— 调用方必须判空并返回明确错误，
// 不能静默成功。
package control

import (
	"context"

	"github.com/boticz/ai-camera-backend/internal/model"
)

// SipController 是业务层可用的 SIP 信令操作集合。
//
// 方法均按「设备」维度操作；通道相关报文由调用方自行组装 body（见 protocol/manscdp）。
type SipController interface {
	// QueryCatalog 向设备发送 Catalog 查询（MANSCDP CmdType=Catalog）。
	// 设备会异步返回一条或多条 Catalog 响应，sipserver 内部负责重组并落库。
	QueryCatalog(ctx context.Context, device *model.Device) error

	// Subscribe 发送订阅请求（catalog / alarm / mobile_position）。
	// interval 为订阅间隔（秒），<=0 时使用设备或平台默认值。
	Subscribe(ctx context.Context, device *model.Device, subType string, interval int) error

	// Unsubscribe 取消订阅。
	Unsubscribe(ctx context.Context, device *model.Device, subType string) error

	// SendMessage 发送带外 MESSAGE（PTZ、录像查询、设备控制等）。
	// body 为已按设备 charset 编码好的 UTF-8 XML；编码转换由实现负责。
	SendMessage(ctx context.Context, device *model.Device, body string) error
}

var current SipController

// Set 注入 SIP 控制器实现。仅在信令服务启动时调用一次。
func Set(c SipController) { current = c }

// Default 返回当前注入的控制器；未启用信令服务时为 nil。
func Default() SipController { return current }
