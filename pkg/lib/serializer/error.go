/**
 * @Author: dingQingHui
 * @Description:
 * @File: error
 * @Version: 1.0.0
 * @Date: 2024/11/19 18:15
 */

package serializer

import "errors"

var (
	ErrMsgPackPack   = errors.New("msgpack打包错误")
	ErrMsgPackUnPack = errors.New("msgpack解析错误")
	ErrPBPack        = errors.New("pb打包错误")
	ErrPBUnPack      = errors.New("pb解析错误")
	ErrNotPBMsg      = errors.New("不是pb消息")
	ErrJsonPack      = errors.New("json打包错误")
	ErrJsonUnPack    = errors.New("json解析错误")
)
