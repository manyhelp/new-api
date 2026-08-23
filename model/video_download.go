package model

import "time"

// VideoDownload 视频本地化下载记录。
// SourceURL 为上游平台地址，属内部保密字段，任何对外响应都不应返回。
type VideoDownload struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	TaskID     string    `json:"task_id" gorm:"index;size:128"`
	UserID     int       `json:"user_id"`
	ChannelID  int       `json:"channel_id"`
	SourceURL  string    `json:"source_url" gorm:"type:text"`
	LocalPath  string    `json:"local_path" gorm:"type:text"`
	PublicURL  string    `json:"public_url" gorm:"type:text"`
	Status     string    `json:"status" gorm:"size:32;index"`
	FileSize   int64     `json:"file_size"`
	MIMEType   string    `json:"mime_type" gorm:"size:128"`
	Error      string    `json:"error" gorm:"type:text"`
	RetryCount int       `json:"retry_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

const (
	VideoDownloadStatusPending     = "pending"
	VideoDownloadStatusDownloading = "downloading"
	VideoDownloadStatusSuccess     = "success"
	VideoDownloadStatusFailed      = "failed"
)

func (VideoDownload) TableName() string { return "video_downloads" }

func CreateVideoDownload(v *VideoDownload) error {
	return DB.Create(v).Error
}

func GetVideoDownloadByTaskID(taskID string) (*VideoDownload, error) {
	var v VideoDownload
	err := DB.Where("task_id = ?", taskID).Order("id desc").First(&v).Error
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func GetVideoDownloadByID(id uint) (*VideoDownload, error) {
	var v VideoDownload
	err := DB.First(&v, id).Error
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func UpdateVideoDownload(v *VideoDownload) error {
	return DB.Save(v).Error
}

func DeleteVideoDownload(id uint) error {
	return DB.Delete(&VideoDownload{}, id).Error
}

func ListVideoDownloads(page, pageSize int, status, taskID string) ([]*VideoDownload, int64, error) {
	q := DB.Model(&VideoDownload{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if taskID != "" {
		q = q.Where("task_id = ?", taskID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []*VideoDownload
	if err := q.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// CountVideoDownloadsByStatus 统计某状态下的记录数（用于并发池判断在途数量）。
func CountVideoDownloadsByStatus(status string) int64 {
	var n int64
	DB.Model(&VideoDownload{}).Where("status = ?", status).Count(&n)
	return n
}

// ClaimPendingVideoDownload 取最早的一条 pending 记录并原子标记为 downloading。
// 单 dispatcher 调用，无需并发竞争保护；返回 nil 表示当前无 pending。
func ClaimPendingVideoDownload() (*VideoDownload, error) {
	var v VideoDownload
	err := DB.Where("status = ?", VideoDownloadStatusPending).Order("id asc").First(&v).Error
	if err != nil {
		return nil, nil // 无 pending（ErrRecordNotFound 也归入无任务）
	}
	res := DB.Model(&VideoDownload{}).
		Where("id = ? AND status = ?", v.ID, VideoDownloadStatusPending).
		Update("status", VideoDownloadStatusDownloading)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}
	v.Status = VideoDownloadStatusDownloading
	return &v, nil
}
