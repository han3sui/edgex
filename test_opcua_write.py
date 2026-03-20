from opcua import Client
import time

# OPC UA服务器地址
url = "opc.tcp://localhost:5050/test"

# 测试数据点
TEST_POINTS = [
    {"name": "HR 40001", "type": "Int16", "value": 11},
    {"name": "HR 40002", "type": "Int32", "value": 12345},
    {"name": "HR 40011", "type": "Float64", "value": 123.45}
]

def test_opcua_write():
    print("===== OPC UA写入功能测试 =====")
    print(f"连接到OPC UA服务器: {url}")
    
    try:
        # 创建客户端并连接
        client = Client(url)
        client.set_user("admin")
        client.set_password("admin")
        client.connect()
        print("连接成功")
        # 激活会话
        client.activate_session()
        
        # 获取根节点
        root = client.get_root_node()
        print("获取根节点成功")
        
        # 遍历所有设备和数据点
        objects = root.get_child(["0:Objects"])
        gateway = objects.get_child(["2:Gateway"])
        channels = gateway.get_child(["2:Channels"])
        
        # 遍历所有通道
        channel_names = channels.get_children()
        print(f"找到 {len(channel_names)} 个通道")
        
        for channel_node in channel_names:
            channel_name = channel_node.get_browse_name().Name
            print(f"\n通道: {channel_name}")
            
            # 获取设备
            devices = channel_node.get_child(["2:Devices"])
            device_names = devices.get_children()
            print(f"  找到 {len(device_names)} 个设备")
            
            for device_node in device_names:
                device_name = device_node.get_browse_name().Name
                print(f"  设备: {device_name}")
                
                # 获取数据点
                points = device_node.get_child(["2:Points"])
                point_names = points.get_children()
                print(f"    找到 {len(point_names)} 个数据点")
                
                # 测试写入
                for test_point in TEST_POINTS:
                    for point_node in point_names:
                        point_name = point_node.get_browse_name().Name
                        if point_name == test_point["name"]:
                            print(f"\n测试写入: {point_name} ({test_point['type']}) = {test_point['value']}")
                            try:
                                # 写入值
                                point_node.set_value(test_point['value'])
                                print(f"  写入成功")
                                
                                # 读取回值以验证
                                read_value = point_node.get_value()
                                print(f"  读取验证: {read_value}")
                                print(f"  测试结果: {'通过' if read_value == test_point['value'] else '失败'}")
                            except Exception as e:
                                print(f"  写入失败: {e}")
        
        # 测试完成后断开连接
        client.disconnect()
        print("\n===== 测试完成 =====")
        
    except Exception as e:
        print(f"错误: {e}")

if __name__ == "__main__":
    test_opcua_write()
