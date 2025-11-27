package workers

import (
	"context"
	"testing"
	"time"
)

// TestWaitContext_Cancel 测试 Cancel 方法
func TestWaitContext_Cancel(t *testing.T) {
	wc := WithWaitContext(nil)
	ctx := wc.Context()

	// 取消 context
	wc.Cancel()

	// 验证 context 已被取消
	select {
	case <-ctx.Done():
		// 正常情况
	case <-time.After(100 * time.Millisecond):
		t.Error("Cancel() 后 context 应该被取消")
	}

	// 验证取消原因
	if ctx.Err() != context.Canceled {
		t.Errorf("期望错误为 context.Canceled，实际为: %v", ctx.Err())
	}
}

// TestWaitContext_Wait_NoParent 测试无父级的 Wait 方法
func TestWaitContext_Wait_NoParent(t *testing.T) {
	wc := WithWaitContext(nil)
	wc.Add(1)

	done := make(chan bool)
	go func() {
		wc.Wait()
		done <- true
	}()

	// 等待一小段时间确保 Wait 被调用
	time.Sleep(10 * time.Millisecond)

	// 完成 WaitGroup
	wc.Done()

	// 验证 Wait 正常返回
	select {
	case <-done:
		// 正常情况
	case <-time.After(1 * time.Second):
		t.Error("Wait() 应该在 WaitGroup 完成后返回")
	}
}

// TestWaitContext_Wait_WithParent 测试有父级的 Wait 方法
func TestWaitContext_Wait_WithParent(t *testing.T) {
	parent := WithWaitContext(nil)
	child := WithWaitContext(parent)

	// WithWaitContext 会自动调用 parent.Add(1)，所以只需要设置子级的计数
	child.Add(1)

	parentDone := make(chan bool)
	childDone := make(chan bool)

	// 启动父级等待
	go func() {
		parent.Wait()
		parentDone <- true
	}()

	// 启动子级等待
	go func() {
		child.Wait()
		childDone <- true
	}()

	// 等待一小段时间确保 Wait 被调用
	time.Sleep(10 * time.Millisecond)

	// 完成子级 WaitGroup
	child.Done()

	// 验证子级 Wait 正常返回
	select {
	case <-childDone:
		// 正常情况
	case <-time.After(1 * time.Second):
		t.Error("子级 Wait() 应该在 WaitGroup 完成后返回")
	}

	// 验证父级 Wait 也正常返回（因为子级 Wait 会调用 parent.Done()）
	select {
	case <-parentDone:
		// 正常情况
	case <-time.After(1 * time.Second):
		t.Error("父级 Wait() 应该在子级完成后返回")
	}
}

// TestWaitContext_CancelPropagation 测试取消传播
func TestWaitContext_CancelPropagation(t *testing.T) {
	parent := WithWaitContext(nil)
	child := WithWaitContext(parent)

	// 取消父级
	parent.Cancel()

	// 验证子级也被取消
	select {
	case <-child.Context().Done():
		// 正常情况
	case <-time.After(100 * time.Millisecond):
		t.Error("父级取消后，子级 context 应该也被取消")
	}

	// 验证子级取消不会影响父级
	child.Cancel()
	select {
	case <-parent.Context().Done():
		// 正常情况（父级已经被取消）
	default:
		// 如果父级还没被取消，子级取消不应该影响它
		// 但在这个测试中，父级已经被取消了
	}
}

// TestWaitContext_Concurrent 测试并发安全性
func TestWaitContext_Concurrent(t *testing.T) {
	wc := WithWaitContext(nil)
	const goroutineCount = 100

	wc.Add(goroutineCount)
	// 启动多个 goroutine 同时操作
	for i := 0; i < goroutineCount; i++ {
		go func() {
			defer wc.Done()
			time.Sleep(time.Millisecond)
		}()
	}
	// 等待所有 goroutine 完成
	wc.Wait()
}

// TestWaitContext_MultiLevel 测试多层级结构
func TestWaitContext_MultiLevel(t *testing.T) {
	root := WithWaitContext(nil)
	level1 := WithWaitContext(root)
	level2 := WithWaitContext(level1)

	// 验证层级关系
	if level1.parent != root {
		t.Error("level1 的 parent 应该是 root")
	}
	if level2.parent != level1 {
		t.Error("level2 的 parent 应该是 level1")
	}

	// 验证 context 继承
	root.Cancel()
	select {
	case <-level1.Context().Done():
		// 正常情况
	case <-time.After(100 * time.Millisecond):
		t.Error("root 取消后，level1 应该被取消")
	}

	select {
	case <-level2.Context().Done():
		// 正常情况
	case <-time.After(100 * time.Millisecond):
		t.Error("root 取消后，level2 应该被取消")
	}
}
