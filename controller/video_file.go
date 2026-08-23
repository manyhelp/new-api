package controller

import (
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// ServeVideoFile 对外本地视频直链：/api/videos/<task_id>.mp4
// 无需令牌；URL 带 .mp4 后缀，浏览器会按 video/mp4 识别、用正确文件名下载/播放，支持 Range。
func ServeVideoFile(c *gin.Context) {
	// 兼容带或不带 .mp4 后缀的请求
	taskID := strings.TrimSuffix(c.Param("task_id"), ".mp4")

	v, err := model.GetVideoDownloadByTaskID(taskID)
	if err != nil || v == nil || v.Status != model.VideoDownloadStatusSuccess || v.LocalPath == "" {
		c.String(404, "file not ready")
		return
	}

	// 防路径穿越：本地路径必须位于存储目录内。
	abs, err := filepath.Abs(v.LocalPath)
	if err != nil {
		c.String(404, "file not found")
		return
	}
	dir := filepath.Clean(service.VideoLocalizeDirAbs())
	if !strings.HasPrefix(abs+string(filepath.Separator), dir+string(filepath.Separator)) {
		c.String(403, "forbidden")
		return
	}

	// FileAttachment 设置 Content-Disposition: attachment，浏览器访问会直接下载（文件名取 <task_id>.mp4）；
	// 底层仍是 http.ServeFile，支持 Range/断点续传。
	c.FileAttachment(abs, filepath.Base(abs))
}
