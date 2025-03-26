# MJPEG 摄像头流服务器
这是一个基于Go语言的MJPEG摄像头流服务器，可以通过网页浏览器实时查看摄像头画面。

## 功能特点
- 自动检测摄像头支持的格式和分辨率
- 使用MJPEG格式进行视频流传输
- 提供简洁的网页界面实时显示摄像头画面
- 支持自动重连和连接状态显示
- 高效的帧率控制和缓冲区管理

## 系统要求
- Linux操作系统（已测试在Ubuntu和Debian上）
- 支持V4L2的摄像头设备
- Go 1.18或更高版本

## 安装
克隆仓库：
```bash
git clone https://github.com/tinykvm/streaming-dev.git
cd streaming-dev
cd mjpeg-stream
```
安装依赖：
```bash
go mod tidy
```
编译程序：
```bash
go build
```
使用方法
1. 确保摄像头已连接并被系统识别：
```bash
ls /dev/video*
```
2. 运行程序：
```bash
./mjpeg-stream
```
3. 打开浏览器访问：
```bash
http://localhost:8080
```

# 配置
在main.go文件中，您可以修改以下常量来自定义配置：
```go
const (
    devicePath = "/dev/video0"  // 摄像头设备路径
    width      = 640            // 目标宽度
    height     = 480            // 目标高度
    port       = 8080           // HTTP服务器端口
    fps        = 30             // 目标帧率
)
```

# 故障排除
1. 找不到摄像头设备：
- 确认摄像头已正确连接
- 检查设备路径是否正确（可能是/dev/video1或其他）
- 确保当前用户有权限访问摄像头设备
2. 无法显示视频流：
- 检查摄像头是否支持MJPEG格式
- 尝试降低分辨率和帧率
- 检查网络连接是否正常
3. 帧率过低：
- 检查摄像头支持的最大帧率
- 降低分辨率可能会提高帧率
- 确保系统资源充足

# 项目结构
```bash
mjpeg-stream/
├── main.go         # 主程序代码
├── go.mod          # Go模块定义
├── go.sum          # 依赖版本锁定
└── static/         # 静态文件目录
    └── index.html  # 网页界面
```
# 依赖项
- github.com/blackjack/webcam - Go语言V4L2接口库
- golang.org/x/sys - 系统调用库

# 许可证
MIT License

