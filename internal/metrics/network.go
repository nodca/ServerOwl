package metrics

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/net"
)

// CollectNetwork 采集网络指标
func (c *defaultCollector) CollectNetwork(ctx context.Context) (*NetworkMetrics, error) {
	ioCounters, err := net.IOCountersWithContext(ctx, true)
	if err != nil {
		return nil, err
	}

	interfaces := make([]NetworkInterface, 0, len(ioCounters))
	for _, counter := range ioCounters {
		interfaces = append(interfaces, NetworkInterface{
			Name:        counter.Name,
			BytesSent:   counter.BytesSent,
			BytesRecv:   counter.BytesRecv,
			PacketsSent: counter.PacketsSent,
			PacketsRecv: counter.PacketsRecv,
			ErrIn:       counter.Errin,
			ErrOut:      counter.Errout,
			DropIn:      counter.Dropin,
			DropOut:     counter.Dropout,
		})
	}

	return &NetworkMetrics{
		Interfaces: interfaces,
		Timestamp:  time.Now(),
	}, nil
}

// GetNetworkIO 获取网络 IO 统计（所有接口汇总）
func GetNetworkIO(ctx context.Context) (bytesSent, bytesRecv uint64, err error) {
	ioCounters, err := net.IOCountersWithContext(ctx, false)
	if err != nil {
		return 0, 0, err
	}
	if len(ioCounters) > 0 {
		return ioCounters[0].BytesSent, ioCounters[0].BytesRecv, nil
	}
	return 0, 0, nil
}

// GetNetworkInterfaces 获取所有网络接口信息
func GetNetworkInterfaces(ctx context.Context) ([]NetworkInterface, error) {
	metrics, err := Default().CollectNetwork(ctx)
	if err != nil {
		return nil, err
	}
	return metrics.Interfaces, nil
}
