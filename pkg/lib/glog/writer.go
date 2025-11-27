/**
 * @Author: dingQingHui
 * @Description:
 * @File: writer
 * @Version: 1.0.0
 * @Date: 2024/11/14 11:25
 */

package glog

import (
	"io"

	"gopkg.in/natefinch/lumberjack.v2"
)

func defaultWriter(filename string) io.Writer {
	var writer = &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    500,   // 最大M数，超过则切割
		MaxBackups: 100,   // 最大文件保留数，超过就删除最老的日志文件
		MaxAge:     30,    // 保存30天
		LocalTime:  true,  // 本地时间
		Compress:   false, // 是否压缩
	}
	return writer
}
