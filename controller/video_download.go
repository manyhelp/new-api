package controller

import (
	"os"
	"strconv"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// GetVideoDownloadList 后台「下载列表」：分页 + 按 status/task_id 过滤。
func GetVideoDownloadList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	list, total, err := model.ListVideoDownloads(page, pageSize, c.Query("status"), c.Query("task_id"))
	if err != nil {
		c.JSON(500, gin.H{"message": "list failed", "error": err.Error()})
		return
	}
	// 成功记录的对外地址按当前 BaseURL 实时生成，保证「打开」与查询接口返回一致
	for _, v := range list {
		if v.Status == model.VideoDownloadStatusSuccess {
			v.PublicURL = service.BuildVideoPublicURL(v.TaskID)
		}
	}
	c.JSON(200, gin.H{"success": true, "data": gin.H{"list": list, "total": total}})
}

// RetryVideoDownloadByTask 按 task_id 重新下载（失败/异常文件重试）。
func RetryVideoDownloadByTask(c *gin.Context) {
	taskID := c.Param("task_id")
	if err := service.RetryVideoDownload(taskID); err != nil {
		c.JSON(400, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true})
}

// DeleteVideoDownloadByID 删除下载记录，并联动删除本地文件。
func DeleteVideoDownloadByID(c *gin.Context) {
	id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	id := uint(id64)
	v, err := model.GetVideoDownloadByID(id)
	if err == nil && v != nil && v.LocalPath != "" {
		_ = os.Remove(v.LocalPath)
	}
	if err := model.DeleteVideoDownload(id); err != nil {
		c.JSON(400, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true})
}
