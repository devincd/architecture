/**
限流方式-计数器
在一段时间间隔内，对请求进行计数，与阈值进行判断是否需要限流，一旦到了时间临界点，将计数器清零

planB: 当请求频率太快时,后续的请求等待之前的请求完成后才进行
 */
package main

import (
	"time"
	"sync"
	"fmt"
)

type LimitRate struct {
	rate int  			//计数周期内最多允许的请求数 (可以等于)
	begin time.Time		//计数开始时间
	cycle time.Duration	//计数周期
	count int 			//计数周期内累计收到的请求数
	lock sync.Mutex
}

func (limit *LimitRate) Create(r int, cycle time.Duration) {
	limit.rate = r
	limit.begin = time.Now()
	limit.cycle = cycle
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

	//(2)计数的个数大于或等于计数周期内最多允许的请求数时,需要判断是否到达了计数周期
	if limit.count >= limit.rate {
		//循环等待请求,避免将请求直接丢弃
		for {
			now := time.Now()
			//(2.1)当前时间距离上次重置时的时间间隔大于或等于计数周期时需要重置计数器
			if now.Sub(limit.begin) >= limit.cycle {
				limit.Reset()
				//(2.2)重置后需要将对该次请求计数
				limit.count++
				return true
			}
		}
	} else {
		limit.count++
		return true
	}
}

func main() {
	var limitEg LimitRate
	limitEg.Create(2, time.Second)	// 1s内只允许2个请求通过
	wg := sync.WaitGroup{}
	for i:=0; i<10; i++ {
		wg.Add(1)

		//fmt.Println("Create req", i, time.Now())
		go func() {
			if limitEg.Allow() {
				fmt.Println("Respon req", i, time.Now())
			}
			wg.Done()
		}()
		time.Sleep(200 * time.Millisecond)
	}
	wg.Wait()
}


