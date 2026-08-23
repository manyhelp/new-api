package ali

var ModelList = []string{
	"wan3.0-video",        // 万相3.0 All-in-One（文/首帧/首尾帧/参考生视频）
	"wan2.7-i2v",         // 万相2.7图生视频（新input.media协议）
	"wan2.7-t2v",         // 万相2.7文生视频
	"wan2.5-i2v-preview", // 万相2.5 preview（有声视频）推荐
	"wan2.2-i2v-flash",   // 万相2.2极速版（无声视频）
	"wan2.2-i2v-plus",    // 万相2.2专业版（无声视频）
	"wanx2.1-i2v-plus",   // 万相2.1专业版（无声视频）
	"wanx2.1-i2v-turbo",  // 万相2.1极速版（无声视频）
	// HappyHorse（阿里百炼 DashScope video-synthesis 异步协议，与万相同端点）。
	// 裸名按请求素材自动路由到 -t2v / -i2v / -r2v；带后缀名固定模式。
	"happyhorse-1.1",
	"happyhorse-1.1-t2v",
	"happyhorse-1.1-i2v",
	"happyhorse-1.1-r2v",
	"happyhorse-1.0",
	"happyhorse-1.0-t2v",
	"happyhorse-1.0-i2v",
	"happyhorse-1.0-r2v",
}

var ChannelName = "ali"
