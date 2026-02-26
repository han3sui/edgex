package main

import (
	"fmt"
	"log"
	"time"

	"github.com/simonvetter/modbus"
)

type Point struct {
	id      string
	address uint16
}

func main() {
	client, err := modbus.NewClient(&modbus.ClientConfiguration{
		URL:     "tcp://127.0.0.1:502",
		Timeout: 300 * time.Millisecond,
	})
	if err != nil {
		log.Fatalf("创建Modbus客户端失败: %v", err)
	}
	defer client.Close()

	err = client.Open()
	if err != nil {
		log.Fatalf("打开Modbus连接失败: %v", err)
	}

	err = client.SetUnitId(1)
	if err != nil {
		log.Fatalf("设置从站ID失败: %v", err)
	}

	points := []Point{
		{"hr_40000", 0},
		{"hr_40001", 1},
		{"hr_40002", 2},
		{"hr_40005", 5},
		{"hr_40006", 6},
		{"hr_40007", 7},
		{"hr_40008", 8},
		{"hr_40009", 9},
		{"hr_40187", 187},
		{"hr_40193", 193},
		{"hr_40195", 195},
		{"hr_40199", 199},
	}

	fmt.Println("========================================")
	fmt.Println("Modbus TCP 点位读取测试")
	fmt.Println("========================================")
	fmt.Printf("服务器: tcp://127.0.0.1:502\n")
	fmt.Printf("从站ID: 1\n")
	fmt.Printf("超时: 300ms\n")
	fmt.Println("========================================")

	successCount := 0
	failCount := 0

	for _, point := range points {
		value, err := client.ReadRegister(point.address, modbus.HOLDING_REGISTER)
		if err != nil {
			fmt.Printf("[FAIL] %s (地址 %d): 错误 - %v\n", point.id, point.address, err)
			failCount++
		} else {
			fmt.Printf("[GOOD] %s (地址 %d): 值 = %d (0x%04x)\n", point.id, point.address, value, value)
			successCount++
		}
	}

	fmt.Println("========================================")
	fmt.Printf("测试完成: 成功 %d/%d, 失败 %d/%d\n", successCount, len(points), failCount, len(points))
	fmt.Println("========================================")

	if failCount > 0 {
		fmt.Println("\n异常情况处理建议:")
		fmt.Println("1. 检查Modbus TCP服务器是否正在运行")
		fmt.Println("2. 确认网络连接是否正常")
		fmt.Println("3. 验证从站ID是否正确")
		fmt.Println("4. 检查寄存器地址是否存在")
		fmt.Println("5. 确认防火墙是否阻止了502端口")
	}
}
