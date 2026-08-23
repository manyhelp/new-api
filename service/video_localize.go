package service

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

// 视频本地化：把上游平台返回的视频文件下载到本地，对外只暴露本地（网关）直链地址，
// 从根本上避免泄露上游 OSS 域名/路径等平台信息。
// 对外地址形如：https://<网关>/api/videos/<task_id>.mp4（干净直链，浏览器可直接下载/播放）。

const videoFileRoute = "/api/videos"

// StartVideoLocalize 启动下载调度器与清理协程（进程级单次调用）。
func StartVideoLocalize() {
	go videoLocalizeDispatcher()
	go videoLocalizeCleanup()
}

// ShouldLocalizeVideo 视频本地化总开关。
func ShouldLocalizeVideo() bool { return setting.VideoLocalizeEnabledBool() }

// EnqueueVideoDownload 为任务创建 pending 下载记录（幂等：同 task_id 已存在则跳过）。
func EnqueueVideoDownload(taskID string, userID, channelID int, sourceURL string) {
	if existing, err := model.GetVideoDownloadByTaskID(taskID); err == nil && existing != nil {
		return
	}
	v := &model.VideoDownload{
		TaskID:    taskID,
		UserID:    userID,
		ChannelID: channelID,
		SourceURL: sourceURL,
		Status:    model.VideoDownloadStatusPending,
	}
	if err := model.CreateVideoDownload(v); err != nil {
		common.SysError("video_localize: enqueue failed for " + taskID + ": " + err.Error())
	}
}

// RetryVideoDownload 重置失败记录为 pending，等待重新下载。
func RetryVideoDownload(taskID string) error {
	v, err := model.GetVideoDownloadByTaskID(taskID)
	if err != nil || v == nil {
		return fmt.Errorf("download record not found for task %s", taskID)
	}
	v.Status = model.VideoDownloadStatusPending
	v.Error = ""
	return model.UpdateVideoDownload(v)
}

func videoLocalizeDispatcher() {
	for {
		concurrency := setting.VideoLocalizeConcurrencyInt()
		inflight := model.CountVideoDownloadsByStatus(model.VideoDownloadStatusDownloading)
		for i := inflight; i < int64(concurrency); i++ {
			rec, err := model.ClaimPendingVideoDownload()
			if err != nil || rec == nil {
				break
			}
			go downloadOne(rec)
		}
		time.Sleep(3 * time.Second)
	}
}

func downloadOne(v *model.VideoDownload) {
	dir := VideoLocalizeDirAbs()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		finishVideoDownloadFail(v, err)
		return
	}
	tmpPath := filepath.Join(dir, v.TaskID+".mp4.tmp")
	finalPath := filepath.Join(dir, v.TaskID+".mp4")

	client := &http.Client{Timeout: setting.VideoLocalizeTimeoutDur()}
	req, err := http.NewRequest(http.MethodGet, v.SourceURL, nil)
	if err != nil {
		finishVideoDownloadFail(v, err)
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		finishVideoDownloadFail(v, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		finishVideoDownloadFail(v, fmt.Errorf("upstream status %d", resp.StatusCode))
		return
	}
	mime := resp.Header.Get("Content-Type")
	f, err := os.Create(tmpPath)
	if err != nil {
		finishVideoDownloadFail(v, err)
		return
	}
	n, err := io.Copy(f, resp.Body)
	_ = f.Close()
	if err != nil {
		_ = os.Remove(tmpPath)
		finishVideoDownloadFail(v, err)
		return
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		finishVideoDownloadFail(v, err)
		return
	}

	v.LocalPath = finalPath
	v.FileSize = n
	v.MIMEType = mime
	v.PublicURL = BuildVideoPublicURL(v.TaskID)
	v.Status = model.VideoDownloadStatusSuccess
	v.Error = ""
	if err := model.UpdateVideoDownload(v); err != nil {
		common.SysError("video_localize: mark success failed: " + err.Error())
	}
}

func finishVideoDownloadFail(v *model.VideoDownload, err error) {
	v.Status = model.VideoDownloadStatusFailed
	v.Error = err.Error()
	v.RetryCount++
	_ = model.UpdateVideoDownload(v)
}

// VideoLocalizeDirAbs 返回本地存储目录的绝对路径（含路径穿越校验基线）。
func VideoLocalizeDirAbs() string {
	abs, err := filepath.Abs(setting.VideoLocalizeDirStr())
	if err != nil {
		return setting.VideoLocalizeDirStr()
	}
	return abs
}

