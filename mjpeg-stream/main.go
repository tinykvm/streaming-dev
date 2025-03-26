package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/blackjack/webcam"
)

const (
	devicePath = "/dev/video0"
	width      = 1920
	height     = 1080
	port       = 8080
	fps        = 60
)

var (
	frame      []byte
	frameMutex sync.RWMutex
)

func captureFrames() {
	// 打开摄像头
	cam, err := webcam.Open(devicePath)
	if err != nil {
		log.Fatalf("无法打开摄像头设备 %s: %v", devicePath, err)
	}
	defer cam.Close()

	// 获取支持的格式
	formatDesc := cam.GetSupportedFormats()
	log.Println("支持的格式:")
	var mjpegFormat webcam.PixelFormat
	for format, desc := range formatDesc {
		log.Printf("格式: %s (%#x)", desc, format)
		if desc == "Motion-JPEG" || desc == "MJPEG" {
			mjpegFormat = format
			break
		}
	}

	if mjpegFormat == 0 {
		log.Println("未找到MJPEG格式，尝试使用其他格式...")
		// 尝试使用第一个可用格式
		for format := range formatDesc {
			mjpegFormat = format
			break
		}
	}

	// 获取支持的帧大小
	frameSizes := cam.GetSupportedFrameSizes(mjpegFormat)
	log.Println("支持的帧大小:")
	var selectedSize webcam.FrameSize
	for _, size := range frameSizes {
		log.Printf("宽度: %d, 高度: %d", size.MaxWidth, size.MaxHeight)
		if size.MaxWidth == width && size.MaxHeight == height {
			selectedSize = size
			break
		}
	}

	if selectedSize.MaxWidth == 0 {
		log.Println("未找到精确匹配的分辨率，尝试找到最接近的...")
		// 寻找最接近的分辨率
		var bestMatch webcam.FrameSize
		var minDiff uint32 = 0xFFFFFFFF
		
		for _, size := range frameSizes {
			// 计算与目标分辨率的差异
			diff := abs(int(size.MaxWidth) - int(width)) + abs(int(size.MaxHeight) - int(height))
			if uint32(diff) < minDiff {
				minDiff = uint32(diff)
				bestMatch = size
			}
		}
		
		if minDiff != 0xFFFFFFFF {
			selectedSize = bestMatch
		} else if len(frameSizes) > 0 {
			// 如果没有找到任何匹配，使用第一个可用的分辨率
			selectedSize = frameSizes[0]
		}
	}

	if selectedSize.MaxWidth == 0 {
		log.Fatal("无法找到合适的分辨率")
	}

	log.Printf("选择的格式: %s, 分辨率: %dx%d", formatDesc[mjpegFormat], selectedSize.MaxWidth, selectedSize.MaxHeight)

	// 设置摄像头格式
	f, w, h, err := cam.SetImageFormat(mjpegFormat, uint32(selectedSize.MaxWidth), uint32(selectedSize.MaxHeight))
	if err != nil {
		log.Fatalf("设置图像格式失败: %v", err)
	}
	log.Printf("实际设置的格式: %s, 分辨率: %dx%d", formatDesc[f], w, h)

	// 设置缓冲区数量
	err = cam.SetBufferCount(4)
	if err != nil {
		log.Fatalf("设置缓冲区数量失败: %v", err)
	}

	// 开始流传输
	err = cam.StartStreaming()
	if err != nil {
		log.Fatalf("开始流传输失败: %v", err)
	}
	defer cam.StopStreaming()

	// 计算帧间隔
	frameInterval := time.Second / time.Duration(fps)
	timeout := uint32(5) // 5秒超时

	frameCount := 0
	startTime := time.Now()

	// 循环捕获帧
	for {
		// 等待帧
		err = cam.WaitForFrame(timeout)
		if err != nil {
			log.Printf("等待帧超时: %v", err)
			continue
		}

		// 读取帧
		frameData, err := cam.ReadFrame()
		if err != nil {
			log.Printf("读取帧失败: %v", err)
			continue
		}

		if len(frameData) <= 4 {
			continue
		}

		// 更新帧数据
		frameMutex.Lock()
		frame = make([]byte, len(frameData))
		copy(frame, frameData)
		frameMutex.Unlock()

		// 计算帧率
		frameCount++
		if frameCount%30 == 0 {
			elapsed := time.Since(startTime)
			log.Printf("当前帧率: %.2f fps, 帧大小: %d 字节", float64(frameCount)/elapsed.Seconds(), len(frameData))
			frameCount = 0
			startTime = time.Now()
		}

		// 控制帧率
		time.Sleep(frameInterval)
	}
}

// 辅助函数：计算绝对值
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func mjpegHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("新的流连接: %s", r.RemoteAddr)
	
	w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Add("Pragma", "no-cache")
	w.Header().Add("Expires", "0")
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Printf("客户端 %s 不支持流", r.RemoteAddr)
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}
	
	// 检测客户端断开连接
	notify := r.Context().Done()
	go func() {
		<-notify
		log.Printf("客户端断开连接: %s", r.RemoteAddr)
	}()
	
	frameInterval := time.Second / time.Duration(fps)
	
	for {
		select {
		case <-notify:
			return
		default:
			frameMutex.RLock()
			currentFrame := make([]byte, len(frame))
			copy(currentFrame, frame)
			frameMutex.RUnlock()

			if len(currentFrame) <= 4 {
				time.Sleep(frameInterval / 2)
				continue
			}

			_, err := fmt.Fprintf(w, "--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(currentFrame))
			if err != nil {
				log.Printf("写入头部失败: %v", err)
				return
			}
			
			_, err = w.Write(currentFrame)
			if err != nil {
				log.Printf("写入帧数据失败: %v", err)
				return
			}
			
			_, err = fmt.Fprintf(w, "\r\n")
			if err != nil {
				log.Printf("写入结束标记失败: %v", err)
				return
			}
			
			flusher.Flush()
			
			// 使用精确的帧率控制
			time.Sleep(frameInterval)
		}
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/index.html")
}

func main() {
	// 检查设备是否存在
	if _, err := os.Stat(devicePath); os.IsNotExist(err) {
		log.Fatalf("摄像头设备 %s 不存在", devicePath)
	}
	
	// 启动帧捕获协程
	go captureFrames()
	
	// 等待一段时间，确保摄像头初始化完成
	time.Sleep(2 * time.Second)
	
	// 检查是否已经捕获到帧
	frameMutex.RLock()
	hasFrame := len(frame) > 0
	frameMutex.RUnlock()
	
	if !hasFrame {
		log.Println("警告: 尚未捕获到任何帧，但服务器仍将启动")
	}

	// 设置HTTP路由
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/stream", mjpegHandler)

	// 启动HTTP服务器
	log.Printf("启动HTTP服务器在 http://localhost:%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
