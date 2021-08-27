/**
限流方式-计数器
在一段时间间隔内，对请求进行计数，与阈值进行判断是否需要限流，一旦到了时间临界点，将计数器清零。

planA: 当请求频率过快时,直接抛弃后续请求
*/
package main

import (
	"fmt"
	"sync"
	"time"
)

type LimitRate struct {
	rate  int           //计数周期内最多允许的请求数 (可以等于)
	begin time.Time     //计数开始时间
	cycle time.Duration //计数周期
	count int           //计数周期内累计收到的请求数
	lock  sync.Mutex
}

func (limit *LimitRate) Create(r int, cycle time.Duration) {
	limit.rate = r
	limit.begin = time.Now()
	limit.cycle = cycle //初始化计数周期,也就是在该时间段内最多允许rate中请求透过
	limit.count = 0
}

func (limit *LimitRate) Reset() {
	limit.begin = time.Now()
	limit.count = 0
}

func (limit *LimitRate) Allow() bool {
	//(1)并发下需要加锁,保证协程安全操作
	limit.lock.Lock()
	defer limit.lock.Unlock()

	//(2)计数的个数(计数提前已经加过1)大于计数周期内最多允许的请求数时,需要判断是否到达了计数周期
	if limit.count+1 > limit.rate {
		now := time.Now()
		//(2.1)当前时间距离上次重置时的时间间隔大于或等于计数周期时需要重置计数器
		if now.Sub(limit.begin) >= limit.cycle {
			limit.Reset()
			return false
			//(2.2)小于计数周期的话,直接过滤掉该请求
		} else {
			return false
		}
	} else {
		limit.count++
		return true
	}
}

func main() {
	var limiter LimitRate
	limiter.Create(2, time.Second) // 1s内只允许两个请求通过
	wg := sync.WaitGroup{}
	// 模拟在 2s 内发送20个请求，正常情况下有4个请求通过
	for i := 1; i <= 20; i++ {
		wg.Add(1)
		go func(i int) {
			if limiter.Allow() {
				fmt.Println("Response req", i, time.Now())
			}
			wg.Done()
		}(i)
		time.Sleep(100 * time.Millisecond)
	}
	wg.Wait()
}