// BuildVideoPublicURL 生成对外的本地直链地址（带 .mp4 后缀，浏览器可直接下载/播放）。
// 优先用配置的公网 BaseURL，为空则回退系统 ServerAddress；每次调用按当前配置实时生成。
func BuildVideoPublicURL(taskID string) string {
	base := strings.TrimRight(setting.VideoLocalizePublicBaseURLStr(), "/")
	if base == "" {
		base = strings.TrimRight(system_setting.ServerAddress, "/")
	}
	return fmt.Sprintf("%s%s/%s.mp4", base, videoFileRoute, taskID)
}

// safeReqParamNames 允许回显给用户的请求参数白名单（剔除图片URL等可能含平台信息的字段）。
var safeReqParamNames = map[string]bool{
	"prompt":         true,
	"model_name":     true,
	"duration":       true,
	"mode":           true,
	"aspect_ratio":   true,
	"generate_audio": true,
	"tools":          true,
}

// sanitizeVideoTaskData 从上游原始响应里只提取白名单内的请求参数，拼成 {request_params:[...]}。
// 这样既能让用户看到 720p/5s 等请求信息，又不泄露平台 OSS 地址、appId、计费等内部数据。
func sanitizeVideoTaskData(raw []byte) []byte {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if err := common.Unmarshal(raw, &m); err != nil {
		return nil
	}
	r1, _ := m["result"].(map[string]any)
	inner, _ := r1["result"].(map[string]any)
	rawParam, ok := inner["reqParam"].([]any)
	if !ok {
		return nil
	}
	clean := make([]any, 0, len(rawParam))
	for _, p := range rawParam {
		pm, ok := p.(map[string]any)
		if !ok {
			continue
		}
		if name, _ := pm["filedName"].(string); safeReqParamNames[name] {
			clean = append(clean, pm)
		}
	}
	if len(clean) == 0 {
		return nil
	}
	out, err := common.Marshal(map[string]any{"request_params": clean})
	if err != nil {
		return nil
	}
	return out
}

// ApplyVideoLocalizeGating 按本地化状态改写对外任务 DTO：
//  1. data 只回显脱敏后的请求参数；
//  2. 没下完 → 对外不为 SUCCESS、不返回 URL；
//  3. 已下完 → 对外 SUCCESS + 本地直链；
//  4. 失败 → 对外 FAILURE。
//
// 仅当本地化开关开启时生效。
func ApplyVideoLocalizeGating(task *model.Task, d *dto.TaskDto) {
	if !ShouldLocalizeVideo() {
		return
	}
	d.Data = sanitizeVideoTaskData(task.Data)

	v, err := model.GetVideoDownloadByTaskID(task.TaskID)
	if err != nil || v == nil {
		if d.Status == string(model.TaskStatusSuccess) {
			d.ResultURL = ""
		}
		return
	}
	switch v.Status {
	case model.VideoDownloadStatusSuccess:
		d.Status = string(model.TaskStatusSuccess)
		d.ResultURL = BuildVideoPublicURL(task.TaskID) // 实时按当前 BaseURL 生成，改完立即生效
		d.Progress = "100%"
	case model.VideoDownloadStatusFailed:
		d.Status = string(model.TaskStatusFailure)
		d.ResultURL = ""
		reason := v.Error
		if reason == "" {
			reason = "video localize failed"
		}
		d.FailReason = reason
	default: // pending / downloading
		if d.Status == string(model.TaskStatusSuccess) {
			d.Status = string(model.TaskStatusInProgress)
			d.Progress = "downloading"
		}
		d.ResultURL = ""
	}
}

func videoLocalizeCleanup() {
	for {
		time.Sleep(1 * time.Hour)
		days := setting.VideoLocalizeRetainDaysInt()
		if days <= 0 {
			continue
		}
		cutoff := time.Now().AddDate(0, 0, -days)
		var list []*model.VideoDownload
		model.DB.Where("updated_at < ? AND status = ?", cutoff, model.VideoDownloadStatusSuccess).Find(&list)
		for _, v := range list {
			if v.LocalPath != "" {
				_ = os.Remove(v.LocalPath)
			}
			_ = model.DeleteVideoDownload(v.ID)
		}
	}
}
