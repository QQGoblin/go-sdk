/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package leaderelection

import (
	"bytes"
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rl "k8s.io/client-go/tools/leaderelection/resourcelock"
)

/*
  LeaderElectorWithConditions 重写了原生的 LeaderElector 对象的 tryAcquireOrRenew 接口。
  用户可以在初始化时，传入一个 PreConditions 函数。该函数用户可以自定义健康检查规则，当健康检查失败时将停止 renew 租约。
  LeaderElectorWithConditions 修改了原有 LeaderElector 的以下函数：
  - tryAcquireOrRenew
  - renew
  - acquire
  - 移除了同步接口 Run，添加异步接口 Startup 和 Stop
*/

const (
	DEFAULT_JOINING_ELECTOR_AGAIN = 30 * time.Second
)

type PreConditions func(context.Context) error

type LeaderElectorWithConditions struct {
	le      *LeaderElector
	preHook PreConditions
	id      string
	mutex   sync.Mutex
	cancel  context.CancelFunc
	active  bool
}

func NewLeaderElectorWithConditions(id string, lec LeaderElectionConfig, preCondHook PreConditions) (*LeaderElectorWithConditions, error) {

	le, err := NewLeaderElector(lec)
	if err != nil {
		return nil, err
	}

	if preCondHook == nil {
		return nil, fmt.Errorf("PreConditions callback must not be nil")
	}

	return &LeaderElectorWithConditions{
		le:      le,
		preHook: preCondHook,
		id:      id,
		cancel:  nil,
		active:  false,
	}, nil
}

func (lewc *LeaderElectorWithConditions) GetLeader() string {
	return lewc.le.GetLeader()
}

func (lewc *LeaderElectorWithConditions) IsLeader() bool {
	return lewc.le.IsLeader()
}

func (lewc *LeaderElectorWithConditions) Check(maxTolerableExpiredLease time.Duration) error {
	return lewc.le.Check(maxTolerableExpiredLease)
}

func (lewc *LeaderElectorWithConditions) tryAcquireOrRenew(ctx context.Context) bool {

	// tryAcquireOrRenew 是获取 leader 的核心函数，负责更新 lease 对象状态
	// 创建 LeaderElectionRecord
	now := metav1.Now()
	leaderElectionRecord := rl.LeaderElectionRecord{
		HolderIdentity:       lewc.le.config.Lock.Identity(),
		LeaseDurationSeconds: int(lewc.le.config.LeaseDuration / time.Second),
		RenewTime:            now,
		AcquireTime:          now,
	}

	// 1. obtain or create the ElectionRecord
	oldLeaderElectionRecord, oldLeaderElectionRawRecord, err := lewc.le.config.Lock.Get(ctx)
	if err != nil {
		// 获取对象失败，返回
		if !errors.IsNotFound(err) {
			klog.Errorf("error retrieving resource lock %v: %v", lewc.le.config.Lock.Describe(), err)
			return false
		}

		// TODO: 添加前置检查
		if err := lewc.preHook(ctx); err != nil {
			klog.Errorf("Failed to preHook action: %+v", err)
			return false
		}

		// 对象不存在创建
		if err = lewc.le.config.Lock.Create(ctx, leaderElectionRecord); err != nil {
			klog.Errorf("error initially creating leader election record: %v", err)
			return false
		}
		// 更新内存中的 ElectionRecord 信息，
		// 创建了 LeaderElectionRecord，当前作为 Leader
		lewc.le.observedRecord = leaderElectionRecord
		lewc.le.observedTime = lewc.le.clock.Now()
		return true
	}

	// 2. Record obtained, check the Identity & Time
	if !bytes.Equal(lewc.le.observedRawRecord, oldLeaderElectionRawRecord) {
		lewc.le.observedRecord = *oldLeaderElectionRecord
		lewc.le.observedRawRecord = oldLeaderElectionRawRecord
		lewc.le.observedTime = lewc.le.clock.Now()
	}

	// Leader 不是自己，并且当前租约还未超期
	if len(oldLeaderElectionRecord.HolderIdentity) > 0 &&
		lewc.le.observedTime.Add(lewc.le.config.LeaseDuration).After(now.Time) &&
		!lewc.IsLeader() {
		klog.V(4).Infof("lock is held by %v and has not yet expired", oldLeaderElectionRecord.HolderIdentity)
		return false
	}

	// 3. We're going to try to update. The leaderElectionRecord is set to it's default
	// here. Let's correct it before updating.
	if lewc.IsLeader() {
		// 租约超期，但是 Leader 还是自己
		leaderElectionRecord.AcquireTime = oldLeaderElectionRecord.AcquireTime
		leaderElectionRecord.LeaderTransitions = oldLeaderElectionRecord.LeaderTransitions
	} else {
		// Leader 不是自己，并且租约超期
		leaderElectionRecord.LeaderTransitions = oldLeaderElectionRecord.LeaderTransitions + 1
	}

	// update the lock itself
	if err = lewc.le.config.Lock.Update(ctx, leaderElectionRecord); err != nil {
		klog.Errorf("Failed to update lock: %v", err)
		return false
	}

	// TODO: 添加前置检查
	if err := lewc.preHook(ctx); err != nil {
		klog.Errorf("Failed to preHook action: %+v", err)
		return false
	}

	lewc.le.observedRecord = leaderElectionRecord
	lewc.le.observedTime = lewc.le.clock.Now()
	return true
}

