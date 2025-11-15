/**
 * @Author: dingQingHui
 * @Description:
 * @File: system
 * @Version: 1.0.0
 * @Date: 2023/12/7 14:54
 */

package actor

import (
	"sync/atomic"

	"github.com/duke-git/lancet/v2/maputil"
)

var (
	uniqId      atomic.Uint64
	processDict = maputil.NewConcurrentMap[uint64, IProcess](10)
)

func Spawn(producer Producer, options ...Option) IProcess {
	opts := loadOptions(options...)
	actorId := uniqId.Add(1)
	actor := producer()
	context := newBaseActorContext(actorId, actor, opts.Middlewares)
	mailBox := NewMailbox()
	mailBox.RegisterHandlers(context, NewDefaultDispatcher(50))
	process := NewProcess(context, mailBox)
	_ = process.PushTask(func(ctx IContext) error {
		return ctx.Actor().OnInit(ctx, opts.Params)
	})
	return process
}
