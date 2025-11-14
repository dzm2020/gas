/**
 * @Author: dingQingHui
 * @Description:
 * @File: system
 * @Version: 1.0.0
 * @Date: 2023/12/7 14:54
 */

package actor

import (
	"gas/pkg/utils/reflectx"
	"sync/atomic"

	"github.com/duke-git/lancet/v2/maputil"
)

var (
	uniqId      atomic.Uint64
	processDict = maputil.NewConcurrentMap[uint64, IProcess](10)

	routerDict = maputil.NewConcurrentMap[string, IRouter](10)
)

func Spawn(producer Producer, params ...interface{}) IProcess {
	actorId := uniqId.Add(1)
	actor := producer()
	router := getRouter(actor)
	context := newBaseActorContext(actorId, actor, router)
	mailBox := NewMailbox()
	mailBox.RegisterHandlers(context, NewDefaultDispatcher(50))
	process := NewProcess(context, mailBox)
	_ = process.Post("OnInit", params)
	return process
}

func getRouter(actor IActor) IRouter {
	name := reflectx.TypeFullName(actor)
	v, ok := routerDict.Get(name)
	if !ok {
		v, ok = routerDict.GetOrSet(name, NewRouter(actor))
	}
	return v
}
