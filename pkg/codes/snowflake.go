package codes

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Snowflake 雪花 ID 生成器
type Snowflake struct {
	mu        sync.Mutex // 保证并发安全
	epoch     int64      // 起始时间戳（毫秒）
	lastTime  int64      // 上一次生成 ID 的时间戳
	machineID int64      // 机器 ID（0-1023）
	sequence  int64      // 序列号（0-4095）
}

// NewSnowflake 创建雪花 ID 生成器
// machineID: 机器 ID（必须在 0-1023 之间）
// epoch: 起始时间戳（毫秒，如未指定则默认使用 2023-01-01 00:00:00）
func NewSnowflake(machineID int64, epoch ...int64) (*Snowflake, error) {
	// 校验机器 ID 范围
	if machineID < 0 || machineID > 1023 {
		return nil, errors.New("machineID 必须在 0-1023 之间")
	}

	// 设置默认起始时间（2023-01-01 00:00:00）
	defaultEpoch := int64(1672502400000)
	if len(epoch) > 0 {
		defaultEpoch = epoch[0]
	}

	return &Snowflake{
		epoch:     defaultEpoch,
		machineID: machineID,
	}, nil
}

// Generate 生成一个雪花 ID
func (s *Snowflake) Generate() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 获取当前时间戳（毫秒）
	now := time.Now().UnixMilli()

	// 情况 1：当前时间戳 < 上一次生成 ID 的时间戳（时钟回拨）
	if now < s.lastTime {
		return 0, fmt.Errorf("时钟回拨，拒绝生成 ID，差值: %d 毫秒", s.lastTime-now)
	}

	// 情况 2：同一毫秒内，递增序列号
	if now == s.lastTime {
		s.sequence++
		// 序列号超过最大值（4095），等待下一毫秒
		if s.sequence > 4095 {
			// 阻塞到下一毫秒
			for now <= s.lastTime {
				now = time.Now().UnixMilli()
			}
			s.sequence = 0 // 重置序列号
		}
	} else {
		// 情况 3：不同毫秒，重置序列号
		s.sequence = 0
	}

	// 更新上一次生成 ID 的时间戳
	s.lastTime = now

	// 计算雪花 ID：
	// 时间戳部分：(now - epoch) << 22（左移 10 位机器 ID + 12 位序列号）
	// 机器 ID 部分：machineID << 12（左移 12 位序列号）
	// 序列号部分：sequence
	id := (now-s.epoch)<<22 | (s.machineID << 12) | s.sequence
	return id, nil
}

// Parse 解析雪花 ID 为时间戳、机器 ID、序列号
func (s *Snowflake) Parse(id int64) (timestamp, machineID, sequence int64) {
	timestamp = (id >> 22) + s.epoch
	machineID = (id >> 12) & 0x3FF // 0x3FF 是 10 位掩码（2^10-1）
	sequence = id & 0xFFF          // 0xFFF 是 12 位掩码（2^12-1）
	return
}

// 示例使用
// func main() {
// 	// 创建生成器（机器 ID=1，使用默认起始时间）
// 	sf, err := NewSnowflake(1)
// 	if err != nil {
// 		panic(err)
// 	}

// 	// 并发生成 10 个 ID 测试
// 	var wg sync.WaitGroup
// 	for i := 0; i < 10; i++ {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			id, err := sf.Generate()
// 			if err != nil {
// 				fmt.Println("生成失败:", err)
// 				return
// 			}
// 			fmt.Printf("生成雪花 ID: %d\n", id)
// 		}()
// 	}
// 	wg.Wait()
// }
