/**
漏桶算法思路很简单，水（请求）先进入到漏桶里，漏桶以一定的速度出水，当水流入速度过大会直接溢出，可以看出漏桶算法能强行限制数据的传输速率
*/
package main

import (
	"fmt"
	"math"
	"sync"
	"time"
)

type LeakyBucket struct {
	rate        float64 //固定每秒出水速率
	capacity    float64 //桶的容量
	water       float64 //桶中当前水量
	lastLeakyMs int64   //桶上次漏水时间戳 ms

	lock sync.Mutex
}

func (lb *LeakyBucket) Create(rate float64, capacity float64) {
	lb.rate = rate
	lb.capacity = capacity
	lb.water = 0
	lb.lastLeakyMs = time.Now().UnixNano() / 1e6

	//(2)异步执行漏水的操作
	go func() {
		for {
			lb.lock.Lock()
			now := time.Now().UnixNano() / 1e6 //ms
			expand := float64(now - lb.lastLeakyMs) / 1000 *  lb.rate
			//(2.1)计算剩余水量,需要判断桶是否已经干了
			lb.water = lb.water - expand
			lb.water = math.Max(0, lb.water)
			lb.lastLeakyMs = now
			lb.lock.Unlock()
			//(2.2)sleep一段时间再执行漏水操作
			time.Sleep(time.Duration(1e3/rate) * time.Millisecond)
		}
	}()
}

func (lb *LeakyBucket) Allow() bool {
	lb.lock.Lock()
	defer lb.lock.Unlock()

	// 如果新加的水量之后桶中当前水量大于桶的容量表示水满执行溢出操作
	if lb.water+1 > lb.capacity {
		return false
	} else {
		lb.water++
		return true
	}
}

func main() {
	var leaky LeakyBucket
	leaky.Create(3, 3)

	wg := sync.WaitGroup{}

	for i := 1; i <= 40; i++ {
		wg.Add(1)

		go func(i int) {
			if leaky.Allow() {
				fmt.Println("Response req", i, time.Now())
			}
			wg.Done()
		}(i)
		time.Sleep(100 * time.Millisecond)
	}
	wg.Wait()
}