func (lewc *LeaderElectorWithConditions) acquire(ctx context.Context) bool {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	succeeded := false
	desc := lewc.le.config.Lock.Describe()
	klog.Infof("attempting to acquire leader lease %v...", desc)
	wait.JitterUntil(func() {
		succeeded = lewc.tryAcquireOrRenew(ctx)
		lewc.le.maybeReportTransition()
		if !succeeded {
			klog.V(4).Infof("failed to acquire lease %v", desc)
			return
		}
		lewc.le.config.Lock.RecordEvent("became leader")
		lewc.le.metrics.leaderOn(lewc.le.config.Name)
		klog.Infof("successfully acquired lease %v", desc)
		cancel()
	}, lewc.le.config.RetryPeriod, JitterFactor, true, ctx.Done())
	return succeeded
}

func (lewc *LeaderElectorWithConditions) renew(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	wait.Until(func() {
		timeoutCtx, timeoutCancel := context.WithTimeout(ctx, lewc.le.config.RenewDeadline)
		defer timeoutCancel()
		err := wait.PollImmediateUntil(lewc.le.config.RetryPeriod, func() (bool, error) {
			return lewc.tryAcquireOrRenew(timeoutCtx), nil
		}, timeoutCtx.Done())

		lewc.le.maybeReportTransition()
		desc := lewc.le.config.Lock.Describe()
		if err == nil {
			klog.V(5).Infof("successfully renewed lease %v", desc)
			return
		}
		lewc.le.config.Lock.RecordEvent("stopped leading")
		lewc.le.metrics.leaderOff(lewc.le.config.Name)
		klog.Infof("failed to renew lease %v: %v", desc, err)
		cancel()
	}, lewc.le.config.RetryPeriod, ctx.Done())

	// if we hold the lease, give it up
	if lewc.le.config.ReleaseOnCancel {
		lewc.le.release()
	}
}

func (lewc *LeaderElectorWithConditions) Startup() {

	klog.Info("start the election function")

	if lewc.active {
		return
	}

	// 创建 context
	lewc.mutex.Lock()
	var ctx context.Context
	ctx, lewc.cancel = context.WithCancel(context.Background())
	lewc.active = true
	lewc.mutex.Unlock()

	// 支持在不影响主线程的情况下，启动或者停止选举任务
	go wait.UntilWithContext(ctx,
		func(ctx context.Context) {
			defer runtime.HandleCrash()
			defer func() {
				lewc.le.config.Callbacks.OnStoppedLeading()
			}()

			if !lewc.acquire(ctx) {
				return // ctx signalled done
			}
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			go lewc.le.config.Callbacks.OnStartedLeading(ctx)
			lewc.renew(ctx)
			klog.Warning("the election exit unexpectedly and was ready to restart")
		}, DEFAULT_JOINING_ELECTOR_AGAIN)
}

func (lewc *LeaderElectorWithConditions) Stop() {
	klog.Info("stop the election function")
	// 加锁
	lewc.mutex.Lock()
	defer lewc.mutex.Unlock()
	if lewc.cancel != nil {
		lewc.active = false // 关闭 election 携程
		lewc.cancel()
		lewc.cancel = nil
	}
}
