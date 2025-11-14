/**
 * @Author: dingQingHui
 * @Description:
 * @File: api
 * @Version: 1.0.0
 * @Date: 2024/11/19 18:10
 */

package serializer

var (
	Json    = new(jsonCodec)
	MsgPack = new(msgPackCodec)
	PB      = new(pbCodec)
)
